package twitter

import (
	"github.com/tidwall/gjson"
)

const (
	ErrTimeout         = 29
	ErrDependency      = 0
	ErrExceedPostLimit = 88
	ErrOverCapacity    = 130
	ErrAccountLocked   = 326
)

func CheckApiResp(body []byte) error {
	errors := gjson.GetBytes(body, "errors")
	if !errors.Exists() {
		return nil
	}

	codej := errors.Get("0.code")
	code := -1
	if codej.Exists() {
		code = int(codej.Int())
	}
	return NewTwitterApiError(code, string(body))
}

type TwitterApiError struct {
	Code int
	raw  string
}

func (err *TwitterApiError) Error() string {
	return err.raw
}

func NewTwitterApiError(code int, raw string) *TwitterApiError {
	return &TwitterApiError{Code: code, raw: raw}
}
