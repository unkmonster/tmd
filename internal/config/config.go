package config

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unkmonster/tmd/internal/utils"
	"gopkg.in/yaml.v3"
)

const (
	MinFileNameLen = 50
	MaxFileNameLen = 250
)

// DefaultMaxDownloadRoutine 返回默认的最大下载并发数
// 基于 CPU 核心数计算，但不超过 100
func DefaultMaxDownloadRoutine() int {
	return min(100, runtime.GOMAXPROCS(0)*10)
}

type Cookie struct {
	AuthToken string `yaml:"auth_token"`
	Ct0       string `yaml:"ct0"`
}

type Config struct {
	RootPath           string `yaml:"root_path"`
	Cookie             Cookie `yaml:"cookie"`
	MaxDownloadRoutine int    `yaml:"max_download_routine"`
	MaxFileNameLen     int    `yaml:"max_file_name_len"`
	ProxyURL           string `yaml:"proxy_url"`
}

type envFieldBinding struct {
	envName   string
	fieldName string
}

var envFieldBindings = []envFieldBinding{
	{envName: "TMD_ROOT_PATH", fieldName: "root_path"},
	{envName: "TMD_AUTH_TOKEN", fieldName: "auth_token"},
	{envName: "TMD_CT0", fieldName: "ct0"},
	{envName: "TMD_PROXY_URL", fieldName: "proxy_url"},
	{envName: "TMD_MAX_DOWNLOAD_ROUTINE", fieldName: "max_download_routine"},
	{envName: "TMD_MAX_FILE_NAME_LEN", fieldName: "max_file_name_len"},
}

// FieldDef 字段定义，包含 getter/setter 实现双向绑定
type FieldDef struct {
	Name    string                      // 字段名（用于日志和状态显示）
	Prompt  string                      // 提示文本
	Default string                      // 默认值（新配置或零值时使用）
	Getter  func(*Config) string        // 获取当前值
	Setter  func(*Config, string) error // 设置值并验证
}

func GetFieldDefs() []FieldDef {
	return []FieldDef{
		{
			Name:    "root_path",
			Prompt:  "enter storage dir",
			Default: func() string { return "" }(),
			Getter:  func(c *Config) string { return c.RootPath },
			Setter: func(c *Config, v string) error {
				if strings.TrimSpace(v) == "" {
					return fmt.Errorf("storage dir cannot be empty")
				}
				if err := os.MkdirAll(v, 0755); err != nil {
					return err
				}
				absPath, err := filepath.Abs(v)
				if err != nil {
					return err
				}
				c.RootPath = absPath
				return nil
			},
		},
		{
			Name:    "auth_token",
			Prompt:  "enter auth_token",
			Default: func() string { return "" }(),
			Getter:  func(c *Config) string { return c.Cookie.AuthToken },
			Setter:  func(c *Config, v string) error { c.Cookie.AuthToken = v; return nil },
		},
		{
			Name:    "ct0",
			Prompt:  "enter ct0",
			Default: func() string { return "" }(),
			Getter:  func(c *Config) string { return c.Cookie.Ct0 },
			Setter:  func(c *Config, v string) error { c.Cookie.Ct0 = v; return nil },
		},
		{
			Name:    "max_download_routine",
			Prompt:  "enter max download routine",
			Default: strconv.Itoa(DefaultMaxDownloadRoutine()),
			Getter: func(c *Config) string {
				if c.MaxDownloadRoutine == 0 {
					return strconv.Itoa(DefaultMaxDownloadRoutine())
				}
				return strconv.Itoa(c.MaxDownloadRoutine)
			},
			Setter: func(c *Config, v string) error {
				n, err := parseIntWithDefault(v, DefaultMaxDownloadRoutine())
				if err != nil {
					return fmt.Errorf("invalid number: %w", err)
				}
				c.MaxDownloadRoutine = n
				return nil
			},
		},
		{
			Name:    "max_file_name_len",
			Prompt:  fmt.Sprintf("enter max file name length (%d-%d)", MinFileNameLen, MaxFileNameLen),
			Default: strconv.Itoa(utils.DefaultMaxFileNameLen),
			Getter: func(c *Config) string {
				if c.MaxFileNameLen == 0 {
					return strconv.Itoa(utils.DefaultMaxFileNameLen)
				}
				return strconv.Itoa(c.MaxFileNameLen)
			},
			Setter: func(c *Config, v string) error {
				n, err := parseIntWithDefault(v, utils.DefaultMaxFileNameLen)
				if err != nil {
					return fmt.Errorf("invalid number: %w", err)
				}
				c.MaxFileNameLen = clampInt(n, MinFileNameLen, MaxFileNameLen)
				return nil
			},
		},
		{
			Name:    "proxy_url",
			Prompt:  "enter proxy url (e.g., http://127.0.0.1:7897, leave empty for system proxy)",
			Default: func() string { return "" }(),
			Getter:  func(c *Config) string { return c.ProxyURL },
			Setter: func(c *Config, v string) error {
				proxyURL, err := normalizeProxyURL(v)
				if err != nil {
					return err
				}
				c.ProxyURL = proxyURL
				return nil
			},
		},
	}
}

func normalizeProxyURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", nil
	}
	switch parsed.Scheme {
	case "http", "https", "socks5":
		return parsed.String(), nil
	default:
		return "", nil
	}
}

func HasEnvOverrides() bool {
	for _, binding := range envFieldBindings {
		if strings.TrimSpace(os.Getenv(binding.envName)) != "" {
			return true
		}
	}
	return false
}

func ApplyEnv(conf *Config) (bool, error) {
	if conf == nil {
		return false, fmt.Errorf("config is nil")
	}

	fieldDefs := GetFieldDefs()
	fieldsByName := make(map[string]FieldDef, len(fieldDefs))
	for _, field := range fieldDefs {
		fieldsByName[field.Name] = field
	}

	next := *conf
	applied := false
	for _, binding := range envFieldBindings {
		rawValue := strings.TrimSpace(os.Getenv(binding.envName))
		if rawValue == "" {
			continue
		}

		field, ok := fieldsByName[binding.fieldName]
		if !ok {
			return false, fmt.Errorf("config field %q for %s is not registered", binding.fieldName, binding.envName)
		}
		if err := field.Setter(&next, rawValue); err != nil {
			return false, fmt.Errorf("invalid %s: %w", binding.envName, err)
		}
		applied = true
	}
	if applied {
		*conf = next
	}
	return applied, nil
}

// GetFieldValue 获取字段当前值（通过 FieldDef.Getter）
func GetFieldValue(conf *Config, field FieldDef) string {
	if field.Getter != nil {
		val := field.Getter(conf)
		if val == "" {
			return field.Default
		}
		return val
	}
	return field.Default
}

// parseIntWithDefault 解析整数，空值返回默认值
func parseIntWithDefault(input string, defaultVal int) (int, error) {
	v := strings.TrimSpace(input)
	if v == "" {
		return defaultVal, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// clampInt 将值限制在 [min, max] 范围内
func clampInt(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func ReadConf(path string) (*Config, error) {
	var result Config
	err := readYAMLFile(path, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func WriteConf(path string, conf *Config) error {
	return writeYAMLFile(path, conf)
}

// PromptConfig 交互式配置（参考 tmd-2.4.4 简化实现）
func PromptConfig(saveto string) (*Config, error) {
	conf := &Config{}
	scan := bufio.NewScanner(os.Stdin)

	for _, field := range GetFieldDefs() {
		fmt.Fprintf(os.Stderr, "%s [%s]: ", field.Prompt, field.Default)
		if !scan.Scan() {
			if err := scan.Err(); err != nil {
				return nil, fmt.Errorf("failed to read input for %s: %w", field.Name, err)
			}
			return nil, fmt.Errorf("unexpected EOF while reading %s", field.Name)
		}
		input := scan.Text()

		if err := field.Setter(conf, input); err != nil {
			return nil, fmt.Errorf("failed to set %s: %w", field.Name, err)
		}
	}

	if err := backupConf(saveto); err != nil {
		log.Warnf("Failed to backup config: %v", err)
	}

	return conf, WriteConf(saveto, conf)
}

// backupConf 备份现有配置文件（与 API 模式一致）
func backupConf(confPath string) error {
	data, err := os.ReadFile(confPath)
	if err != nil {
		return nil
	}
	backupPath := confPath + ".backup." + strconv.FormatInt(time.Now().Unix(), 10)
	return os.WriteFile(backupPath, data, 0600)
}

func ReadAdditionalCookies(path string) ([]*Cookie, error) {
	res := []*Cookie{}
	err := readYAMLFile(path, &res)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

func WriteAdditionalCookies(path string, cookies []*Cookie) error {
	return writeYAMLFile(path, cookies)
}

func readYAMLFile(path string, out interface{}) error {
	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, out)
}

func writeYAMLFile(path string, in interface{}) error {
	file, err := os.OpenFile(path, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := yaml.Marshal(in)
	if err != nil {
		return err
	}
	_, err = io.Copy(file, bytes.NewReader(data))
	return err
}
