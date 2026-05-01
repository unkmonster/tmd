package api

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/unkmonster/tmd/internal/service"
)

func TestNewSSEProgressReporter(t *testing.T) {
	server := &Server{}
	reporter := NewSSEProgressReporter(server)

	assert.NotNil(t, reporter)

	sseReporter, ok := reporter.(*SSEProgressReporter)
	assert.True(t, ok)
	assert.Equal(t, server, sseReporter.server)
}

func TestSSEProgressReporter_OnProgress(t *testing.T) {
	tm := NewTaskManager()
	server := &Server{
		taskManager: tm,
	}

	task := tm.CreateTask(TaskTypeUserDownload, nil)

	reporter := NewSSEProgressReporter(server)

	progress := service.Progress{
		Stage:     "downloading",
		Total:     100,
		Completed: 50,
		Failed:    5,
		Current:   "user1",
	}

	reporter.OnProgress(task.ID, progress)

	updatedTask, ok := tm.GetTask(task.ID)
	assert.True(t, ok)
	assert.NotNil(t, updatedTask.Progress)
	assert.Equal(t, 100, updatedTask.Progress.Total)
	assert.Equal(t, 50, updatedTask.Progress.Completed)
	assert.Equal(t, 5, updatedTask.Progress.Failed)
}

func TestSSEProgressReporter_OnProgress_NotFound(t *testing.T) {
	server := &Server{
		taskManager: NewTaskManager(),
	}

	reporter := NewSSEProgressReporter(server)

	progress := service.Progress{
		Total:     100,
		Completed: 50,
	}

	reporter.OnProgress("non_existent_task", progress)
}

func TestSSEProgressReporter_OnComplete(t *testing.T) {
	tm := NewTaskManager()
	server := &Server{
		taskManager: tm,
	}

	task := tm.CreateTask(TaskTypeUserDownload, nil)
	tm.UpdateTaskStatus(task.ID, TaskStatusRunning)

	reporter := NewSSEProgressReporter(server)

	result := service.Result{
		Main: &service.MainResult{
			Downloaded: 95,
			Failed:     5,
		},
		Profile: &service.ProfileResult{
			Downloaded: 12,
			Failed:     1,
			Versioned:  10,
		},
		Message: "Download completed successfully",
	}

	reporter.OnComplete(task.ID, result)

	updatedTask, ok := tm.GetTask(task.ID)
	assert.True(t, ok)
	assert.Equal(t, TaskStatusCompleted, updatedTask.Status)
	assert.NotNil(t, updatedTask.Result)
	assert.NotNil(t, updatedTask.Result.Main)
	assert.NotNil(t, updatedTask.Result.Profile)
	assert.Equal(t, 95, updatedTask.Result.Main.Downloaded)
	assert.Equal(t, 5, updatedTask.Result.Main.Failed)
	assert.Equal(t, 10, updatedTask.Result.Profile.Versioned)
	assert.Equal(t, "Download completed successfully", updatedTask.Result.Message)
}

func TestSSEProgressReporter_OnError(t *testing.T) {
	tm := NewTaskManager()
	server := &Server{
		taskManager: tm,
	}

	task := tm.CreateTask(TaskTypeUserDownload, nil)
	tm.UpdateTaskStatus(task.ID, TaskStatusRunning)

	reporter := NewSSEProgressReporter(server)

	testErr := errors.New("download failed: network error")

	reporter.OnError(task.ID, testErr)

	updatedTask, ok := tm.GetTask(task.ID)
	assert.True(t, ok)
	assert.Equal(t, TaskStatusFailed, updatedTask.Status)
	assert.Equal(t, testErr.Error(), updatedTask.Error)
	assert.NotNil(t, updatedTask.EndedAt)
}

func TestSSEProgressReporter_MultipleProgressUpdates(t *testing.T) {
	tm := NewTaskManager()
	server := &Server{
		taskManager: tm,
	}

	task := tm.CreateTask(TaskTypeUserDownload, nil)
	reporter := NewSSEProgressReporter(server)

	stages := []struct {
		progress service.Progress
	}{
		{service.Progress{Stage: "syncing", Total: 100, Completed: 0, Current: "starting"}},
		{service.Progress{Stage: "downloading", Total: 100, Completed: 25, Current: "user1"}},
		{service.Progress{Stage: "downloading", Total: 100, Completed: 50, Current: "user2"}},
		{service.Progress{Stage: "downloading", Total: 100, Completed: 75, Current: "user3"}},
		{service.Progress{Stage: "completed", Total: 100, Completed: 100, Current: "done"}},
	}

	for _, s := range stages {
		reporter.OnProgress(task.ID, s.progress)
	}

	updatedTask, _ := tm.GetTask(task.ID)
	assert.Equal(t, 100, updatedTask.Progress.Completed)
	assert.Equal(t, 100, updatedTask.Progress.Total)
}

func TestProgress_Struct(t *testing.T) {
	p := TaskProgress{
		Stage:     "downloading",
		Total:     100,
		Completed: 75,
		Failed:    5,
		Current:   "current_user",
	}

	data, err := json.Marshal(p)
	assert.NoError(t, err)

	var decoded TaskProgress
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, p, decoded)
}

func TestResult_Struct(t *testing.T) {
	r := TaskResult{
		Main: &TaskMainResult{
			Downloaded: 100,
			Failed:     5,
		},
		Profile: &TaskProfileResult{
			Downloaded: 8,
			Failed:     1,
			Versioned:  10,
		},
		Message: "Task completed",
	}

	data, err := json.Marshal(r)
	assert.NoError(t, err)

	var decoded TaskResult
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, r, decoded)
}

func TestSSEProgressReporter_InterfaceCompliance(t *testing.T) {
	var _ service.ProgressReporter = (*SSEProgressReporter)(nil)
}

func TestSSEProgressReporter_CompleteWorkflow(t *testing.T) {
	tm := NewTaskManager()
	server := &Server{
		taskManager: tm,
	}

	task := tm.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{ScreenName: "testuser"})

	reporter := NewSSEProgressReporter(server)

	reporter.OnProgress(task.ID, service.Progress{
		Stage:     "syncing",
		Total:     100,
		Completed: 0,
		Current:   "starting",
	})

	reporter.OnProgress(task.ID, service.Progress{
		Stage:     "downloading",
		Total:     100,
		Completed: 50,
		Current:   "halfway",
	})

	reporter.OnComplete(task.ID, service.Result{
		Main: &service.MainResult{
			Downloaded: 100,
			Failed:     0,
		},
		Message: "All done",
	})

	finalTask, _ := tm.GetTask(task.ID)
	assert.Equal(t, TaskStatusCompleted, finalTask.Status)
	assert.NotNil(t, finalTask.Result.Main)
	assert.Equal(t, 100, finalTask.Result.Main.Downloaded)
	assert.Equal(t, "All done", finalTask.Result.Message)
}

func TestSSEProgressReporter_ErrorWorkflow(t *testing.T) {
	tm := NewTaskManager()
	server := &Server{
		taskManager: tm,
	}

	task := tm.CreateTask(TaskTypeUserDownload, nil)
	reporter := NewSSEProgressReporter(server)

	reporter.OnProgress(task.ID, service.Progress{
		Total:     100,
		Completed: 30,
	})

	testErr := errors.New("network timeout")
	reporter.OnError(task.ID, testErr)

	finalTask, _ := tm.GetTask(task.ID)
	assert.Equal(t, TaskStatusFailed, finalTask.Status)
	assert.Equal(t, testErr.Error(), finalTask.Error)
	assert.Equal(t, 30, finalTask.Progress.Completed)
}
