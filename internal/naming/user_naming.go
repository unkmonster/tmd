package naming

import (
	"github.com/unkmonster/tmd/internal/utils"
)

type UserNaming struct {
	sanitized string
}

func NewUserNaming(name, screenName string) *UserNaming {
	title := name + "(" + screenName + ")"
	return &UserNaming{
		sanitized: utils.WinFileNameWithMaxLen(title, MaxFileNameLen),
	}
}

func (un *UserNaming) SanitizedTitle() string {
	return un.sanitized
}
