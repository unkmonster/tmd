package api

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewTaskManager(t *testing.T) {
	tm := NewTaskManager(nil)
	assert.NotNil(t, tm)
	assert.NotNil(t, tm.tasks)
	assert.Empty(t, tm.tasks)
}

func TestTaskManager_CreateTask(t *testing.T) {
	tm := NewTaskManager(nil)

	tests := []struct {
		name     string
		taskType TaskType
		data     interface{}
	}{
		{
			name:     "创建用户下载任务",
			taskType: TaskTypeUserDownload,
			data:     &UserDownloadTaskData{ScreenName: "testuser"},
		},
		{
			name:     "创建列表下载任务",
			taskType: TaskTypeListDownload,
			data:     &ListDownloadTaskData{ListID: 123},
		},
		{
			name:     "创建批量下载任务",
			taskType: TaskTypeBatchDownload,
			data:     &BatchDownloadTaskData{Users: []string{"user1"}},
		},
		{
			name:     "创建 Profile 下载任务",
			taskType: TaskTypeProfileDownload,
			data:     &ProfileDownloadTaskData{ScreenName: "profile_user"},
		},
		{
			name:     "创建关注下载任务",
			taskType: TaskTypeFollowingDownload,
			data:     &FollowingDownloadTaskData{ScreenName: "following_user"},
		},
		{
			name:     "创建标记已下载任务",
			taskType: TaskTypeMarkDownloaded,
			data:     &MarkDownloadedTaskData{ScreenName: "mark_user"},
		},
		{
			name:     "创建 JSON 文件下载任务",
			taskType: TaskTypeJsonFileDownload,
			data:     &JsonFileDownloadTaskData{Paths: []string{"/path/to/file.json"}},
		},
		{
			name:     "创建列表 Profile 任务",
			taskType: TaskTypeListProfile,
			data:     &ListProfileTaskData{ListID: 456},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := tm.CreateTask(tt.taskType, tt.data)

			assert.NotNil(t, task)
			assert.NotEmpty(t, task.ID)
			assert.True(t, strings.HasPrefix(task.ID, "task_"))
			assert.Equal(t, tt.taskType, task.Type)
			assert.Equal(t, TaskStatusQueued, task.Status)
			assert.Equal(t, tt.data, task.Data)
			assert.NotNil(t, task.Progress)
			assert.NotNil(t, task.Ctx)
			assert.NotNil(t, task.Cancel)
			assert.WithinDuration(t, time.Now(), task.CreatedAt, time.Second)
			assert.Nil(t, task.StartedAt)
			assert.Nil(t, task.EndedAt)
			assert.Empty(t, task.Error)
		})
	}
}

func TestTaskManager_CreateTask_ClonesTaskData(t *testing.T) {
	tm := NewTaskManager(nil)
	now := time.Now()
	data := &BatchDownloadTaskData{
		Users:          []string{"user1"},
		Lists:          []StringUint64{1},
		FollowingNames: []string{"following1"},
		AutoFollow:     true,
	}
	markData := &MarkDownloadedTaskData{
		ScreenName: "testuser",
		Timestamp:  &now,
	}

	task := tm.CreateTask(TaskTypeBatchDownload, data)
	markTask := tm.CreateTask(TaskTypeMarkDownloaded, markData)

	createdData, ok := task.Data.(*BatchDownloadTaskData)
	assert.True(t, ok)
	assert.NotSame(t, data, createdData)

	data.Users[0] = "mutated"
	data.Lists[0] = 99
	data.FollowingNames[0] = "changed"
	assert.Equal(t, "user1", createdData.Users[0])
	assert.Equal(t, StringUint64(1), createdData.Lists[0])
	assert.Equal(t, "following1", createdData.FollowingNames[0])

	createdMarkData, ok := markTask.Data.(*MarkDownloadedTaskData)
	assert.True(t, ok)
	assert.NotSame(t, markData, createdMarkData)
	assert.NotSame(t, markData.Timestamp, createdMarkData.Timestamp)

	expectedTimestamp := *createdMarkData.Timestamp
	*markData.Timestamp = markData.Timestamp.Add(time.Hour)
	assert.WithinDuration(t, expectedTimestamp, *createdMarkData.Timestamp, 0)
}

