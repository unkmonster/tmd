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

func NewListNamingFromBase(lst ListNamer, maxLen int) *ListNaming {
	return &ListNaming{
		sanitized: utils.WinFileNameWithMaxLen(lst.Title(), maxLen),
	}
}

func (ln *ListNaming) SanitizedTitle() string {
	return ln.sanitized
}
