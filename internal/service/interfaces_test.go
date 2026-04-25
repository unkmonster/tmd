package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDownloadOptions_Struct(t *testing.T) {
	markTime := "2024-01-01T00:00:00"
	opts := DownloadOptions{
		AutoFollow:  true,
		SkipProfile: false,
		NoRetry:     true,
		MarkTime:    &markTime,
	}

	assert.True(t, opts.AutoFollow)
	assert.False(t, opts.SkipProfile)
	assert.True(t, opts.NoRetry)
	assert.NotNil(t, opts.MarkTime)
	assert.Equal(t, "2024-01-01T00:00:00", *opts.MarkTime)
}

func TestDownloadOptions_NilMarkTime(t *testing.T) {
	opts := DownloadOptions{
		AutoFollow:  false,
		SkipProfile: true,
		NoRetry:     false,
		MarkTime:    nil,
	}

	assert.False(t, opts.AutoFollow)
	assert.True(t, opts.SkipProfile)
	assert.False(t, opts.NoRetry)
	assert.Nil(t, opts.MarkTime)
}

func TestDownloadOptions_DefaultValues(t *testing.T) {
	opts := DownloadOptions{}

	assert.False(t, opts.AutoFollow)
	assert.False(t, opts.SkipProfile)
	assert.False(t, opts.NoRetry)
	assert.Nil(t, opts.MarkTime)
}

func TestDownloadOptions_AllTrue(t *testing.T) {
	opts := DownloadOptions{
		AutoFollow:  true,
		SkipProfile: true,
		NoRetry:     true,
	}

	assert.True(t, opts.AutoFollow)
	assert.True(t, opts.SkipProfile)
	assert.True(t, opts.NoRetry)
}

func TestDownloadOptions_AllFalse(t *testing.T) {
	opts := DownloadOptions{
		AutoFollow:  false,
		SkipProfile: false,
		NoRetry:     false,
	}

	assert.False(t, opts.AutoFollow)
	assert.False(t, opts.SkipProfile)
	assert.False(t, opts.NoRetry)
}
