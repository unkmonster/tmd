package utils

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/go-resty/resty/v2"
)

func GetExtFromUrl(u string) (string, error) {
	pu, err := url.Parse(u)
	if err != nil {
		return "", err
	}

	return filepath.Ext(pu.Path), nil
}

func CheckRespStatus(resp *resty.Response) error {
	if resp.StatusCode() != 200 {
		return &HttpStatusError{Code: resp.StatusCode(), Msg: resp.String()}
	}
	return nil
}

func ParseCookie(cookie string) (map[string]string, error) {
	results := make(map[string]string)
	splited := strings.Split(cookie, ";")
	for _, item := range splited {
		item = strings.TrimSpace(item)
		kv := strings.SplitN(item, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("len(kv) should be 2 but '%v'", item)
		}
		results[kv[0]] = kv[1]
	}
	return results, nil
}

type HttpStatusError struct {
	Code int
	Msg  string
}

func (err *HttpStatusError) Error() string {
	return fmt.Sprintf("%d %s", err.Code, err.Msg)
}

func IsStatusCode(err error, code int) bool {
	e, ok := err.(*HttpStatusError)
	if !ok {
		return false
	}
	return e.Code == code
}
