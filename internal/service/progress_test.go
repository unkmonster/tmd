package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProgress_Struct(t *testing.T) {
	p := Progress{
		Stage:     "downloading",
		Total:     100,
		Completed: 50,
		Failed:    5,
		Current:   "test_user",
	}

	assert.Equal(t, "downloading", p.Stage)
	assert.Equal(t, 100, p.Total)
	assert.Equal(t, 50, p.Completed)
	assert.Equal(t, 5, p.Failed)
	assert.Equal(t, "test_user", p.Current)
}

func TestProgress_AllStages(t *testing.T) {
	stages := []string{
		"syncing",
		"downloading",
		"retrying",
		"profile",
		"profile_warning",
		"marking",
		"completed",
	}

	for _, stage := range stages {
		p := Progress{Stage: stage}
		assert.Equal(t, stage, p.Stage)
	}
}

func TestProgress_ZeroValues(t *testing.T) {
	p := Progress{}

	assert.Empty(t, p.Stage)
	assert.Equal(t, 0, p.Total)
	assert.Equal(t, 0, p.Completed)
	assert.Equal(t, 0, p.Failed)
	assert.Empty(t, p.Current)
}

func TestProgress_NegativeValues(t *testing.T) {
	p := Progress{
		Total:     -1,
		Completed: -10,
		Failed:    -5,
	}

	assert.Equal(t, -1, p.Total)
	assert.Equal(t, -10, p.Completed)
	assert.Equal(t, -5, p.Failed)
}

func TestResult_Struct(t *testing.T) {
	r := Result{
		Main: &MainResult{
			Downloaded: 100,
			Failed:     5,
		},
		Profile: &ProfileResult{
			Downloaded: 8,
			Failed:     1,
			Versioned:  10,
		},
		Message: "Download completed successfully",
	}

	require := assert.New(t)
	if require.NotNil(r.Main) {
		require.Equal(100, r.Main.Downloaded)
		require.Equal(5, r.Main.Failed)
	}
	if require.NotNil(r.Profile) {
		require.Equal(8, r.Profile.Downloaded)
		require.Equal(1, r.Profile.Failed)
		require.Equal(10, r.Profile.Versioned)
	}
	assert.Equal(t, "Download completed successfully", r.Message)
}

func TestResult_ZeroValues(t *testing.T) {
	r := Result{}

	assert.Nil(t, r.Main)
	assert.Nil(t, r.Profile)
	assert.Empty(t, r.Message)
}

func TestResult_OnlyMessage(t *testing.T) {
	r := Result{
		Message: "Test message",
	}

	assert.Nil(t, r.Main)
	assert.Nil(t, r.Profile)
	assert.Equal(t, "Test message", r.Message)
}

func TestNopReporter_OnProgress(t *testing.T) {
	reporter := &NopReporter{}

	p := Progress{
		Stage:     "downloading",
		Total:     100,
		Completed: 50,
		Current:   "test_user",
	}

	reporter.OnProgress("task-123", p)
}

func TestNopReporter_OnComplete(t *testing.T) {
	reporter := &NopReporter{}

	r := Result{
		Main: &MainResult{
			Downloaded: 100,
			Failed:     5,
		},
		Message: "Completed",
	}

	reporter.OnComplete("task-123", r)
}

func TestNopReporter_OnError(t *testing.T) {
	reporter := &NopReporter{}

	err := errors.New("test error")
	reporter.OnError("task-123", err)
}

func TestNewLogReporter(t *testing.T) {
	var loggedMessages []string
	logger := func(format string, args ...interface{}) {
		loggedMessages = append(loggedMessages, format)
	}

	reporter := NewLogReporter(logger)

	assert.NotNil(t, reporter)

	logReporter, ok := reporter.(*LogReporter)
	assert.True(t, ok)
	assert.NotNil(t, logReporter.logger)
}

func TestNewLogReporter_NilLogger(t *testing.T) {
	reporter := NewLogReporter(nil)

	assert.NotNil(t, reporter)

	logReporter, ok := reporter.(*LogReporter)
	assert.True(t, ok)
	assert.Nil(t, logReporter.logger)
}

func TestLogReporter_OnProgress_Syncing(t *testing.T) {
	var loggedMessages []string
	logger := func(format string, args ...interface{}) {
		loggedMessages = append(loggedMessages, format)
	}

	reporter := NewLogReporter(logger)
	reporter.OnProgress("task-123", Progress{Stage: "syncing", Current: "user1"})

	assert.Len(t, loggedMessages, 1)
	assert.Contains(t, loggedMessages[0], "Syncing")
}

func TestLogReporter_OnProgress_DownloadingIsSuppressed(t *testing.T) {
	var loggedMessages []string
	logger := func(format string, args ...interface{}) {
		loggedMessages = append(loggedMessages, format)
	}

	reporter := NewLogReporter(logger)
	reporter.OnProgress("task-123", Progress{
		Stage:     "downloading",
		Total:     100,
		Completed: 50,
		Current:   "user1",
	})

	assert.Len(t, loggedMessages, 0)
}

