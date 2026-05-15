package naming

import (
	"github.com/unkmonster/tmd/internal/utils"
)

type ListNaming struct {
	sanitized string
}

func NewListNamingFromBase(lst interface {
	GetId() int64
	Title() string
}) *ListNaming {
	return &ListNaming{
		sanitized: utils.WinFileNameWithMaxLen(lst.Title(), MaxFileNameLen),
	}
}

func (ln *ListNaming) SanitizedTitle() string {
	return ln.sanitized
}
