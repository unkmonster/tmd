package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDownloadOptions_Struct(t *testing.T) {
	opts := DownloadOptions{
		AutoFollow:  true,
		SkipProfile: false,
		NoRetry:     true,
	}

	assert.True(t, opts.AutoFollow)
	assert.False(t, opts.SkipProfile)
	assert.True(t, opts.NoRetry)
}

func TestDownloadOptions_DefaultValues(t *testing.T) {
	opts := DownloadOptions{}

	assert.False(t, opts.AutoFollow)
	assert.False(t, opts.SkipProfile)
	assert.False(t, opts.NoRetry)
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
