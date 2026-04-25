package cli

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/unkmonster/tmd/internal/twitter"
)

// UserArgs 用户参数（只支持 ScreenName）
type UserArgs struct {
	ScreenName []string
}

func (u *UserArgs) Set(str string) error {
	if u.ScreenName == nil {
		u.ScreenName = make([]string, 0)
	}

	str, _ = strings.CutPrefix(str, "@")
	u.ScreenName = append(u.ScreenName, str)
	return nil
}

func (u *UserArgs) String() string {
	return fmt.Sprintf("screenNames=%v", u.ScreenName)
}

// IntArgs 整数参数
type IntArgs struct {
	ID []uint64
}

func (i *IntArgs) Set(str string) error {
	if i.ID == nil {
		i.ID = make([]uint64, 0)
	}
	id, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		return err
	}
	i.ID = append(i.ID, id)
	return nil
}

func (i *IntArgs) String() string {
	return fmt.Sprintf("%v", i.ID)
}

// ListArgs 列表参数
type ListArgs struct {
	IntArgs
}

func (l ListArgs) GetList(ctx context.Context, client *resty.Client) ([]*twitter.List, error) {
	lists := []*twitter.List{}
	for _, id := range l.ID {
		list, err := twitter.GetLst(ctx, client, id)
		if err != nil {
			return nil, err
		}
		lists = append(lists, list)
	}
	return lists, nil
}

// JsonPathsArgs JSON 路径参数
type JsonPathsArgs struct {
	Paths []string
}

func (j *JsonPathsArgs) Set(str string) error {
	if j.Paths == nil {
		j.Paths = make([]string, 0)
	}
	j.Paths = append(j.Paths, str)
	return nil
}

func (j *JsonPathsArgs) String() string {
	return strings.Join(j.Paths, ",")
}

func (j *JsonPathsArgs) GetPaths() []string {
	return j.Paths
}

// CLIConfig CLI 配置
type CLIConfig struct {
	UsrArgs        UserArgs
	ListArgs       ListArgs
	FollArgs       UserArgs
	ProfileUsers   UserArgs
	ProfileList    ListArgs
	JsonArgs       JsonPathsArgs
	AutoFollow     bool
	NoRetry        bool
	MarkDownloaded bool
	MarkTime       string
	NoProfile      bool
}

// ParseArgs 解析命令行参数
func ParseArgs(args []string) (*flag.FlagSet, *CLIConfig, error) {
	cfg := &CLIConfig{
		UsrArgs:      UserArgs{},
		ListArgs:     ListArgs{},
		FollArgs:     UserArgs{},
		ProfileUsers: UserArgs{},
		ProfileList:  ListArgs{},
		JsonArgs:     JsonPathsArgs{},
	}

	fs := flag.NewFlagSet("tmd", flag.ContinueOnError)
	fs.Var(&cfg.UsrArgs, "user", "download tweets from the user")
	fs.Var(&cfg.ListArgs, "list", "batch download from list")
	fs.Var(&cfg.FollArgs, "foll", "batch download following")
	fs.Var(&cfg.ProfileUsers, "profile-user", "download profile")
	fs.Var(&cfg.ProfileList, "profile-list", "download list profiles")
	fs.Var(&cfg.JsonArgs, "json", "download from JSON")
	fs.BoolVar(&cfg.AutoFollow, "auto-follow", false, "auto follow")
	fs.BoolVar(&cfg.NoRetry, "no-retry", false, "no retry")
	fs.BoolVar(&cfg.MarkDownloaded, "mark-downloaded", false, "mark downloaded")
	fs.StringVar(&cfg.MarkTime, "mark-time", "", "mark time")
	fs.BoolVar(&cfg.NoProfile, "noprofile", false, "skip profile")

	if err := fs.Parse(args); err != nil {
		return nil, nil, err
	}

	return fs, cfg, nil
}
