package cli

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/unkmonster/tmd/internal/utils"
)

// UserArgs 用户参数（只支持 ScreenName）
type UserArgs struct {
	ScreenName []string
}

func (u *UserArgs) Set(str string) error {
	if u.ScreenName == nil {
		u.ScreenName = make([]string, 0)
	}

	str = utils.NormalizeScreenName(str)

	// 校验 screenName 格式
	if !utils.IsValidScreenName(str) {
		return fmt.Errorf("invalid screen name format: %s", str)
	}
	for _, existing := range u.ScreenName {
		if existing == str {
			return fmt.Errorf("duplicate screen name: %s", str)
		}
	}

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

	// 校验 ID 有效性（必须大于 0）
	if id == 0 {
		return fmt.Errorf("invalid ID: must be greater than 0")
	}
	for _, existing := range i.ID {
		if existing == id {
			return fmt.Errorf("duplicate ID: %d", id)
		}
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

// JsonFilePathsArgs 第三方工具JSON文件路径参数（-jsonfile）
type JsonFilePathsArgs struct {
	Paths []string
}

func (j *JsonFilePathsArgs) Set(str string) error {
	if j.Paths == nil {
		j.Paths = make([]string, 0)
	}
	if strings.TrimSpace(str) == "" {
		return fmt.Errorf("jsonfile path cannot be empty")
	}
	j.Paths = append(j.Paths, str)
	return nil
}

func (j *JsonFilePathsArgs) String() string {
	return strings.Join(j.Paths, ",")
}

func (j *JsonFilePathsArgs) GetPaths() []string {
	return j.Paths
}

// JsonFolderPathArgs TMD loongtweet文件夹路径参数（-jsonfolder）
type JsonFolderPathArgs struct {
	Paths []string
}

func (j *JsonFolderPathArgs) Set(str string) error {
	if j.Paths == nil {
		j.Paths = make([]string, 0)
	}
	if strings.TrimSpace(str) == "" {
		return fmt.Errorf("jsonfolder path cannot be empty")
	}
	j.Paths = append(j.Paths, str)
	return nil
}

func (j *JsonFolderPathArgs) String() string {
	return strings.Join(j.Paths, ",")
}

func (j *JsonFolderPathArgs) GetPaths() []string {
	return j.Paths
}

// CLIConfig CLI 配置
type CLIConfig struct {
	UsrArgs        UserArgs
	ListArgs       ListArgs
	FollArgs       UserArgs
	ProfileUsers   UserArgs
	ProfileList    ListArgs
	JsonFileArgs   JsonFilePathsArgs
	JsonFolderArgs JsonFolderPathArgs
	AutoFollow     bool
	FollowMembers  bool
	NoRetry        bool
	MarkDownloaded bool
	MarkTime       string
	NoProfile      bool
}

// ParseArgs 解析命令行参数
func ParseArgs(args []string) (*CLIConfig, error) {
	cfg := &CLIConfig{}

	fs := flag.NewFlagSet("tmd", flag.ContinueOnError)
	fs.Var(&cfg.UsrArgs, "user", "download tweets from the user")
	fs.Var(&cfg.ListArgs, "list", "batch download from list")
	fs.Var(&cfg.FollArgs, "foll", "batch download following")
	fs.Var(&cfg.ProfileUsers, "profile-user", "download profile")
	fs.Var(&cfg.ProfileList, "profile-list", "download list profiles")
	fs.Var(&cfg.JsonFileArgs, "jsonfile", "download from third-party tool exported JSON file (user list)")
	fs.Var(&cfg.JsonFolderArgs, "jsonfolder", "download from TMD generated .loongtweet folder")
	fs.BoolVar(&cfg.AutoFollow, "auto-follow", false, "auto follow")
	fs.BoolVar(&cfg.FollowMembers, "follow-members", false, "follow target users/members while downloading")
	fs.BoolVar(&cfg.NoRetry, "no-retry", false, "no retry")
	fs.BoolVar(&cfg.MarkDownloaded, "mark-downloaded", false, "mark downloaded")
	fs.StringVar(&cfg.MarkTime, "mark-time", "", "mark time")
	fs.BoolVar(&cfg.NoProfile, "noprofile", false, "skip profile")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if err := validateMarkTime(cfg.MarkTime); err != nil {
		return nil, err
	}

	return cfg, nil
}

func validateMarkTime(markTime string) error {
	if markTime == "" {
		return nil
	}
	if strings.EqualFold(markTime, "null") || strings.EqualFold(markTime, "nil") {
		return nil
	}
	if _, err := time.ParseInLocation("2006-01-02T15:04:05", markTime, time.Local); err != nil {
		return fmt.Errorf("invalid mark-time format %q, expected 2006-01-02T15:04:05 or null", markTime)
	}
	return nil
}
