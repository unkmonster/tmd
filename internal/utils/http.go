package utils

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
)

func GetExtFromUrl(u string) (string, error) {
	pu, err := url.Parse(u)
	if err != nil {
		return "", err
	}

	return filepath.Ext(pu.Path), nil
}

func CheckRespStatus(resp *http.Response) error {
	if resp.StatusCode != 200 {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%d: %s", resp.StatusCode, &msg)
	}
	return nil
}
