package twitter

import (
	"log"
	"net/http"
	"net/url"
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
}

func (rl *xRateLimit) Req() bool {
	if !rl.Ready {
		log.Printf("not ready\n")
		return false
	}

	if time.Now().After(rl.ResetTime) {
		log.Printf("expired\n")
		rl.Ready = false
		return false
	}

	if rl.Remaining > 0 {
		rl.Remaining--
		log.Printf("requested: remaining %d\n", rl.Remaining)
		return true
	} else {
		log.Printf("[RateLimit] Sleep until %s\n", rl.ResetTime)
		time.Sleep(time.Until(rl.ResetTime))
		rl.Ready = false
		return false
	}
}

func makeRateLimit(header *http.Header) *xRateLimit {
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
	return &xRateLimit{
		ResetTime: time.Unix(int64(resetTimeNum), 0),
		Remaining: remainingNum,
		Limit:     limitNum,
		Ready:     true,
	}
}

func EnableRateLimit(client *resty.Client) {
	limiters := make(map[string]*xRateLimit)
	mutexs := sync.Map{}

	client.OnBeforeRequest(func(c *resty.Client, req *resty.Request) error {
		// 对一个 URL 初始化
		u, err := url.Parse(req.URL)
		if err != nil {
			return err
		}
		mut, _ := mutexs.LoadOrStore(u.Path, &sync.Mutex{})
		mutex := mut.(*sync.Mutex)

		mutex.Lock()
		limiter, exist := limiters[u.Path]
		if !exist {
			limiters[u.Path] = &xRateLimit{}
			limiter = limiters[u.Path]
		}

		if limiter == nil || limiter.Req() {
			mutex.Unlock()
		}
		return nil
	})

	client.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error {
		u, err := url.Parse(resp.Request.URL)
		if err != nil {
			return err
		}

		limiter := limiters[u.Path] // 绝对能获取到
		if limiter == nil || limiter.Ready {
			return nil
		}

		// 不获取互斥量，仅在速率限制未就绪的情况下使其就绪并解锁
		mut, _ := mutexs.Load(u.Path)
		mutex := mut.(*sync.Mutex)
		// 重置速率限制
		header := resp.Header()
		newLimiter := makeRateLimit(&header)
		limiters[u.Path] = newLimiter
		mutex.Unlock()
		return nil
	})
}
