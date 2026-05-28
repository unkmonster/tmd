package cli

import (
	"bytes"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/unkmonster/tmd/internal/utils"
)

// MaxNumericID keeps CLI IDs in the positive int64 range used by database and
// entity code while still allowing current 19-digit Twitter/X snowflake IDs.
const MaxNumericID = uint64(math.MaxInt64)

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
	if id > MaxNumericID {
		return fmt.Errorf("invalid ID: must be less than or equal to %d", MaxNumericID)
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

func addPathArg(paths *[]string, flagName, str string) error {
	if *paths == nil {
		*paths = make([]string, 0)
	}
	if strings.TrimSpace(str) == "" {
		return fmt.Errorf("%s path cannot be empty", flagName)
	}
	*paths = append(*paths, str)
	return nil
}

func pathArgsString(paths []string) string {
	return strings.Join(paths, ",")
}

func pathArgsPaths(paths []string) []string {
	return paths
}

// JsonFilePathsArgs 第三方工具JSON文件路径参数（-jsonfile）
type JsonFilePathsArgs struct {
	Paths []string
}

func (j *JsonFilePathsArgs) Set(str string) error {
	return addPathArg(&j.Paths, "jsonfile", str)
}

func (j *JsonFilePathsArgs) String() string {
	return pathArgsString(j.Paths)
}

func (j *JsonFilePathsArgs) GetPaths() []string {
	return pathArgsPaths(j.Paths)
}

// JsonFolderPathArgs TMD loongtweet文件夹路径参数（-jsonfolder）
type JsonFolderPathArgs struct {
	Paths []string
}

func (j *JsonFolderPathArgs) Set(str string) error {
	return addPathArg(&j.Paths, "jsonfolder", str)
}

func (j *JsonFolderPathArgs) String() string {
	return pathArgsString(j.Paths)
}

func (j *JsonFolderPathArgs) GetPaths() []string {
	return pathArgsPaths(j.Paths)
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
	var flagOutput bytes.Buffer
	fs.SetOutput(&flagOutput)
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
		if err == flag.ErrHelp {
			_, _ = os.Stderr.Write(flagOutput.Bytes())
		}
		return nil, friendlyFlagError(err)
	}
	if err := validateMarkTime(cfg.MarkTime); err != nil {
		return nil, err
	}

	return cfg, nil
}

func friendlyFlagError(err error) error {
	if err == nil || err == flag.ErrHelp {
		return err
	}

	msg := err.Error()
	const unknownPrefix = "flag provided but not defined: "
	if strings.HasPrefix(msg, unknownPrefix) {
		flagName := strings.TrimSpace(strings.TrimPrefix(msg, unknownPrefix))
		return fmt.Errorf("unknown CLI flag %s; run tmd -help to see supported download flags", flagName)
	}

	const valuePrefix = "flag needs an argument: "
	if strings.HasPrefix(msg, valuePrefix) {
		flagName := strings.TrimSpace(strings.TrimPrefix(msg, valuePrefix))
		return fmt.Errorf("CLI flag %s requires a value", flagName)
	}

	return err
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
