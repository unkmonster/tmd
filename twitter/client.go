package twitter

import (
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
	"github.com/unkmonster/tmd2/internal/utils"
)

func Login(cookie_str string, authToken string) (*resty.Client, string, error) {
	cookie, err := utils.ParseCookie(cookie_str)
	if err != nil {
		return nil, "", err
	}

	client := resty.New()
	// authorization
	client.SetHeader("cookie", cookie_str)
	client.SetHeader("X-Csrf-Token", cookie["ct0"])
	client.SetAuthToken(authToken)
	// retry
	client.SetRetryCount(5)
	//transport
	client.SetTransport(&http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     5 * time.Second,
	})
	// timeout
	client.SetTimeout(5 * time.Second)

	//
	resp, err := client.R().Get("https://api.x.com/1.1/account/settings.json")
	if err != nil {
		return nil, "", err
	}
	if err = utils.CheckRespStatus(resp); err != nil {
		return nil, "", err
	}

	return client, gjson.Get(resp.String(), "screen_name").String(), nil
}

type xRateLimit struct {
	ResetTime time.Time
	Remaining int
	Limit     int
	Ready     bool
	Url       string
}

func (rl *xRateLimit) Req() bool {
	if !rl.Ready {
		log.Printf("not ready %s\n", rl.Url)
		return false
	}

	if time.Now().After(rl.ResetTime) {
		log.Printf("expired %s\n", rl.Url)
		rl.Ready = false
		return false
	}

	if rl.Remaining > rl.Limit/100 {
		rl.Remaining--
		log.Printf("requested %s: remaining  %d\n", rl.Url, rl.Remaining)
		return true
	} else {
		log.Printf("[RateLimit] %s Sleep until %s\n", rl.Url, rl.ResetTime)
		time.Sleep(time.Until(rl.ResetTime))
		rl.Ready = false
		return false
	}
}

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

	resetTimeNum, err := strconv.ParseUint(resetTime, 10, 64)
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

	return &xRateLimit{
		ResetTime: time.Unix(int64(resetTimeNum), 0),
		Remaining: remainingNum,
		Limit:     limitNum,
		Ready:     true,
		Url:       url,
	}
}

func EnableRateLimit(client *resty.Client) {
	limiters := make(map[string]*xRateLimit)
	mutexs := sync.Map{}
	conds := make(map[string]*sync.Cond)

	client.OnBeforeRequest(func(c *resty.Client, req *resty.Request) error {
		u, err := url.Parse(req.URL)
		if err != nil {
			return err
		}
		mut, _ := mutexs.LoadOrStore(u.Path, &sync.Mutex{})
		mutex := mut.(*sync.Mutex)

		mutex.Lock()
		defer mutex.Unlock()

		// 首次请求这个路径，给此路径赋一个初始值后直接开始请求
		_, exist := limiters[u.Path]
		if !exist {
			limiters[u.Path] = &xRateLimit{}
			conds[u.Path] = sync.NewCond(mutex)
			// limiter = limiters[u.Path]
			// cond = conds[u.Path]
			return nil
		}

		for limiters[u.Path] != nil && !limiters[u.Path].Ready {
			conds[u.Path].Wait()
		}

		if limiters[u.Path] != nil {
			limiters[u.Path].Req()
		}
		return nil
	})

	client.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error {
		u, err := url.Parse(resp.Request.URL)
		if err != nil {
			return err
		}

		// 不获取互斥量，仅在速率限制未就绪的情况下使其就绪并解锁
		mut, _ := mutexs.Load(u.Path)
		mutex := mut.(*sync.Mutex)

		mutex.Lock()
		defer mutex.Unlock()
		limiter := limiters[u.Path] // 绝对存在
		if limiter == nil || limiter.Ready {
			return nil
		}

		// 重置速率限制
		newLimiter := makeRateLimit(resp)
		limiters[u.Path] = newLimiter
		conds[u.Path].Broadcast()
		return nil
	})
}
