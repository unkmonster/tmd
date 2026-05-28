package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBootstrapArgs(t *testing.T) {
	parsed, err := parseBootstrapArgs([]string{
		"-server",
		"-port", "8080",
		"-dbg",
		"-user", "alice",
		"-jsonfile", "export.json",
	})

	require.NoError(t, err)
	assert.True(t, parsed.serverMode)
	assert.True(t, parsed.dbg)
	assert.True(t, parsed.serverPortSet)
	assert.Equal(t, 8080, parsed.serverPort)
	assert.Equal(t, []string{"-user", "alice", "-jsonfile", "export.json"}, parsed.cliArgs)
}

func TestParseBootstrapArgsConfIsBoolean(t *testing.T) {
	parsed, err := parseBootstrapArgs([]string{"-conf", "extra.yaml", "-dbg"})

	require.NoError(t, err)
	assert.True(t, parsed.confArg)
	assert.True(t, parsed.dbg)
	assert.Equal(t, []string{"extra.yaml"}, parsed.cliArgs)
}

func TestParseBootstrapArgsPortValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing", args: []string{"-port"}},
		{name: "next flag", args: []string{"-port", "-dbg"}},
		{name: "not number", args: []string{"-port", "abc"}},
		{name: "zero", args: []string{"-port", "0"}},
		{name: "too large", args: []string{"-port", "65536"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseBootstrapArgs(tt.args)
			assert.Error(t, err)
		})
	}
}

func TestParseBootstrapArgsPassesUnknownFlagsToCLI(t *testing.T) {
	parsed, err := parseBootstrapArgs([]string{"-unknown", "value", "-user", "alice"})

	require.NoError(t, err)
	assert.Equal(t, []string{"-unknown", "value", "-user", "alice"}, parsed.cliArgs)
}
