package config

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// 配置默认值常量
const (
	DefaultMaxDownloadRoutine = 35
	DefaultMaxFileNameLen     = 158
	MinFileNameLen            = 50
	MaxFileNameLen            = 250
)

type Cookie struct {
	AuthToken string `yaml:"auth_token"`
	Ct0       string `yaml:"ct0"`
}

// FieldDef 字段定义
type FieldDef struct {
	Name    string                      // 字段名
	Prompt  string                      // 提示文本
	Default string                      // 默认值
	Setter  func(*Config, string) error // 自定义设置逻辑
}

type Config struct {
	RootPath           string `yaml:"root_path"`
	Cookie             Cookie `yaml:"cookie"`
	MaxDownloadRoutine int    `yaml:"max_download_routine"`
	MaxFileNameLen     int    `yaml:"max_file_name_len"`
	ProxyURL           string `yaml:"proxy_url"`
}

// GetFieldDefs 返回所有字段定义
func GetFieldDefs() []FieldDef {
	return []FieldDef{
		{
			Name:    "root_path",
			Prompt:  "enter storage dir",
			Default: "",
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
			Default: "",
			Setter: func(c *Config, v string) error {
				c.Cookie.AuthToken = v
				return nil
			},
		},
		{
			Name:    "ct0",
			Prompt:  "enter ct0",
			Default: "",
			Setter: func(c *Config, v string) error {
				c.Cookie.Ct0 = v
				return nil
			},
		},
		{
			Name:    "max_download_routine",
			Prompt:  "enter max download routine",
			Default: strconv.Itoa(DefaultMaxDownloadRoutine),
			Setter: func(c *Config, v string) error {
				if strings.TrimSpace(v) == "" {
					c.MaxDownloadRoutine = DefaultMaxDownloadRoutine
					return nil
				}
				n, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("invalid number: %v", err)
				}
				c.MaxDownloadRoutine = n
				return nil
			},
		},
		{
			Name:    "max_file_name_len",
			Prompt:  fmt.Sprintf("enter max file name length (%d-%d)", MinFileNameLen, MaxFileNameLen),
			Default: strconv.Itoa(DefaultMaxFileNameLen),
			Setter: func(c *Config, v string) error {
				if strings.TrimSpace(v) == "" {
					c.MaxFileNameLen = DefaultMaxFileNameLen
					return nil
				}
				n, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("invalid number: %v", err)
				}
				if n < MinFileNameLen {
					n = MinFileNameLen
				}
				if n > MaxFileNameLen {
					n = MaxFileNameLen
				}
				c.MaxFileNameLen = n
				return nil
			},
		},
		{
			Name:    "proxy_url",
			Prompt:  "enter proxy url (e.g., http://127.0.0.1:7897, leave empty for system proxy)",
			Default: "",
			Setter: func(c *Config, v string) error {
				c.ProxyURL = v
				return nil
			},
		},
	}
}

// getFieldValue 获取字段当前值
func getFieldValue(conf *Config, field FieldDef) string {
	switch field.Name {
	case "root_path":
		return conf.RootPath
	case "auth_token":
		return conf.Cookie.AuthToken
	case "ct0":
		return conf.Cookie.Ct0
	case "max_download_routine":
		if conf.MaxDownloadRoutine == 0 {
			return field.Default
		}
		return strconv.Itoa(conf.MaxDownloadRoutine)
	case "max_file_name_len":
		if conf.MaxFileNameLen == 0 {
			return field.Default
		}
		return strconv.Itoa(conf.MaxFileNameLen)
	case "proxy_url":
		return conf.ProxyURL
	default:
		return field.Default
	}
}

func ReadConf(path string) (*Config, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var result Config
	err = yaml.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func WriteConf(path string, conf *Config) error {
	file, err := os.OpenFile(path, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := yaml.Marshal(conf)
	if err != nil {
		return err
	}
	_, err = io.Copy(file, bytes.NewReader(data))
	return err
}

// PromptConfig 统一的配置交互
// partialMode: true 显示"已更新/未变更"提示，false 不显示
func PromptConfig(saveto string, partialMode bool) (*Config, error) {
	conf, err := ReadConf(saveto)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Config file not found, creating new configuration...")
		} else {
			backupPath := saveto + ".backup." + strconv.FormatInt(time.Now().Unix(), 10)
			if renameErr := os.Rename(saveto, backupPath); renameErr != nil {
				fmt.Printf("Warning: failed to read existing config (%v)\n", err)
				fmt.Printf("Failed to backup config file: %v\n", renameErr)
				fmt.Println("Starting fresh without backup...")
			} else {
				fmt.Printf("Warning: existing config file is corrupted (%v)\n", err)
				fmt.Printf("Original config has been backed up to: %s\n", backupPath)
				fmt.Println("Creating new configuration...")
			}
		}
		conf = &Config{}
	}

	scan := bufio.NewScanner(os.Stdin)

	for _, field := range GetFieldDefs() {
		currentValue := getFieldValue(conf, field)

		fmt.Printf("%s [%s]: ", field.Prompt, currentValue)
		if !scan.Scan() {
			if err := scan.Err(); err != nil {
				return nil, fmt.Errorf("failed to read input for %s: %w", field.Name, err)
			}
			// EOF reached
			return nil, fmt.Errorf("unexpected EOF while reading %s", field.Name)
		}
		input := scan.Text()

		// 如果用户直接回车，使用当前值
		if strings.TrimSpace(input) == "" {
			input = currentValue
		}

		// 应用设置
		if err := field.Setter(conf, input); err != nil {
			return nil, fmt.Errorf("failed to set %s: %w", field.Name, err)
		}

		// 部分模式显示更新状态
		if partialMode && input != currentValue {
			fmt.Printf("  -> %s updated\n", field.Name)
		} else if partialMode {
			fmt.Printf("  -> %s unchanged\n", field.Name)
		}
	}

	return conf, WriteConf(saveto, conf)
}

func ReadAdditionalCookies(path string) ([]*Cookie, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var res []*Cookie
	return res, yaml.Unmarshal(data, &res)
}