func TestTaskManager_GetTask(t *testing.T) {
	tm := NewTaskManager(nil)

	task := tm.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{ScreenName: "testuser"})

	got, ok := tm.GetTask(task.ID)
	assert.True(t, ok)
	assert.Equal(t, task.ID, got.ID)
	assert.Equal(t, task.Type, got.Type)
	assert.Equal(t, task.Status, got.Status)
	assert.NotSame(t, task, got)

	got, ok = tm.GetTask("non_existent_task")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestTaskManager_GetTask_ReturnsClone(t *testing.T) {
	tm := NewTaskManager(nil)
	task := tm.CreateTask(TaskTypeBatchDownload, &BatchDownloadTaskData{
		Users: []string{"user1"},
	})
	tm.UpdateTaskProgress(task.ID, &TaskProgress{Total: 10, Completed: 1})
	tm.SetTaskResult(task.ID, &TaskResult{
		Main: &TaskMainResult{
			Downloaded: 2,
			Failed:     1,
		},
		Profile: &TaskProfileResult{
			Downloaded: 3,
			Failed:     1,
			Versioned:  1,
		},
	})

	got, ok := tm.GetTask(task.ID)
	assert.True(t, ok)
	assert.NotSame(t, task, got)
	assert.NotSame(t, task.Progress, got.Progress)
	assert.NotSame(t, task.Result, got.Result)
	assert.NotSame(t, task.Result.Main, got.Result.Main)
	assert.NotSame(t, task.Result.Profile, got.Result.Profile)

	got.Status = TaskStatusCompleted
	got.Progress.Completed = 9
	got.Result.Main.Downloaded = 99
	got.Result.Profile.Versioned = 99
	gotData := got.Data.(*BatchDownloadTaskData)
	gotData.Users[0] = "mutated"

	again, ok := tm.GetTask(task.ID)
	assert.True(t, ok)
	assert.Equal(t, TaskStatusQueued, again.Status)
	assert.Equal(t, 1, again.Progress.Completed)
	assert.Equal(t, 2, again.Result.Main.Downloaded)
	assert.Equal(t, 1, again.Result.Profile.Versioned)
	assert.Equal(t, "user1", again.Data.(*BatchDownloadTaskData).Users[0])
}

func TestTaskManager_GetAllTasks(t *testing.T) {
	tm := NewTaskManager(nil)

	tasks := tm.GetAllTasks()
	assert.Empty(t, tasks)

	task1 := tm.CreateTask(TaskTypeUserDownload, nil)
	task2 := tm.CreateTask(TaskTypeListDownload, nil)
	task3 := tm.CreateTask(TaskTypeBatchDownload, nil)
	task1.CreatedAt = time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	task2.CreatedAt = time.Date(2024, 1, 1, 10, 1, 0, 0, time.UTC)
	task3.CreatedAt = time.Date(2024, 1, 1, 10, 2, 0, 0, time.UTC)

	tasks = tm.GetAllTasks()
	assert.Len(t, tasks, 3)
	assert.Equal(t, task3.ID, tasks[0].ID)
	assert.Equal(t, task2.ID, tasks[1].ID)
	assert.Equal(t, task1.ID, tasks[2].ID)

	taskIDs := make(map[string]bool)
	for _, task := range tasks {
		taskIDs[task.ID] = true
	}
	assert.True(t, taskIDs[task1.ID])
	assert.True(t, taskIDs[task2.ID])
	assert.True(t, taskIDs[task3.ID])
}

func TestTaskManager_GetAllTasks_DeepCopiesTaskData(t *testing.T) {
	tm := NewTaskManager(nil)
	now := time.Now()

	task := tm.CreateTask(TaskTypeBatchDownload, &BatchDownloadTaskData{
		Users:          []string{"user1"},
		Lists:          []StringUint64{42},
		FollowingNames: []string{"following1"},
	})
	markTask := tm.CreateTask(TaskTypeMarkDownloaded, &MarkDownloadedTaskData{
		ScreenName: "testuser",
		Timestamp:  &now,
	})

	tasks := tm.GetAllTasks()
	assert.Len(t, tasks, 2)

	var batchCopy *Task
	var markCopy *Task
	for _, copiedTask := range tasks {
		switch copiedTask.ID {
		case task.ID:
			batchCopy = copiedTask
		case markTask.ID:
			markCopy = copiedTask
		}
	}

	if assert.NotNil(t, batchCopy) {
		copiedData := batchCopy.Data.(*BatchDownloadTaskData)
		assert.NotSame(t, task, batchCopy)
		assert.NotSame(t, task.Data, copiedData)
		copiedData.Users[0] = "mutated"
		copiedData.Lists[0] = 99
		copiedData.FollowingNames[0] = "changed"

		original, _ := tm.GetTask(task.ID)
		originalData := original.Data.(*BatchDownloadTaskData)
		assert.Equal(t, "user1", originalData.Users[0])
		assert.Equal(t, StringUint64(42), originalData.Lists[0])
		assert.Equal(t, "following1", originalData.FollowingNames[0])
	}

	if assert.NotNil(t, markCopy) {
		copiedData := markCopy.Data.(*MarkDownloadedTaskData)
		assert.NotSame(t, markTask.Data, copiedData)
		assert.NotSame(t, markTask.Data.(*MarkDownloadedTaskData).Timestamp, copiedData.Timestamp)
		*copiedData.Timestamp = copiedData.Timestamp.Add(2 * time.Hour)

		original, _ := tm.GetTask(markTask.ID)
		originalData := original.Data.(*MarkDownloadedTaskData)
		assert.WithinDuration(t, now, *originalData.Timestamp, 0)
	}
}

