package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unkmonster/tmd/internal/utils"
)

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, field := range GetFieldDefs() {
		t.Setenv(field.EnvName, "")
	}
}

func TestMaxDownloadRoutineFieldUsesConfigDefault(t *testing.T) {
	want := strconv.Itoa(DefaultMaxDownloadRoutine())

	for _, field := range GetFieldDefs() {
		if field.Name != "max_download_routine" {
			continue
		}

		assert.Equal(t, want, field.Default)
		assert.Equal(t, want, field.Getter(&Config{}))

		conf := &Config{}
		assert.NoError(t, field.Setter(conf, ""))
		assert.Equal(t, DefaultMaxDownloadRoutine(), conf.MaxDownloadRoutine)
		return
	}

	t.Fatal("max_download_routine field not found")
}

func TestMaxFileNameLenFieldUsesUtilsDefault(t *testing.T) {
	want := strconv.Itoa(utils.DefaultMaxFileNameLen)

	for _, field := range GetFieldDefs() {
		if field.Name != "max_file_name_len" {
			continue
		}

		assert.Equal(t, want, field.Default)
		assert.Equal(t, want, field.Getter(&Config{}))

		conf := &Config{}
		assert.NoError(t, field.Setter(conf, ""))
		assert.Equal(t, utils.DefaultMaxFileNameLen, conf.MaxFileNameLen)
		return
	}

	t.Fatal("max_file_name_len field not found")
}

func TestProxyURLFieldNormalizesAndValidates(t *testing.T) {
	for _, field := range GetFieldDefs() {
		if field.Name != "proxy_url" {
			continue
		}

		conf := &Config{}
		assert.NoError(t, field.Setter(conf, "  http://127.0.0.1:7897  "))
		assert.Equal(t, "http://127.0.0.1:7897", conf.ProxyURL)

		assert.NoError(t, field.Setter(conf, ""))
		assert.Equal(t, "", conf.ProxyURL)

		assert.NoError(t, field.Setter(conf, "socks5://127.0.0.1:7890"))
		assert.Equal(t, "socks5://127.0.0.1:7890", conf.ProxyURL)

		err := field.Setter(conf, "127.0.0.1:7897")
		assert.EqualError(t, err, `invalid proxy URL "127.0.0.1:7897": missing scheme or host`)
		assert.Equal(t, "socks5://127.0.0.1:7890", conf.ProxyURL)

		err = field.Setter(conf, "ftp://127.0.0.1:21")
		assert.EqualError(t, err, `invalid proxy URL "ftp://127.0.0.1:21": unsupported scheme "ftp"`)
		assert.Equal(t, "socks5://127.0.0.1:7890", conf.ProxyURL)
		return
	}

	t.Fatal("proxy_url field not found")
}

func TestApplyEnvOverridesConfig(t *testing.T) {
	clearConfigEnv(t)

	rootPath := filepath.Join(t.TempDir(), "downloads")
	t.Setenv("TMD_ROOT_PATH", rootPath)
	t.Setenv("TMD_AUTH_TOKEN", "env-auth")
	t.Setenv("TMD_CT0", "env-ct0")
	t.Setenv("TMD_PROXY_URL", "  http://127.0.0.1:7897  ")
	t.Setenv("TMD_MAX_DOWNLOAD_ROUTINE", "8")
	t.Setenv("TMD_MAX_FILE_NAME_LEN", "999")

	conf := &Config{
		RootPath:           "old-root",
		Cookie:             Cookie{AuthToken: "old-auth", Ct0: "old-ct0"},
		MaxDownloadRoutine: 1,
		MaxFileNameLen:     50,
		ProxyURL:           "socks5://127.0.0.1:7890",
	}

	applied, err := ApplyEnv(conf)
	assert.NoError(t, err)
	assert.True(t, applied)

	expectedRoot, err := filepath.Abs(rootPath)
	assert.NoError(t, err)
	assert.Equal(t, expectedRoot, conf.RootPath)
	assert.Equal(t, "env-auth", conf.Cookie.AuthToken)
	assert.Equal(t, "env-ct0", conf.Cookie.Ct0)
	assert.Equal(t, "http://127.0.0.1:7897", conf.ProxyURL)
	assert.Equal(t, 8, conf.MaxDownloadRoutine)
	assert.Equal(t, MaxFileNameLen, conf.MaxFileNameLen)
}

func TestApplyEnvIgnoresEmptyEnvironment(t *testing.T) {
	clearConfigEnv(t)

	conf := &Config{RootPath: "kept"}
	applied, err := ApplyEnv(conf)
	assert.NoError(t, err)
	assert.False(t, applied)
	assert.False(t, HasEnvOverrides())
	assert.Equal(t, "kept", conf.RootPath)
}

func TestApplyEnvInvalidNumber(t *testing.T) {
	clearConfigEnv(t)

	conf := &Config{MaxDownloadRoutine: 5}
	t.Setenv("TMD_MAX_DOWNLOAD_ROUTINE", "not-a-number")

	applied, err := ApplyEnv(conf)
	assert.Error(t, err)
	assert.False(t, applied)
	assert.Equal(t, 5, conf.MaxDownloadRoutine)
}

