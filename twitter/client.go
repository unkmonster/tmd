package twitter

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/unkmonster/tmd/internal/utils"
)

const bearer = "AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA"

var clientScreenNames sync.Map
var clientErrors sync.Map
var clientRateLimiters sync.Map
var apiCounts sync.Map

func SetClientAuth(client *resty.Client, authToken string, ct0 string) {
	client.SetAuthToken(bearer)
	client.SetCookie(&http.Cookie{
		Name:  "auth_token",
		Value: authToken,
	})
	client.SetCookie(&http.Cookie{
		Name:  "ct0",
		Value: ct0,
	})
	client.SetHeader("X-Csrf-Token", ct0)
}

func Login(ctx context.Context, authToken string, ct0 string) (*resty.Client, string, error) {
	client := resty.New()

	// 鉴权
	SetClientAuth(client, authToken, ct0)

	// 错误检查
	client.OnAfterResponse(func(c *resty.Client, r *resty.Response) error {
		if err := CheckApiResp(r.Body()); err != nil {
			return err
		}
		if err := utils.CheckRespStatus(r); err != nil {
			return err
		}
		return nil
	})

	// 重试
	client.SetRetryCount(5)
	client.AddRetryCondition(func(r *resty.Response, err error) bool {
		if err == ErrWouldBlock {
			return false
		}
		// For TCP Error
		_, ok := err.(*TwitterApiError)
		_, ok2 := err.(*utils.HttpStatusError)
		return !ok && !ok2 && err != nil
	})
	client.AddRetryCondition(func(r *resty.Response, err error) bool {
		// For Twitter API Error
		v, ok := err.(*TwitterApiError)
		return ok && r.Request.RawRequest.Host == "x.com" && (v.Code == ErrTimeout || v.Code == ErrOverCapacity || v.Code == ErrDependency)
	})
	client.AddRetryCondition(func(r *resty.Response, err error) bool {
		// For Http 429
		v, ok := err.(*utils.HttpStatusError)
		return ok && r.Request.RawRequest.Host == "x.com" && v.Code == 429
	})

	client.SetTransport(&http.Transport{
		MaxIdleConns:          0,
		MaxIdleConnsPerHost:   1000,            // 每个主机最大并发连接数
		IdleConnTimeout:       5 * time.Second, // 连接空闲 n 秒后断开它
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		Proxy:                 http.ProxyFromEnvironment,
	})

	screenName, err := GetSelfScreenName(ctx, client)
	if err != nil {
		return nil, "", err
	}

	clientScreenNames.Store(client, screenName)
	return client, screenName, nil
}

func GetClientScreenName(client *resty.Client) string {
	if v, ok := clientScreenNames.Load(client); ok {
		return v.(string)
	}
	return ""
}

var ErrWouldBlock = fmt.Errorf("EWOULDBLOCK")

type xRateLimit struct {
	ResetTime time.Time
	Remaining int
	Limit     int
	Ready     bool
	Url       string
	Mtx       sync.Mutex
}

func (rl *xRateLimit) _wouldBlock() bool {
	threshold := max(2*rl.Limit/100, 1)
	return rl.Remaining <= threshold && time.Now().Before(rl.ResetTime)
}

func (rl *xRateLimit) wouldBlock() bool {
	rl.Mtx.Lock()
	defer rl.Mtx.Unlock()
	return rl._wouldBlock()
}

func (rl *xRateLimit) preRequest(ctx context.Context, nonBlocking bool) error {
	rl.Mtx.Lock()
	defer rl.Mtx.Unlock()

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if time.Now().After(rl.ResetTime) {
		log.WithFields(log.Fields{
			"path": rl.Url,
		}).Debugf("[RateLimiter] rate limit is expired")
		rl.Ready = false // 后续的请求等待本次请求完成更新速率限制
		return nil
	}

	if !rl._wouldBlock() {
		rl.Remaining--
		return nil
	} else {
		if nonBlocking {
			return ErrWouldBlock
		}

		insurance := 5 * time.Second
		log.WithFields(log.Fields{
			"path":  rl.Url,
			"until": rl.ResetTime.Add(insurance),
		}).Warnln("[RateLimiter] start sleeping")

		origin, err := utils.GetConsoleTitle()
		if err == nil {
			utils.SetConsoleTitle(fmt.Sprintf("idle - sleeping until %v", rl.ResetTime.Add(insurance).Format(time.TimeOnly)))
			defer utils.SetConsoleTitle(origin)
		} else {
			log.Warnln("failed to set console title:", err)
		}

		select {
		case <-time.After(time.Until(rl.ResetTime) + insurance):
			rl.Ready = false
		case <-ctx.Done():
		}
		return nil
	}
}