func TestTaskManager_UpdateTaskStatus(t *testing.T) {
	tm := NewTaskManager(nil)
	task := tm.CreateTask(TaskTypeUserDownload, nil)

	tests := []struct {
		name        string
		status      TaskStatus
		wantStarted bool
		wantEnded   bool
		expectedOk  bool
	}{
		{
			name:        "设置为运行中",
			status:      TaskStatusRunning,
			wantStarted: true,
			wantEnded:   false,
			expectedOk:  true,
		},
		{
			name:        "设置为已完成",
			status:      TaskStatusCompleted,
			wantStarted: false,
			wantEnded:   true,
			expectedOk:  true,
		},
		{
			name:        "设置为失败",
			status:      TaskStatusFailed,
			wantStarted: false,
			wantEnded:   true,
			expectedOk:  true,
		},
		{
			name:        "设置为已取消",
			status:      TaskStatusCancelled,
			wantStarted: false,
			wantEnded:   true,
			expectedOk:  true,
		},
		{
			name:        "设置为队列中",
			status:      TaskStatusQueued,
			wantStarted: false,
			wantEnded:   false,
			expectedOk:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task = tm.CreateTask(TaskTypeUserDownload, nil)

			ok := tm.UpdateTaskStatus(task.ID, tt.status)
			assert.Equal(t, tt.expectedOk, ok)

			got, _ := tm.GetTask(task.ID)
			assert.Equal(t, tt.status, got.Status)

			if tt.wantStarted {
				assert.NotNil(t, got.StartedAt)
			} else {
				assert.Nil(t, got.StartedAt)
			}
			if tt.wantEnded {
				assert.NotNil(t, got.EndedAt)
			} else {
				assert.Nil(t, got.EndedAt)
			}
		})
	}
}

func TestTaskManager_UpdateTaskStatus_NotFound(t *testing.T) {
	tm := NewTaskManager(nil)

	ok := tm.UpdateTaskStatus("non_existent", TaskStatusRunning)
	assert.False(t, ok)
}

func TestTaskManager_UpdateTaskStatus_RejectsInvalidTransition(t *testing.T) {
	tm := NewTaskManager(nil)
	task := tm.CreateTask(TaskTypeUserDownload, nil)

	assert.True(t, tm.UpdateTaskStatus(task.ID, TaskStatusCompleted))
	assert.False(t, tm.UpdateTaskStatus(task.ID, TaskStatusRunning))

	got, ok := tm.GetTask(task.ID)
	assert.True(t, ok)
	assert.Equal(t, TaskStatusCompleted, got.Status)
}

func TestTaskManager_SetTaskError(t *testing.T) {
	tm := NewTaskManager(nil)
	task := tm.CreateTask(TaskTypeUserDownload, nil)
	tm.UpdateTaskProgress(task.ID, &TaskProgress{Stage: "downloading", Total: 100, Completed: 40, Current: "user1"})

	err := assert.AnError
	ok := tm.SetTaskError(task.ID, err)
	assert.True(t, ok)

	got, _ := tm.GetTask(task.ID)
	assert.Equal(t, TaskStatusFailed, got.Status)
	assert.Equal(t, err.Error(), got.Error)
	assert.NotNil(t, got.EndedAt)
	assert.Equal(t, "", got.Progress.Stage)
	assert.Equal(t, "", got.Progress.Current)
	assert.Equal(t, 40, got.Progress.Completed)

	ok = tm.SetTaskError("non_existent", err)
	assert.False(t, ok)
}