func TestApplyEnvDoesNotPartiallyApplyOnError(t *testing.T) {
	clearConfigEnv(t)

	rootPath := filepath.Join(t.TempDir(), "downloads")
	conf := &Config{
		RootPath:           "old-root",
		MaxDownloadRoutine: 5,
	}
	t.Setenv("TMD_ROOT_PATH", rootPath)
	t.Setenv("TMD_MAX_DOWNLOAD_ROUTINE", "bad")

	applied, err := ApplyEnv(conf)
	assert.Error(t, err)
	assert.False(t, applied)
	assert.Equal(t, "old-root", conf.RootPath)
	assert.Equal(t, 5, conf.MaxDownloadRoutine)
}

func TestApplyEnvInvalidProxyURL(t *testing.T) {
	clearConfigEnv(t)

	conf := &Config{ProxyURL: "http://127.0.0.1:7897"}
	t.Setenv("TMD_PROXY_URL", "ftp://127.0.0.1:21")

	applied, err := ApplyEnv(conf)
	assert.EqualError(t, err, `invalid TMD_PROXY_URL: invalid proxy URL "ftp://127.0.0.1:21": unsupported scheme "ftp"`)
	assert.False(t, applied)
	assert.Equal(t, "http://127.0.0.1:7897", conf.ProxyURL)
}

func TestReadConfNormalizesRuntimeFields(t *testing.T) {
	confPath := filepath.Join(t.TempDir(), "conf.yaml")
	rootPath := filepath.Join(t.TempDir(), "downloads")
	content := []byte("root_path: \"" + filepath.ToSlash(rootPath) + "\"\nproxy_url: \"  http://127.0.0.1:7897  \"\n")

	assert.NoError(t, os.WriteFile(confPath, content, 0600))

	conf, err := ReadConf(confPath)
	assert.NoError(t, err)
	if assert.NotNil(t, conf) {
		assert.Equal(t, rootPath, conf.RootPath)
		assert.Equal(t, "http://127.0.0.1:7897", conf.ProxyURL)
	}
	info, statErr := os.Stat(rootPath)
	assert.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

func TestReadConfRejectsUnknownFields(t *testing.T) {
	confPath := filepath.Join(t.TempDir(), "conf.yaml")
	assert.NoError(t, os.WriteFile(confPath, []byte("rootpath: test\n"), 0600))

	_, err := ReadConf(confPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "field rootpath not found")
}

func TestParseConfYAMLRejectsInvalidProxyURL(t *testing.T) {
	_, err := ParseConfYAML([]byte("root_path: test\nproxy_url: ftp://127.0.0.1:21\n"))
	assert.EqualError(t, err, `invalid proxy_url: invalid proxy URL "ftp://127.0.0.1:21": unsupported scheme "ftp"`)
}

func TestWriteConfPersistsNormalizedConfig(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "downloads")
	confPath := filepath.Join(dir, "conf.yaml")

	conf := &Config{
		RootPath: rootPath,
		ProxyURL: "  http://127.0.0.1:7897  ",
	}
	assert.NoError(t, WriteConf(confPath, conf))

	saved, err := ReadConf(confPath)
	assert.NoError(t, err)
	if assert.NotNil(t, saved) {
		assert.Equal(t, rootPath, saved.RootPath)
		assert.Equal(t, "http://127.0.0.1:7897", saved.ProxyURL)
	}

	data, err := os.ReadFile(confPath)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "proxy_url: http://127.0.0.1:7897")
}

func TestLoadStartupConfigUsesEnvFallbackWhenConfigMissing(t *testing.T) {
	clearConfigEnv(t)

	rootPath := filepath.Join(t.TempDir(), "downloads")
	t.Setenv("TMD_ROOT_PATH", rootPath)

	var promptCalled bool
	result, err := loadStartupConfig(
		filepath.Join(t.TempDir(), "missing-conf.yaml"),
		false,
		nil,
		func(string) (*Config, error) {
			promptCalled = true
			return &Config{}, nil
		},
	)
	assert.NoError(t, err)
	assert.False(t, promptCalled)
	if assert.NotNil(t, result) {
		assert.True(t, result.UsedEnvFallback)
		assert.True(t, result.EnvApplied)
		if assert.NotNil(t, result.Config) {
			assert.Equal(t, rootPath, result.Config.RootPath)
		}
	}
}