// 必须返回 nil 或就绪的 rateLimit，否则死锁
func makeRateLimit(resp *resty.Response) *xRateLimit {
	header := resp.Header()
	limit := header.Get("X-Rate-Limit-Limit")
	if limit == "" {
		return nil // 没有速率限制信息
	}
	remaining := header.Get("X-Rate-Limit-Remaining")
	if remaining == "" {
		return nil // 没有速率限制信息
	}
	resetTime := header.Get("X-Rate-Limit-Reset")
	if resetTime == "" {
		return nil // 没有速率限制信息
	}

	resetTimeNum, err := strconv.ParseInt(resetTime, 10, 64)
	if err != nil {
		return nil
	}
	remainingNum, err := strconv.Atoi(remaining)
	if err != nil {
		return nil
	}
	limitNum, err := strconv.Atoi(limit)
	if err != nil {
		return nil
	}

	u, _ := url.Parse(resp.Request.URL)
	url := filepath.Join(u.Host, u.Path)

	resetTimeTime := time.Unix(resetTimeNum, 0)
	return &xRateLimit{
		ResetTime: resetTimeTime,
		Remaining: remainingNum,
		Limit:     limitNum,
		Ready:     true,
		Url:       url,
	}
}

type rateLimiter struct {
	limits      sync.Map
	conds       sync.Map
	nonBlocking bool
}

func newRateLimiter(nonBlocking bool) rateLimiter {
	return rateLimiter{nonBlocking: nonBlocking}
}

func (rateLimiter *rateLimiter) check(ctx context.Context, url *url.URL) error {
	if !rateLimiter.shouldWork(url) {
		return nil
	}

	path := url.Path
	cod, _ := rateLimiter.conds.LoadOrStore(path, sync.NewCond(&sync.Mutex{}))
	cond := cod.(*sync.Cond)
	cond.L.Lock()
	defer cond.L.Unlock()

	lim, loaded := rateLimiter.limits.LoadOrStore(path, &xRateLimit{})
	limit := lim.(*xRateLimit)
	if !loaded {
		// 首次遇见某路径时直接请求初始化它，后续请求等待这次请求使 limit 就绪
		// 响应中没有速率限制信息：此键赋空，意味不进行速率限制
		return nil
	}

	/*
		同一时刻仅允许一个未就绪的请求通过检查，其余在这里阻塞，等待前者将速率限制就绪
		未就绪的情况：
		1. 首次请求
		2. 休眠后，速率限制过期

		响应钩子中必须使此键就绪/赋空/删除键并唤醒一个新请求，否则会死锁
	*/
	for limit != nil && !limit.Ready {
		cond.Wait()
		lim, loaded := rateLimiter.limits.LoadOrStore(path, &xRateLimit{})
		if !loaded {
			// 上个请求失败了，从它身上继承初始化速率限制的重任
			return nil
		}
		limit = lim.(*xRateLimit)
	}

	// limiter 为 nil 意味着不对此路径做速率限制
	if limit != nil {
		return limit.preRequest(ctx, rateLimiter.nonBlocking)
	}
	return nil
}

// 重置非就绪的速率限制，使其可检查，否则死锁
func (rateLimiter *rateLimiter) reset(url *url.URL, resp *resty.Response) {
	if !rateLimiter.shouldWork(url) {
		return
	}

	path := url.Path
	co, ok := rateLimiter.conds.Load(path)
	if !ok {
		return // BeforeRequest 从未调用的情况下调用了 OnError/OnRetry
	}
	cond := co.(*sync.Cond)
	cond.L.Lock()
	defer cond.L.Unlock()

	lim, ok := rateLimiter.limits.Load(path)
	if !ok {
		return
	}
	limit := lim.(*xRateLimit)
	if limit == nil || limit.Ready {
		return
	}

	if resp != nil && resp.RawResponse != nil {
		// 请求成功，或发生了错误/触发了重试条件但有能力更新速率限制
		rateLimit := makeRateLimit(resp)
		rateLimiter.limits.Store(path, rateLimit)
		cond.Broadcast()
	} else {
		// 将此路径设为首次请求前的状态
		rateLimiter.limits.Delete(path)
		cond.Signal()
	}
}

