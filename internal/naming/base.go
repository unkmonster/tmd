package naming

import (
	"github.com/unkmonster/tmd/internal/utils"
)

const ExtReserveLen = 8

var MaxFileNameLen = utils.DefaultMaxFileNameLen

type baseNaming struct {
	sanitized string
}
