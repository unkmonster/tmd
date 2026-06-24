package twitter

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
)

type AccountCookie struct {
	AuthToken string
	Ct0       string
}

// BatchLoginOptions 批量登录选项
type BatchLoginOptions struct {
	Debug bool
}

func BatchLogin(ctx context.Context, opts BatchLoginOptions, cookies []AccountCookie, master string) []*resty.Client {
	if len(cookies) == 0 {
		return nil
	}

	added := sync.Map{}
	msgs := make([]string, len(cookies))
	clients := []*resty.Client{}
	wg := sync.WaitGroup{}
	mtx := sync.Mutex{}
	added.Store(master, struct{}{})

	loginOpts := LoginOptions{}

	for i, cookie := range cookies {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			cli, sn, err := LoginWithOptions(ctx, cookie.AuthToken, cookie.Ct0, loginOpts)
			if err != nil {
				msgs[index] = fmt.Sprintf("    - ? %v\n", err)
				return
			}
			if _, loaded := added.LoadOrStore(sn, struct{}{}); loaded {
				msgs[index] = "    - ? repeated\n"
				return
			}
			EnableRateLimit(cli)
			if opts.Debug {
				EnableRequestCounting(cli)
			}
			mtx.Lock()
			defer mtx.Unlock()
			clients = append(clients, cli)
			msgs[index] = fmt.Sprintf("    - %s\n", sn)
		}(i)
	}

	wg.Wait()
	log.Infoln("[twitter] Loaded additional accounts:", len(clients))
	for _, msg := range msgs {
		fmt.Print(msg)
	}
	return clients
}
