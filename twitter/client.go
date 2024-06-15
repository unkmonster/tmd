package twitter

import (
	"fmt"
	"net/http"
	"strings"
)

type AddHeaderTransport struct {
	auth      string
	cookieStr string
	csrfToker string
}

func (c *AddHeaderTransport) SetAuth(auth string) {
	c.auth = auth
}

func (c *AddHeaderTransport) SetCookie(cookie string) error {
	c.cookieStr = cookie
	begin := strings.Index(cookie, "ct0=")
	if begin == -1 {
		return fmt.Errorf("invalid cookie str")
	}
	end := strings.Index(cookie[begin:], ";")
	if end == -1 {
		return fmt.Errorf("invalid cookie str")
	}
	c.csrfToker = cookie[begin:]
	c.csrfToker = c.csrfToker[:end]
	return nil
}

func (adt *AddHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	//req.Header.Add("User-Agent", "go")
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	req.Header.Add("Cookie", adt.cookieStr)
	req.Header.Add("Authorization", adt.auth)
	req.Header.Add("X-Csrf-Token", adt.csrfToker)
	return http.DefaultTransport.RoundTrip(req)
}