func (*rateLimiter) shouldWork(url *url.URL) bool {
	return !strings.HasSuffix(url.Host, "twimg.com")
}

func (rl *rateLimiter) wouldBlock(path string) bool {
	if v, ok := rl.limits.Load(path); ok {
		return v.(*xRateLimit) != nil && v.(*xRateLimit).wouldBlock()
	}
	return false
}

func EnableRateLimit(client *resty.Client) {
	rateLimiter := newRateLimiter(true)
	clientRateLimiters.Store(client, &rateLimiter)

	client.OnBeforeRequest(func(c *resty.Client, req *resty.Request) error {
		u, err := url.Parse(req.URL)
		if err != nil {
			return err
		}
		return rateLimiter.check(req.Context(), u)
	})

	client.OnSuccess(func(c *resty.Client, resp *resty.Response) {
		rateLimiter.reset(resp.Request.RawRequest.URL, resp)
	})

	client.OnError(func(req *resty.Request, err error) {
		// onbeforerequest 返回假会导致 rawRequest 为空
		if req == nil || req.RawRequest == nil {
			return
		}

		var resp *resty.Response
		if v, ok := err.(*resty.ResponseError); ok {
			// Do something with v.Response
			resp = v.Response
		}
		// Log the error, increment a metric, etc...
		rateLimiter.reset(req.RawRequest.URL, resp)
	})

	client.AddRetryHook(func(resp *resty.Response, err error) {
		// 请求发起前的错误
		if resp == nil || resp.Request == nil || resp.Request.RawRequest == nil {
			return
		}
		rateLimiter.reset(resp.Request.RawRequest.URL, resp)
	})
}

func EnableRequestCounting(client *resty.Client) {
	client.OnBeforeRequest(func(c *resty.Client, req *resty.Request) error {
		url, err := url.Parse(req.URL)
		if err != nil {
			return err
		}

		if strings.HasSuffix(url.Host, "twimg.com") {
			return nil
		}

		v, _ := apiCounts.LoadOrStore(url.Path, &atomic.Int32{})
		v.(*atomic.Int32).Add(1)
		return nil
	})
}

func ReportRequestCount() {
	apiCounts.Range(func(key, value any) bool {
		log.Debugf("* %s request count: %d", key, value.(*atomic.Int32).Load())
		return true
	})
}

func GetSelfScreenName(ctx context.Context, client *resty.Client) (string, error) {
	resp, err := client.R().SetContext(ctx).Get("https://api.x.com/1.1/account/settings.json")
	if err != nil {
		return "", err
	}
	if err := utils.CheckRespStatus(resp); err != nil {
		return "", err
	}
	return gjson.GetBytes(resp.Body(), "screen_name").String(), nil
}

func GetClientError(cli *resty.Client) error {
	if v, ok := clientErrors.Load(cli); ok {
		return v.(error)
	}
	return nil
}

func SetClientError(cli *resty.Client, err error) {
	clientErrors.Store(cli, err)
	if err != nil {
		log.WithField("client", GetClientScreenName(cli)).Debugln("client is no longer available:", err)
	}
}

func GetClientRateLimiter(cli *resty.Client) *rateLimiter {
	if v, ok := clientRateLimiters.Load(cli); ok {
		return v.(*rateLimiter)
	}
	return nil
}

var showStateToken = make(chan struct{}, 1)

// 选择一个请求指定端点不会阻塞的客户端
func SelectClient(ctx context.Context, clients []*resty.Client, path string) *resty.Client {
	for ctx.Err() == nil {
		errs := 0
		for _, client := range clients {
			if GetClientError(client) != nil {
				errs++
				continue
			}

			rl := GetClientRateLimiter(client)
			if rl == nil || !rl.wouldBlock(path) {
				return client
			}
		}

		if errs == len(clients) {
			return nil // no client available
		}

		select {
		default:
		case showStateToken <- struct{}{}:
			defer func() { <-showStateToken }()
			log.Warnln("waiting for any client to wake up")
			origin, err := utils.GetConsoleTitle()
			if err == nil {
				defer utils.SetConsoleTitle(origin)
				utils.SetConsoleTitle("waiting for any client to wake up")
			} else {
				log.Debugln("failed to get console title:", err)
			}
		}

		select {
		case <-ctx.Done():
		case <-time.After(3 * time.Second):
		}
	}
	return nil
}

func SelectUserMediaClient(ctx context.Context, clients []*resty.Client) *resty.Client {
	return SelectClient(ctx, clients, (&userMedia{}).Path())
}
