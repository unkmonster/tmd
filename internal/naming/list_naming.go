package naming

import (
	"github.com/unkmonster/tmd/internal/utils"
)

type ListNamer interface {
	GetId() int64
	Title() string
}

type ListNaming struct {
	sanitized string
}

func NewListNamingFromBase(lst ListNamer) *ListNaming {
	return &ListNaming{
		sanitized: utils.WinFileNameWithMaxLen(lst.Title(), MaxFileNameLen),
	}
}

func (ln *ListNaming) SanitizedTitle() string {
	return ln.sanitized
}
