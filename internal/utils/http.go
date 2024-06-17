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
		return fmt.Errorf("%s: %s", resp.Status(), resp.String())
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
			return nil, fmt.Errorf("kv should be 2 but '%v'", item)
		}
		results[kv[0]] = kv[1]
	}
	return results, nil
}
