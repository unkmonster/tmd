package config

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestMaxFileNameLenFieldUsesConfigDefault(t *testing.T) {
	want := strconv.Itoa(DefaultMaxFileNameLen())

	for _, field := range GetFieldDefs() {
		if field.Name != "max_file_name_len" {
			continue
		}

		assert.Equal(t, want, field.Default)
		assert.Equal(t, want, field.Getter(&Config{}))

		conf := &Config{}
		assert.NoError(t, field.Setter(conf, ""))
		assert.Equal(t, DefaultMaxFileNameLen(), conf.MaxFileNameLen)
		return
	}

	t.Fatal("max_file_name_len field not found")
}