func TestLogReporter_OnProgress_Profile(t *testing.T) {
	var loggedMessages []string
	logger := func(format string, args ...interface{}) {
		loggedMessages = append(loggedMessages, format)
	}

	reporter := NewLogReporter(logger)
	reporter.OnProgress("task-123", Progress{Stage: "profile"})

	assert.Len(t, loggedMessages, 1)
	assert.Contains(t, loggedMessages[0], "profiles")
}

func TestLogReporter_OnProgress_Marking(t *testing.T) {
	var loggedMessages []string
	logger := func(format string, args ...interface{}) {
		loggedMessages = append(loggedMessages, format)
	}

	reporter := NewLogReporter(logger)
	reporter.OnProgress("task-123", Progress{Stage: "marking", Current: "user1"})

	assert.Len(t, loggedMessages, 1)
	assert.Contains(t, loggedMessages[0], "Marking")
}

func TestLogReporter_OnProgress_DefaultStage(t *testing.T) {
	var loggedMessages []string
	logger := func(format string, args ...interface{}) {
		loggedMessages = append(loggedMessages, format)
	}

	reporter := NewLogReporter(logger)
	reporter.OnProgress("task-123", Progress{Stage: "custom_stage", Current: "value"})

	assert.Len(t, loggedMessages, 1)
}

func TestLogReporter_OnProgress_NilLogger(t *testing.T) {
	reporter := NewLogReporter(nil)

	reporter.OnProgress("task-123", Progress{Stage: "downloading"})
}

func TestLogReporter_OnComplete_WithStats(t *testing.T) {
	var loggedMessages []string
	var loggedArgs []interface{}
	logger := func(format string, args ...interface{}) {
		loggedMessages = append(loggedMessages, format)
		loggedArgs = append(loggedArgs, args...)
	}

	reporter := NewLogReporter(logger)
	reporter.OnComplete("task-123", Result{
		Main: &MainResult{
			Downloaded: 100,
			Failed:     5,
		},
		Profile: &ProfileResult{
			Downloaded: 12,
			Failed:     1,
			Versioned:  10,
		},
		Message: "User download completed",
	})

	assert.Len(t, loggedMessages, 1)
	assert.Equal(t, "[%s] Completed (%s)", loggedMessages[0])
	assert.Equal(t, "task-123", loggedArgs[0])
	assert.Equal(t, "main(downloaded=100, Failedtweet=5), profile(downloaded=12, failed=1, versionedfile=10)", loggedArgs[1])
}

func TestLogReporter_OnComplete_WithoutStats(t *testing.T) {
	var loggedMessages []string
	var loggedArgs []interface{}
	logger := func(format string, args ...interface{}) {
		loggedMessages = append(loggedMessages, format)
		loggedArgs = append(loggedArgs, args...)
	}

	reporter := NewLogReporter(logger)
	reporter.OnComplete("task-123", Result{Message: "Done"})

	assert.Len(t, loggedMessages, 1)
	assert.Equal(t, "[%s] Completed: %s", loggedMessages[0])
	// 检查 args 中是否包含 "Done"
	found := false
	for _, arg := range loggedArgs {
		if s, ok := arg.(string); ok && s == "Done" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected 'Done' in logged args")
}

func TestLogReporter_OnComplete_NilLogger(t *testing.T) {
	reporter := NewLogReporter(nil)

	reporter.OnComplete("task-123", Result{Message: "Done"})
}

func TestLogReporter_OnError(t *testing.T) {
	var loggedMessages []string
	logger := func(format string, args ...interface{}) {
		loggedMessages = append(loggedMessages, format)
	}

	reporter := NewLogReporter(logger)
	err := errors.New("test error")
	reporter.OnError("task-123", err)

	assert.Len(t, loggedMessages, 1)
	assert.Contains(t, loggedMessages[0], "Error")
}

func TestLogReporter_OnError_NilLogger(t *testing.T) {
	reporter := NewLogReporter(nil)

	err := errors.New("test error")
	reporter.OnError("task-123", err)
}

func TestLogReporter_OnError_NilError(t *testing.T) {
	var loggedMessages []string
	logger := func(format string, args ...interface{}) {
		loggedMessages = append(loggedMessages, format)
	}

	reporter := NewLogReporter(logger)
	reporter.OnError("task-123", nil)

	assert.Len(t, loggedMessages, 1)
}

func TestProgressReporter_Interface(t *testing.T) {
	var reporter ProgressReporter

	reporter = &NopReporter{}
	assert.NotNil(t, reporter)

	reporter.OnProgress("task-1", Progress{})
	reporter.OnComplete("task-1", Result{})
	reporter.OnError("task-1", nil)
}
