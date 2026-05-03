package config

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unkmonster/tmd/internal/utils"
)

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

		assert.Error(t, field.Setter(conf, "127.0.0.1:7897"))
		assert.Error(t, field.Setter(conf, "ftp://127.0.0.1:21"))
		return
	}

	t.Fatal("proxy_url field not found")
}
