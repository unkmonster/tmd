package naming

import (
	"fmt"
	"path/filepath"
	"unicode/utf8"

	"github.com/unkmonster/tmd/internal/utils"
)

type TweetNaming struct {
	sanitized string
	tweetID   uint64
	creator   string
	maxLen    int
}

func NewTweetNaming(text string, tweetID uint64, creator string, maxLen int) *TweetNaming {
	return &TweetNaming{
		sanitized: utils.WinFileNameWithMaxLen(text, maxLen),
		tweetID:   tweetID,
		creator:   creator,
		maxLen:    maxLen,
	}
}

func (tn *TweetNaming) baseName() string {
	idPart := fmt.Sprintf("_%d", tn.tweetID)
	maxTextLen := tn.maxLen - len(idPart) - ExtReserveLen
	if maxTextLen < 0 {
		maxTextLen = 0
	}

	text := tn.sanitized
	if len(text) > maxTextLen {
		truncateAt := maxTextLen
		for truncateAt > 0 && !utf8.RuneStart(text[truncateAt]) {
			truncateAt--
		}
		text = text[:truncateAt]
	}
	if text == "" {
		text = "tweet"
	}

	return text + idPart
}

func (tn *TweetNaming) LogFormat() string {
	return fmt.Sprintf("[%s] %s", tn.creator, tn.baseName())
}

func (tn *TweetNaming) FileName(ext string) string {
	return tn.baseName() + ext
}

func (tn *TweetNaming) FilePath(dir string, ext string) (string, error) {
	fullPath := filepath.Join(dir, tn.FileName(ext))
	return utils.UniquePath(fullPath)
}

func (tn *TweetNaming) FilePathWithResolver(dir string, ext string, resolver *utils.UniquePathResolver) (string, error) {
	fullPath := filepath.Join(dir, tn.FileName(ext))
	return resolver.UniquePath(fullPath)
}
