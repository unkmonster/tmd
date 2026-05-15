package config

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unkmonster/tmd/internal/utils"
)

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, binding := range envFieldBindings {
		t.Setenv(binding.envName, "")
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
