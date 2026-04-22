package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type FieldUpdater struct {
	Name   string
	Prompt string
	Getter func(*Config) string
	Setter func(*Config, string) error
}

func GetFieldUpdaters() []FieldUpdater {
	return []FieldUpdater{
		{
			Name:   "root_path",
			Prompt: "enter storage dir",
			Getter: func(c *Config) string { return c.RootPath },
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
			Name:   "auth_token",
			Prompt: "enter auth_token",
			Getter: func(c *Config) string { return c.Cookie.AuthToken },
			Setter: func(c *Config, v string) error {
				c.Cookie.AuthToken = v
				return nil
			},
		},
		{
			Name:   "ct0",
			Prompt: "enter ct0",
			Getter: func(c *Config) string { return c.Cookie.Ct0 },
			Setter: func(c *Config, v string) error {
				c.Cookie.Ct0 = v
				return nil
			},
		},
		{
			Name:   "max_download_routine",
			Prompt: "enter max download routine",
			Getter: func(c *Config) string {
				if c.MaxDownloadRoutine == 0 {
					return "35"
				}
				return strconv.Itoa(c.MaxDownloadRoutine)
			},
			Setter: func(c *Config, v string) error {
				if strings.TrimSpace(v) == "" {
					return nil
				}
				routine, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("invalid max download routine: %v", err)
				}
				c.MaxDownloadRoutine = routine
				return nil
			},
		},
		{
			Name:   "max_file_name_len",
			Prompt: "enter max file name length (50-250, default 158)",
			Getter: func(c *Config) string {
				if c.MaxFileNameLen == 0 {
					return "158"
				}
				return strconv.Itoa(c.MaxFileNameLen)
			},
			Setter: func(c *Config, v string) error {
				if strings.TrimSpace(v) == "" {
					return nil
				}
				length, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("invalid max file name length: %v", err)
				}
				if length < 50 {
					length = 50
				}
				if length > 250 {
					length = 250
				}
				c.MaxFileNameLen = length
				return nil
			},
		},
	}
}

func PromptPartialConfig(saveto string) (*Config, error) {
	conf, err := ReadConf(saveto)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Config file not found, creating new configuration...")
			conf = &Config{}
		} else {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	if conf == nil {
		conf = &Config{}
	}

	scan := bufio.NewScanner(os.Stdin)

	getInputOrKeep := func(prompt string, currentValue string) string {
		fmt.Printf("%s [%s]: ", prompt, currentValue)
		scan.Scan()
		input := scan.Text()
		if strings.TrimSpace(input) == "" {
			return currentValue
		}
		return input
	}

	for _, updater := range GetFieldUpdaters() {
		currentValue := updater.Getter(conf)
		newValue := getInputOrKeep(updater.Prompt, currentValue)

		if newValue != currentValue {
			if err := updater.Setter(conf, newValue); err != nil {
				return nil, fmt.Errorf("failed to update %s: %w", updater.Name, err)
			}
			fmt.Printf("  -> %s updated\n", updater.Name)
		} else {
			fmt.Printf("  -> %s unchanged\n", updater.Name)
		}
	}

	return conf, WriteConf(saveto, conf)
}
