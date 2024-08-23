package twitter

import (
	"context"
	"io"
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
	"github.com/unkmonster/tmd2/internal/utils"
)

const bearer = "AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA"

var clientScreenNames map[*resty.Client]string = make(map[*resty.Client]string)
var clientBlockStates map[*resty.Client]*atomic.Bool = make(map[*resty.Client]*atomic.Bool)

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

	// 禁用 logger
	nullLogger := log.New()
	nullLogger.SetOutput(io.Discard)
	client.SetLogger(nullLogger)

	// 鉴权
	SetClientAuth(client, authToken, ct0)

	// 重试
	client.SetRetryCount(5)
	client.AddRetryAfterErrorCondition()
	client.AddRetryCondition(func(r *resty.Response, err error) bool {
		return !strings.HasSuffix(r.Request.RawRequest.Host, "twimg.com") && err != nil
	})
	client.AddRetryCondition(func(r *resty.Response, err error) bool {
		// For OverCapacity
		return r.Request.RawRequest.Host == "x.com" && r.StatusCode() == 400
	})
	client.AddRetryCondition(func(r *resty.Response, err error) bool {
		// 仅重试 429 Rate Limit Exceed
		return r.Request.RawRequest.Host == "x.com" && r.StatusCode() == 429 && CheckApiResp(r) == nil
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

	clientBlockStates[client] = &atomic.Bool{}
	clientScreenNames[client] = screenName
	return client, screenName, nil
}

func GetClientScreenName(client *resty.Client) string {
	return clientScreenNames[client]
}

func GetClientBlockState(client *resty.Client) bool {
	return clientBlockStates[client].Load()
}

type xRateLimit struct {
	ResetTime time.Time
	Remaining int
	Limit     int
	Ready     bool
	Url       string
}

func (rl *xRateLimit) preRequest(ctx context.Context) error {
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

	threshold := max(2*rl.Limit/100, 1)

	if rl.Remaining > threshold {
		rl.Remaining--
		return nil
	} else {
		insurance := 5 * time.Second
		log.WithFields(log.Fields{
			"path":  rl.Url,
			"until": rl.ResetTime.Add(insurance),
		}).Warnln("[RateLimiter] start sleeping")

		select {
		case <-time.After(time.Until(rl.ResetTime) + insurance):
			rl.Ready = false
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
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
	limits sync.Map
	conds  sync.Map
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
		return limit.preRequest(ctx)
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

func EnableRateLimit(client *resty.Client) {
	rateLimiter := rateLimiter{}

	client.OnBeforeRequest(func(c *resty.Client, req *resty.Request) error {
		u, err := url.Parse(req.URL)
		if err != nil {
			return err
		}
		// temp

		clientBlockStates[c].Store(true)
		defer clientBlockStates[c].Store(false)
		return rateLimiter.check(req.Context(), u)
	})

	client.OnSuccess(func(c *resty.Client, resp *resty.Response) {
		rateLimiter.reset(resp.Request.RawRequest.URL, resp)
	})

	client.OnError(func(req *resty.Request, err error) {
		var resp *resty.Response
		if v, ok := err.(*resty.ResponseError); ok {
			// Do something with v.Response
			resp = v.Response
		}
		// Log the error, increment a metric, etc...
		rateLimiter.reset(req.RawRequest.URL, resp)
	})

	client.AddRetryHook(func(resp *resty.Response, err error) {
		rateLimiter.reset(resp.Request.RawRequest.URL, resp)
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