func TestTaskManager_CompleteTask_DoesNotOverrideFailedTask(t *testing.T) {
	tm := NewTaskManager(nil)
	task := tm.CreateTask(TaskTypeUserDownload, nil)

	assert.True(t, tm.SetTaskError(task.ID, assert.AnError))

	result := &TaskResult{
		Main: &TaskMainResult{
			Downloaded: 100,
		},
		Message: "should not override failed task",
	}
	assert.False(t, tm.CompleteTask(task.ID, result))

	got, ok := tm.GetTask(task.ID)
	assert.True(t, ok)
	assert.Equal(t, TaskStatusFailed, got.Status)
	assert.Equal(t, assert.AnError.Error(), got.Error)
	assert.Nil(t, got.Result)
}

func TestTaskManager_SetTaskError_DoesNotOverrideCompletedTask(t *testing.T) {
	tm := NewTaskManager(nil)
	task := tm.CreateTask(TaskTypeUserDownload, nil)

	result := &TaskResult{
		Main: &TaskMainResult{
			Downloaded: 100,
		},
		Message: "completed",
	}
	assert.True(t, tm.CompleteTask(task.ID, result))
	assert.False(t, tm.SetTaskError(task.ID, assert.AnError))

	got, ok := tm.GetTask(task.ID)
	assert.True(t, ok)
	assert.Equal(t, TaskStatusCompleted, got.Status)
	assert.NotNil(t, got.Result)
	assert.NotNil(t, got.Result.Main)
	assert.Equal(t, 100, got.Result.Main.Downloaded)
	assert.Empty(t, got.Error)
}

func TestTaskManager_CompleteTask_DoesNotOverrideCompletedTask(t *testing.T) {
	tm := NewTaskManager(nil)
	task := tm.CreateTask(TaskTypeUserDownload, nil)

	first := &TaskResult{
		Main: &TaskMainResult{
			Downloaded: 100,
			Failed:     1,
		},
		Profile: &TaskProfileResult{
			Downloaded: 7,
			Failed:     1,
			Versioned:  2,
		},
		Message: "detailed result",
	}
	second := &TaskResult{
		Message: "summary only",
	}

	assert.True(t, tm.CompleteTask(task.ID, first))
	assert.False(t, tm.CompleteTask(task.ID, second))

	got, ok := tm.GetTask(task.ID)
	assert.True(t, ok)
	assert.Equal(t, TaskStatusCompleted, got.Status)
	assert.NotNil(t, got.Result)
	assert.NotNil(t, got.Result.Main)
	assert.NotNil(t, got.Result.Profile)
	assert.Equal(t, 100, got.Result.Main.Downloaded)
	assert.Equal(t, 1, got.Result.Main.Failed)
	assert.Equal(t, 2, got.Result.Profile.Versioned)
	assert.Equal(t, "detailed result", got.Result.Message)
}

func TestTaskManager_CompleteTask_ConvergesProgress(t *testing.T) {
	tm := NewTaskManager(nil)
	task := tm.CreateTask(TaskTypeUserDownload, nil)
	tm.UpdateTaskProgress(task.ID, &TaskProgress{
		Stage:     "retrying",
		Total:     100,
		Completed: 80,
		Failed:    3,
		Current:   "user1",
	})

	result := &TaskResult{
		Main: &TaskMainResult{
			Downloaded: 97,
			Failed:     3,
		},
		Message: "done",
	}
	assert.True(t, tm.CompleteTask(task.ID, result))

	got, ok := tm.GetTask(task.ID)
	assert.True(t, ok)
	assert.Equal(t, TaskStatusCompleted, got.Status)
	assert.Equal(t, "completed", got.Progress.Stage)
	assert.Equal(t, "", got.Progress.Current)
	assert.Equal(t, 100, got.Progress.Completed)
	assert.Equal(t, 3, got.Progress.Failed)
}

func TestTaskManager_UpdateTaskProgress(t *testing.T) {
	tm := NewTaskManager(nil)
	task := tm.CreateTask(TaskTypeUserDownload, nil)

	progress := &TaskProgress{
		Total:     100,
		Completed: 50,
		Failed:    5,
	}

	ok := tm.UpdateTaskProgress(task.ID, progress)
	assert.True(t, ok)

	got, _ := tm.GetTask(task.ID)
	assert.Equal(t, progress, got.Progress)
	assert.Equal(t, 100, got.Progress.Total)
	assert.Equal(t, 50, got.Progress.Completed)
	assert.Equal(t, 5, got.Progress.Failed)

	ok = tm.UpdateTaskProgress("non_existent", progress)
	assert.False(t, ok)

	assert.True(t, tm.UpdateTaskStatus(task.ID, TaskStatusCompleted))
	ok = tm.UpdateTaskProgress(task.ID, &TaskProgress{Total: 200, Completed: 200})
	assert.False(t, ok)
}