func TestLoadStartupConfigAppliesEnvOverridesToExistingConfig(t *testing.T) {
	clearConfigEnv(t)

	dir := t.TempDir()
	confPath := filepath.Join(dir, "conf.yaml")
	assert.NoError(t, os.WriteFile(confPath, []byte("root_path: "+filepath.ToSlash(dir)+"\nproxy_url: http://127.0.0.1:7897\n"), 0600))
	t.Setenv("TMD_PROXY_URL", "socks5://127.0.0.1:7890")

	result, err := loadStartupConfig(confPath, false, nil, func(string) (*Config, error) {
		t.Fatal("prompt should not be called when config exists and prompt mode is off")
		return nil, nil
	})
	assert.NoError(t, err)
	if assert.NotNil(t, result) {
		assert.False(t, result.UsedEnvFallback)
		assert.True(t, result.EnvApplied)
		if assert.NotNil(t, result.Config) {
			assert.Equal(t, "socks5://127.0.0.1:7890", result.Config.ProxyURL)
		}
	}
}

func TestLoadStartupConfigPromptModeUsesPromptFunction(t *testing.T) {
	clearConfigEnv(t)

	dir := t.TempDir()
	confPath := filepath.Join(dir, "conf.yaml")
	assert.NoError(t, os.WriteFile(confPath, []byte("root_path: "+filepath.ToSlash(dir)+"\n"), 0600))

	stderr := &bytes.Buffer{}
	result, err := loadStartupConfig(confPath, true, stderr, func(path string) (*Config, error) {
		assert.Equal(t, confPath, path)
		return &Config{RootPath: dir}, nil
	})
	assert.NoError(t, err)
	if assert.NotNil(t, result) {
		assert.True(t, result.Prompted)
		assert.False(t, result.EnvApplied)
		assert.False(t, result.UsedEnvFallback)
		if assert.NotNil(t, result.Config) {
			assert.Equal(t, dir, result.Config.RootPath)
		}
	}
	assert.Equal(t, "", stderr.String())
}

func TestValidateRejectsMissingRootPath(t *testing.T) {
	err := Validate(&Config{})
	assert.EqualError(t, err, "root_path is required; set it in conf.yaml or TMD_ROOT_PATH")
}

func TestValidateRejectsNilConfig(t *testing.T) {
	err := Validate(nil)
	assert.EqualError(t, err, "config is nil")
}

// ==================== normalizeInt 测试 ====================

func TestNormalizeInt_ReturnsZeroForNegatives(t *testing.T) {
	assert.Equal(t, 0, normalizeInt(-1, 0, 5, 100))
	assert.Equal(t, 0, normalizeInt(-100, 0, 5, 100))
	assert.Equal(t, 42, normalizeInt(-1, 42, 5, 100))
}

func TestNormalizeInt_ClampsAboveHi(t *testing.T) {
	assert.Equal(t, 100, normalizeInt(200, 0, 5, 100))
	assert.Equal(t, 50, normalizeInt(50, 0, 5, 50))
}

func TestNormalizeInt_ClampsBelowLo(t *testing.T) {
	assert.Equal(t, 5, normalizeInt(1, 0, 5, 100))
	assert.Equal(t, 5, normalizeInt(3, 0, 5, 100))
}

func TestNormalizeInt_PreservesZero(t *testing.T) {
	assert.Equal(t, 0, normalizeInt(0, 0, 5, 100))
}

func TestNormalizeInt_PreservesValidRange(t *testing.T) {
	assert.Equal(t, 10, normalizeInt(10, 0, 5, 100))
	assert.Equal(t, 5, normalizeInt(5, 0, 5, 100))
	assert.Equal(t, 100, normalizeInt(100, 0, 5, 100))
}

// ==================== AdditionalCookies 测试 ====================

func TestReadAdditionalCookies_FileNotExist(t *testing.T) {
	cookies, err := ReadAdditionalCookies(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	assert.NoError(t, err)
	assert.Nil(t, cookies)
}

func TestReadWriteAdditionalCookies_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "additional_cookies.yaml")

	input := []*Cookie{
		{AuthToken: "token1", Ct0: "ct01"},
		{AuthToken: "token2", Ct0: "ct02"},
	}
	err := WriteAdditionalCookies(path, input)
	assert.NoError(t, err)

	output, err := ReadAdditionalCookies(path)
	assert.NoError(t, err)
	require.Len(t, output, 2)
	assert.Equal(t, "token1", output[0].AuthToken)
	assert.Equal(t, "ct01", output[0].Ct0)
	assert.Equal(t, "token2", output[1].AuthToken)
	assert.Equal(t, "ct02", output[1].Ct0)
}

func TestReadAdditionalCookies_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	require.NoError(t, os.WriteFile(path, []byte("{bad yaml: ["), 0600))

	_, err := ReadAdditionalCookies(path)
	assert.Error(t, err)
}

func TestReadAdditionalCookies_WritesNonEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "additional_cookies.yaml")
	err := WriteAdditionalCookies(path, []*Cookie{})
	assert.NoError(t, err)

	data, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Greater(t, len(data), 0)
}

func TestWriteAdditionalCookies_CreatesNestedDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "cookies.yaml")

	err := WriteAdditionalCookies(path, []*Cookie{{AuthToken: "a", Ct0: "b"}})
	assert.NoError(t, err)

	_, err = os.Stat(path)
	assert.NoError(t, err)
}
