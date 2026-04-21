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

type Cookie struct {
	AuthToken string `yaml:"auth_token"`
	Ct0       string `yaml:"ct0"`
}

type Config struct {
	RootPath           string `yaml:"root_path"`
	Cookie             Cookie `yaml:"cookie"`
	MaxDownloadRoutine int    `yaml:"max_download_routine"`
	MaxFileNameLen     int    `yaml:"max_file_name_len"`
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

func PromptConfig(saveto string) (*Config, error) {
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

	getInputOrDefault := func(prompt string, defaultValue string) string {
		fmt.Printf("%s [%s]: ", prompt, defaultValue)
		scan.Scan()
		input := scan.Text()
		if strings.TrimSpace(input) == "" {
			return defaultValue
		}
		return input
	}

	storePath := getInputOrDefault("enter storage dir", conf.RootPath)
	if strings.TrimSpace(storePath) == "" {
		return nil, fmt.Errorf("storage dir cannot be empty")
	}
	err = os.MkdirAll(storePath, 0755)
	if err != nil {
		return nil, err
	}
	storePath, err = filepath.Abs(storePath)
	if err != nil {
		return nil, err
	}
	conf.RootPath = storePath

	conf.Cookie.AuthToken = getInputOrDefault("enter auth_token", conf.Cookie.AuthToken)
	conf.Cookie.Ct0 = getInputOrDefault("enter ct0", conf.Cookie.Ct0)

	routineStr := getInputOrDefault("enter max download routine", strconv.Itoa(conf.MaxDownloadRoutine))
	if strings.TrimSpace(routineStr) != "" {
		routine, err := strconv.Atoi(routineStr)
		if err != nil {
			return nil, fmt.Errorf("invalid max download routine: %v", err)
		}
		conf.MaxDownloadRoutine = routine
	}

	fileNameLenStr := getInputOrDefault("enter max file name length (50-250)", strconv.Itoa(conf.MaxFileNameLen))
	if strings.TrimSpace(fileNameLenStr) != "" {
		length, err := strconv.Atoi(fileNameLenStr)
		if err != nil {
			return nil, fmt.Errorf("invalid max file name length: %v", err)
		}
		if length < 50 {
			length = 50
		}
		if length > 250 {
			length = 250
		}
		conf.MaxFileNameLen = length
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