func TestTaskManager_SetTaskResult(t *testing.T) {
	tm := NewTaskManager(nil)
	task := tm.CreateTask(TaskTypeUserDownload, nil)

	result := &TaskResult{
		Main: &TaskMainResult{
			Downloaded: 100,
			Failed:     5,
		},
		Profile: &TaskProfileResult{
			Downloaded: 6,
			Failed:     1,
			Versioned:  10,
		},
		Message: "Download completed",
	}

	ok := tm.SetTaskResult(task.ID, result)
	assert.True(t, ok)

	got, _ := tm.GetTask(task.ID)
	assert.Equal(t, result, got.Result)
	assert.NotNil(t, got.Result.Main)
	assert.NotNil(t, got.Result.Profile)
	assert.Equal(t, 100, got.Result.Main.Downloaded)
	assert.Equal(t, 5, got.Result.Main.Failed)
	assert.Equal(t, 10, got.Result.Profile.Versioned)
	assert.Equal(t, "Download completed", got.Result.Message)

	ok = tm.SetTaskResult("non_existent", result)
	assert.False(t, ok)
}

func TestTaskManager_CancelTask(t *testing.T) {
	tm := NewTaskManager(nil)

	t.Run("取消队列中的任务", func(t *testing.T) {
		task := tm.CreateTask(TaskTypeUserDownload, nil)
		assert.Equal(t, TaskStatusQueued, task.Status)

		ok := tm.CancelTask(task.ID)
		assert.True(t, ok)

		got, _ := tm.GetTask(task.ID)
		assert.Equal(t, TaskStatusCancelled, got.Status)
		assert.NotNil(t, got.EndedAt)

		select {
		case <-got.Ctx.Done():
		case <-time.After(time.Second):
			t.Error("Context should be cancelled")
		}
	})

	t.Run("取消运行中的任务", func(t *testing.T) {
		task := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.UpdateTaskStatus(task.ID, TaskStatusRunning)
		tm.UpdateTaskProgress(task.ID, &TaskProgress{Stage: "downloading", Total: 10, Completed: 3, Current: "user1"})

		ok := tm.CancelTask(task.ID)
		assert.True(t, ok)

		got, _ := tm.GetTask(task.ID)
		assert.Equal(t, TaskStatusCancelled, got.Status)
		assert.Equal(t, "", got.Progress.Stage)
		assert.Equal(t, "", got.Progress.Current)
		assert.Equal(t, 3, got.Progress.Completed)
	})

	t.Run("取消已完成的任务", func(t *testing.T) {
		task := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.UpdateTaskStatus(task.ID, TaskStatusCompleted)

		ok := tm.CancelTask(task.ID)
		assert.False(t, ok)

		got, _ := tm.GetTask(task.ID)
		assert.Equal(t, TaskStatusCompleted, got.Status)
	})

	t.Run("取消已失败的任务", func(t *testing.T) {
		task := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.SetTaskError(task.ID, assert.AnError)

		ok := tm.CancelTask(task.ID)
		assert.False(t, ok)
	})

	t.Run("取消不存在的任务", func(t *testing.T) {
		ok := tm.CancelTask("non_existent")
		assert.False(t, ok)
	})

	t.Run("取消已取消的任务", func(t *testing.T) {
		task := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.CancelTask(task.ID)

		ok := tm.CancelTask(task.ID)
		assert.False(t, ok)
	})
}

func TestTaskManager_ConcurrentAccess(t *testing.T) {
	tm := NewTaskManager(nil)
	var wg sync.WaitGroup
	numGoroutines := 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			defer wg.Done()
			tm.CreateTask(TaskTypeUserDownload, map[string]int{"index": i})
		}(i)
	}
	wg.Wait()

	tasks := tm.GetAllTasks()
	assert.Len(t, tasks, numGoroutines)

	wg.Add(numGoroutines)
	for _, task := range tasks {
		go func(taskID string) {
			defer wg.Done()
			tm.UpdateTaskStatus(taskID, TaskStatusRunning)
			tm.UpdateTaskProgress(taskID, &TaskProgress{Total: 100, Completed: 50})
		}(task.ID)
	}
	wg.Wait()

	for _, task := range tasks {
		got, ok := tm.GetTask(task.ID)
		assert.True(t, ok)
		assert.Equal(t, TaskStatusRunning, got.Status)
		assert.Equal(t, 50, got.Progress.Completed)
	}
}

