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

// FieldDef 字段定义，包含 getter/setter 实现双向绑定
type FieldDef struct {
	Name    string                      // 字段名（用于日志和状态显示）
	EnvName string                      // 环境变量名（用于支持环境变量覆盖配置）
	Prompt  string                      // 提示文本
	Default string                      // 默认值（新配置或零值时使用）
	Getter  func(*Config) string        // 获取当前值
	Setter  func(*Config, string) error // 设置值并验证
}

type StartupLoadResult struct {
	Config          *Config
	UsedEnvFallback bool
	EnvApplied      bool
	Prompted        bool
}

func GetFieldDefs() []FieldDef {
	return []FieldDef{
		{
			Name:    "root_path",
			EnvName: "TMD_ROOT_PATH",
			Prompt:  "enter storage dir",
			Default: "",
			Getter:  func(c *Config) string { return c.RootPath },
			Setter: func(c *Config, v string) error {
				absPath, err := normalizeRootPath(v)
				if err != nil {
					return err
				}
				c.RootPath = absPath
				return nil
			},
		},
		{
			Name:    "auth_token",
			EnvName: "TMD_AUTH_TOKEN",
			Prompt:  "enter auth_token",
			Default: "",
			Getter:  func(c *Config) string { return c.Cookie.AuthToken },
			Setter:  func(c *Config, v string) error { c.Cookie.AuthToken = v; return nil },
		},
		{
			Name:    "ct0",
			EnvName: "TMD_CT0",
			Prompt:  "enter ct0",
			Default: "",
			Getter:  func(c *Config) string { return c.Cookie.Ct0 },
			Setter:  func(c *Config, v string) error { c.Cookie.Ct0 = v; return nil },
		},
		{
			Name:    "max_download_routine",
			EnvName: "TMD_MAX_DOWNLOAD_ROUTINE",
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
			EnvName: "TMD_MAX_FILE_NAME_LEN",
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
			EnvName: "TMD_PROXY_URL",
			Prompt:  "enter proxy url (e.g., http://127.0.0.1:7897, leave empty for system proxy)",
			Default: "",
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
	if !strings.Contains(raw, "://") {
		return "", fmt.Errorf("invalid proxy URL %q: missing scheme or host", raw)
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid proxy URL %q: %w", raw, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid proxy URL %q: missing scheme or host", raw)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "socks5":
		return parsed.String(), nil
	default:
		return "", fmt.Errorf("invalid proxy URL %q: unsupported scheme %q", raw, parsed.Scheme)
	}
}

func normalizeRootPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("storage dir cannot be empty")
	}
	if err := os.MkdirAll(raw, 0755); err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	return absPath, nil
}

func HasEnvOverrides() bool {
	for _, field := range GetFieldDefs() {
		if strings.TrimSpace(os.Getenv(field.EnvName)) != "" {
			return true
		}
	}
	return false
}

func ApplyEnv(conf *Config) (bool, error) {
	if conf == nil {
		return false, fmt.Errorf("config is nil")
	}

	next := *conf
	applied := false
	for _, field := range GetFieldDefs() {
		rawValue := strings.TrimSpace(os.Getenv(field.EnvName))
		if rawValue == "" {
			continue
		}
		if err := field.Setter(&next, rawValue); err != nil {
			return false, fmt.Errorf("invalid %s: %w", field.EnvName, err)
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
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseConfYAML(data)
}

func WriteConf(path string, conf *Config) error {
	data, err := MarshalConf(conf)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0600)
}

func ParseConfYAML(data []byte) (*Config, error) {
	var result Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&result); err != nil {
		if err == io.EOF {
			return &Config{}, nil
		}
		return nil, err
	}
	if err := NormalizeLoadedConf(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func MarshalConf(conf *Config) ([]byte, error) {
	if conf == nil {
		return nil, fmt.Errorf("config is nil")
	}

	next := *conf
	if err := NormalizeLoadedConf(&next); err != nil {
		return nil, err
	}
	return yaml.Marshal(&next)
}

func NormalizeLoadedConf(conf *Config) error {
	if conf == nil {
		return fmt.Errorf("config is nil")
	}

	conf.RootPath = strings.TrimSpace(conf.RootPath)
	if conf.RootPath != "" {
		absPath, err := normalizeRootPath(conf.RootPath)
		if err != nil {
			return fmt.Errorf("invalid root_path: %w", err)
		}
		conf.RootPath = absPath
	}

	proxyURL, err := normalizeProxyURL(conf.ProxyURL)
	if err != nil {
		return fmt.Errorf("invalid proxy_url: %w", err)
	}
	conf.ProxyURL = proxyURL
	return nil
}

func Validate(conf *Config) error {
	if conf == nil {
		return fmt.Errorf("config is nil")
	}
	if strings.TrimSpace(conf.RootPath) == "" {
		return fmt.Errorf("root_path is required; set it in conf.yaml or TMD_ROOT_PATH")
	}
	return nil
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

func LoadStartupConfig(path string, prompt bool, stderr io.Writer) (*StartupLoadResult, error) {
	return loadStartupConfig(path, prompt, stderr, PromptConfig)
}

func loadStartupConfig(path string, prompt bool, stderr io.Writer, promptFn func(string) (*Config, error)) (*StartupLoadResult, error) {
	result := &StartupLoadResult{}

	conf, err := ReadConf(path)
	if os.IsNotExist(err) {
		if prompt || !HasEnvOverrides() {
			if stderr != nil {
				_, _ = io.WriteString(stderr, "Config file not found, creating new configuration...\n")
			}
			conf, err = promptFn(path)
			if err != nil {
				return nil, err
			}
			result.Prompted = true
		} else {
			conf = &Config{}
			result.UsedEnvFallback = true
		}
	} else if err != nil {
		return nil, err
	} else if prompt {
		conf, err = promptFn(path)
		if err != nil {
			return nil, err
		}
		result.Prompted = true
	}

	if !prompt {
		applied, err := ApplyEnv(conf)
		if err != nil {
			return nil, err
		}
		result.EnvApplied = applied
	}

	result.Config = conf
	return result, nil
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
	data, err := yaml.Marshal(in)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0600)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}

	tmpPath := file.Name()
	success := false
	defer func() {
		file.Close()
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := file.Chmod(perm); err != nil {
		return err
	}
	if _, err := io.Copy(file, bytes.NewReader(data)); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}

	success = true
	return nil
}