func TestGenerateTaskID(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := generateTaskID()
		assert.True(t, strings.HasPrefix(id, "task_"))
		assert.Greater(t, len(id), len("task_"))
		assert.False(t, ids[id], "Task ID should be unique")
		ids[id] = true
	}
}

func TestTask_ContextCancellation(t *testing.T) {
	tm := NewTaskManager(nil)
	task := tm.CreateTask(TaskTypeUserDownload, nil)

	assert.NoError(t, task.Ctx.Err())

	tm.CancelTask(task.ID)

	assert.Error(t, task.Ctx.Err())
	assert.Equal(t, context.Canceled, task.Ctx.Err())
}

func TestTaskManager_TaskLifecycle(t *testing.T) {
	tm := NewTaskManager(nil)

	task := tm.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{ScreenName: "testuser"})
	assert.Equal(t, TaskStatusQueued, task.Status)

	tm.UpdateTaskStatus(task.ID, TaskStatusRunning)
	task, _ = tm.GetTask(task.ID)
	assert.Equal(t, TaskStatusRunning, task.Status)
	assert.NotNil(t, task.StartedAt)

	tm.UpdateTaskProgress(task.ID, &TaskProgress{Total: 100, Completed: 30, Failed: 2})
	task, _ = tm.GetTask(task.ID)
	assert.Equal(t, 30, task.Progress.Completed)

	tm.UpdateTaskProgress(task.ID, &TaskProgress{Total: 100, Completed: 100, Failed: 2})
	task, _ = tm.GetTask(task.ID)
	assert.Equal(t, 100, task.Progress.Completed)

	tm.SetTaskResult(task.ID, &TaskResult{
		Main: &TaskMainResult{
			Downloaded: 98,
			Failed:     2,
		},
		Message: "Completed",
	})
	tm.UpdateTaskStatus(task.ID, TaskStatusCompleted)
	task, _ = tm.GetTask(task.ID)
	assert.Equal(t, TaskStatusCompleted, task.Status)
	assert.NotNil(t, task.EndedAt)
	assert.NotNil(t, task.Result.Main)
	assert.Equal(t, 98, task.Result.Main.Downloaded)
	assert.Equal(t, "Completed", task.Result.Message)
	assert.Equal(t, "completed", task.Progress.Stage)
	assert.Equal(t, "", task.Progress.Current)
	assert.Equal(t, 100, task.Progress.Completed)
}

func TestTaskStatus_Constants(t *testing.T) {
	assert.Equal(t, TaskStatus("queued"), TaskStatusQueued)
	assert.Equal(t, TaskStatus("running"), TaskStatusRunning)
	assert.Equal(t, TaskStatus("completed"), TaskStatusCompleted)
	assert.Equal(t, TaskStatus("failed"), TaskStatusFailed)
	assert.Equal(t, TaskStatus("cancelled"), TaskStatusCancelled)
}

func TestTaskType_Constants(t *testing.T) {
	assert.Equal(t, TaskType("user_download"), TaskTypeUserDownload)
	assert.Equal(t, TaskType("list_download"), TaskTypeListDownload)
	assert.Equal(t, TaskType("following_download"), TaskTypeFollowingDownload)
	assert.Equal(t, TaskType("profile_download"), TaskTypeProfileDownload)
	assert.Equal(t, TaskType("mark_downloaded"), TaskTypeMarkDownloaded)
	assert.Equal(t, TaskType("json_file_download"), TaskTypeJsonFileDownload)
	assert.Equal(t, TaskType("json_folder_download"), TaskTypeJsonFolderDownload)
	assert.Equal(t, TaskType("batch_download"), TaskTypeBatchDownload)
	assert.Equal(t, TaskType("list_profile"), TaskTypeListProfile)
}

func TestTaskManager_Cleanup(t *testing.T) {
	tm := NewTaskManager(nil)
	assert.NotNil(t, tm)

	for i := 0; i < 5; i++ {
		task := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.UpdateTaskStatus(task.ID, TaskStatusCompleted)
	}

	tasks := tm.GetAllTasks()
	assert.Len(t, tasks, 5)
}

func TestTaskManager_Close(t *testing.T) {
	tm := NewTaskManager(nil)

	done := make(chan struct{})
	go func() {
		tm.Close()
		tm.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("TaskManager.Close should not block")
	}
}
