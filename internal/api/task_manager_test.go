package api

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
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

func TestTaskManager_CreateTask_RetriesTaskIDCollision(t *testing.T) {
	tm := NewTaskManager(nil)
	ids := []string{"task_collision", "task_collision", "task_next"}
	var calls int
	tm.idGenerator = func() string {
		id := ids[calls]
		calls++
		return id
	}

	first := tm.CreateTask(TaskTypeUserDownload, nil)
	second := tm.CreateTask(TaskTypeListDownload, nil)

	assert.Equal(t, "task_collision", first.ID)
	assert.Equal(t, "task_next", second.ID)
	assert.Equal(t, 3, calls)

	gotFirst, ok := tm.GetTask(first.ID)
	assert.True(t, ok)
	assert.Equal(t, TaskTypeUserDownload, gotFirst.Type)

	gotSecond, ok := tm.GetTask(second.ID)
	assert.True(t, ok)
	assert.Equal(t, TaskTypeListDownload, gotSecond.Type)
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
	tm.CompleteTask(task.ID, &TaskResult{
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
	assert.Equal(t, TaskStatusCompleted, again.Status)
	assert.Equal(t, 10, again.Progress.Completed)
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
	tm.mu.Lock()
	tm.tasks[task1.ID].CreatedAt = time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	tm.tasks[task2.ID].CreatedAt = time.Date(2024, 1, 1, 10, 1, 0, 0, time.UTC)
	tm.tasks[task3.ID].CreatedAt = time.Date(2024, 1, 1, 10, 2, 0, 0, time.UTC)
	tm.rebuildSnapshotLocked()
	tm.mu.Unlock()

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

func TestTaskManager_CancelTask(t *testing.T) {
	tm := NewTaskManager(nil)

	t.Run("取消队列中的任务", func(t *testing.T) {
		task := tm.CreateTask(TaskTypeUserDownload, nil)
		assert.Equal(t, TaskStatusQueued, task.Status)

		result := tm.CancelTask(task.ID)
		assert.Equal(t, CancelTaskResultCancelled, result)

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

		result := tm.CancelTask(task.ID)
		assert.Equal(t, CancelTaskResultCancelled, result)

		got, _ := tm.GetTask(task.ID)
		assert.Equal(t, TaskStatusCancelled, got.Status)
		assert.Equal(t, "", got.Progress.Stage)
		assert.Equal(t, "", got.Progress.Current)
		assert.Equal(t, 3, got.Progress.Completed)
	})

	t.Run("取消已完成的任务", func(t *testing.T) {
		task := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.UpdateTaskStatus(task.ID, TaskStatusCompleted)

		result := tm.CancelTask(task.ID)
		assert.Equal(t, CancelTaskResultNotCancellable, result)

		got, _ := tm.GetTask(task.ID)
		assert.Equal(t, TaskStatusCompleted, got.Status)
	})

	t.Run("取消已失败的任务", func(t *testing.T) {
		task := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.SetTaskError(task.ID, assert.AnError)

		result := tm.CancelTask(task.ID)
		assert.Equal(t, CancelTaskResultNotCancellable, result)
	})

	t.Run("取消不存在的任务", func(t *testing.T) {
		result := tm.CancelTask("non_existent")
		assert.Equal(t, CancelTaskResultNotFound, result)
	})

	t.Run("取消已取消的任务", func(t *testing.T) {
		task := tm.CreateTask(TaskTypeUserDownload, nil)
		assert.Equal(t, CancelTaskResultCancelled, tm.CancelTask(task.ID))

		result := tm.CancelTask(task.ID)
		assert.Equal(t, CancelTaskResultNotCancellable, result)
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
		assert.Len(t, id, len("task_")+36)
		_, err := uuid.Parse(strings.TrimPrefix(id, "task_"))
		assert.NoError(t, err)
		assert.False(t, ids[id], "Task ID should be unique")
		ids[id] = true
	}
}

func TestTask_ContextCancellation(t *testing.T) {
	tm := NewTaskManager(nil)
	task := tm.CreateTask(TaskTypeUserDownload, nil)

	assert.NoError(t, task.Ctx.Err())

	assert.Equal(t, CancelTaskResultCancelled, tm.CancelTask(task.ID))

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

	tm.CompleteTask(task.ID, &TaskResult{
		Main: &TaskMainResult{
			Downloaded: 98,
			Failed:     2,
		},
		Message: "Completed",
	})
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

func TestTaskManager_GetTaskStats(t *testing.T) {
	t.Run("空任务管理器", func(t *testing.T) {
		tm := NewTaskManager(nil)
		stats := tm.GetTaskStats()
		assert.Equal(t, 0, stats.Queued)
		assert.Equal(t, 0, stats.Running)
		assert.Equal(t, 0, stats.Completed)
		assert.Equal(t, 0, stats.Failed)
		assert.Equal(t, 0, stats.Cancelled)
		assert.Equal(t, 0, stats.Total)
	})

	t.Run("混合状态任务", func(t *testing.T) {
		tm := NewTaskManager(nil)

		// 2 个 queued（默认状态）
		queued1 := tm.CreateTask(TaskTypeUserDownload, nil)
		queued2 := tm.CreateTask(TaskTypeUserDownload, nil)

		// 1 个 running
		running := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.UpdateTaskStatus(running.ID, TaskStatusRunning)

		// 1 个 completed
		completed := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.UpdateTaskStatus(completed.ID, TaskStatusCompleted)

		// 1 个 failed
		failed := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.SetTaskError(failed.ID, assert.AnError)

		// 1 个 cancelled
		cancelled := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.CancelTask(cancelled.ID)

		// 验证 queued1 和 queued2 仍为 queued
		q1, _ := tm.GetTask(queued1.ID)
		q2, _ := tm.GetTask(queued2.ID)
		assert.Equal(t, TaskStatusQueued, q1.Status)
		assert.Equal(t, TaskStatusQueued, q2.Status)

		stats := tm.GetTaskStats()
		assert.Equal(t, 2, stats.Queued)
		assert.Equal(t, 1, stats.Running)
		assert.Equal(t, 1, stats.Completed)
		assert.Equal(t, 1, stats.Failed)
		assert.Equal(t, 1, stats.Cancelled)
		assert.Equal(t, 6, stats.Total)
	})
}

func TestTaskManager_CancelQueuedTasks(t *testing.T) {
	t.Run("取消排队任务", func(t *testing.T) {
		tm := NewTaskManager(nil)
		task1 := tm.CreateTask(TaskTypeUserDownload, nil)
		task2 := tm.CreateTask(TaskTypeListDownload, nil)
		task3 := tm.CreateTask(TaskTypeBatchDownload, nil)

		count := tm.CancelQueuedTasks()
		assert.Equal(t, 3, count)

		for _, id := range []string{task1.ID, task2.ID, task3.ID} {
			got, ok := tm.GetTask(id)
			assert.True(t, ok)
			assert.Equal(t, TaskStatusCancelled, got.Status)
			assert.NotNil(t, got.EndedAt)

			select {
			case <-got.Ctx.Done():
			case <-time.After(time.Second):
				t.Error("Context should be cancelled")
			}
		}
	})

	t.Run("不影响运行中任务", func(t *testing.T) {
		tm := NewTaskManager(nil)
		queued := tm.CreateTask(TaskTypeUserDownload, nil)
		running := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.UpdateTaskStatus(running.ID, TaskStatusRunning)

		count := tm.CancelQueuedTasks()
		assert.Equal(t, 1, count)

		gotQueued, _ := tm.GetTask(queued.ID)
		assert.Equal(t, TaskStatusCancelled, gotQueued.Status)

		gotRunning, _ := tm.GetTask(running.ID)
		assert.Equal(t, TaskStatusRunning, gotRunning.Status)
	})

	t.Run("空队列", func(t *testing.T) {
		tm := NewTaskManager(nil)
		count := tm.CancelQueuedTasks()
		assert.Equal(t, 0, count)
	})

	t.Run("不影响终态任务", func(t *testing.T) {
		tm := NewTaskManager(nil)
		completed := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.UpdateTaskStatus(completed.ID, TaskStatusCompleted)
		failed := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.SetTaskError(failed.ID, assert.AnError)

		count := tm.CancelQueuedTasks()
		assert.Equal(t, 0, count)

		gotCompleted, _ := tm.GetTask(completed.ID)
		assert.Equal(t, TaskStatusCompleted, gotCompleted.Status)

		gotFailed, _ := tm.GetTask(failed.ID)
		assert.Equal(t, TaskStatusFailed, gotFailed.Status)
	})
}

func TestTaskManager_DeleteTask(t *testing.T) {
	t.Run("删除已完成任务", func(t *testing.T) {
		tm := NewTaskManager(nil)
		task := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.UpdateTaskStatus(task.ID, TaskStatusCompleted)

		result := tm.DeleteTask(task.ID)
		assert.Equal(t, DeleteTaskResultDeleted, result)

		_, ok := tm.GetTask(task.ID)
		assert.False(t, ok)

		assert.Empty(t, tm.GetAllTasks())
	})

	t.Run("删除失败任务", func(t *testing.T) {
		tm := NewTaskManager(nil)
		task := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.SetTaskError(task.ID, assert.AnError)

		result := tm.DeleteTask(task.ID)
		assert.Equal(t, DeleteTaskResultDeleted, result)

		_, ok := tm.GetTask(task.ID)
		assert.False(t, ok)
	})

	t.Run("删除已取消任务", func(t *testing.T) {
		tm := NewTaskManager(nil)
		task := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.CancelTask(task.ID)

		result := tm.DeleteTask(task.ID)
		assert.Equal(t, DeleteTaskResultDeleted, result)

		_, ok := tm.GetTask(task.ID)
		assert.False(t, ok)
	})

	t.Run("不可删除运行中任务", func(t *testing.T) {
		tm := NewTaskManager(nil)
		task := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.UpdateTaskStatus(task.ID, TaskStatusRunning)

		result := tm.DeleteTask(task.ID)
		assert.Equal(t, DeleteTaskResultNotDeletable, result)

		got, ok := tm.GetTask(task.ID)
		assert.True(t, ok)
		assert.Equal(t, TaskStatusRunning, got.Status)
	})

	t.Run("不可删除排队中任务", func(t *testing.T) {
		tm := NewTaskManager(nil)
		task := tm.CreateTask(TaskTypeUserDownload, nil)

		result := tm.DeleteTask(task.ID)
		assert.Equal(t, DeleteTaskResultNotDeletable, result)

		got, ok := tm.GetTask(task.ID)
		assert.True(t, ok)
		assert.Equal(t, TaskStatusQueued, got.Status)
	})

	t.Run("不存在的任务", func(t *testing.T) {
		tm := NewTaskManager(nil)
		result := tm.DeleteTask("non_existent")
		assert.Equal(t, DeleteTaskResultNotFound, result)
	})
}

func TestTaskManager_RetryTask(t *testing.T) {
	t.Run("重试失败任务", func(t *testing.T) {
		tm := NewTaskManager(nil)
		original := tm.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{ScreenName: "testuser"})
		tm.SetTaskError(original.ID, assert.AnError)

		newTask, result := tm.RetryTask(original.ID)
		assert.Equal(t, RetryTaskResultSuccess, result)
		assert.NotNil(t, newTask)
		assert.NotEqual(t, original.ID, newTask.ID)
		assert.Equal(t, TaskTypeUserDownload, newTask.Type)
		assert.Equal(t, TaskStatusQueued, newTask.Status)

		newData, ok := newTask.Data.(*UserDownloadTaskData)
		assert.True(t, ok)
		assert.Equal(t, "testuser", newData.ScreenName)

		// 原任务状态不变
		gotOriginal, _ := tm.GetTask(original.ID)
		assert.Equal(t, TaskStatusFailed, gotOriginal.Status)
	})

	t.Run("重试取消任务", func(t *testing.T) {
		tm := NewTaskManager(nil)
		original := tm.CreateTask(TaskTypeListDownload, &ListDownloadTaskData{ListID: 123})
		tm.CancelTask(original.ID)

		newTask, result := tm.RetryTask(original.ID)
		assert.Equal(t, RetryTaskResultSuccess, result)
		assert.NotNil(t, newTask)
		assert.Equal(t, TaskTypeListDownload, newTask.Type)

		newData, ok := newTask.Data.(*ListDownloadTaskData)
		assert.True(t, ok)
		assert.Equal(t, StringUint64(123), newData.ListID)
	})

	t.Run("不可重试运行中任务", func(t *testing.T) {
		tm := NewTaskManager(nil)
		task := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.UpdateTaskStatus(task.ID, TaskStatusRunning)

		newTask, result := tm.RetryTask(task.ID)
		assert.Equal(t, RetryTaskResultNotRetryable, result)
		assert.Nil(t, newTask)
	})

	t.Run("不可重试已完成任务", func(t *testing.T) {
		tm := NewTaskManager(nil)
		task := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.UpdateTaskStatus(task.ID, TaskStatusCompleted)

		newTask, result := tm.RetryTask(task.ID)
		assert.Equal(t, RetryTaskResultNotRetryable, result)
		assert.Nil(t, newTask)
	})

	t.Run("不存在的任务", func(t *testing.T) {
		tm := NewTaskManager(nil)
		newTask, result := tm.RetryTask("non_existent")
		assert.Equal(t, RetryTaskResultNotFound, result)
		assert.Nil(t, newTask)
	})

	t.Run("Data 被深拷贝", func(t *testing.T) {
		tm := NewTaskManager(nil)
		original := tm.CreateTask(TaskTypeBatchDownload, &BatchDownloadTaskData{
			Users: []string{"user1"},
		})
		tm.SetTaskError(original.ID, assert.AnError)

		newTask, result := tm.RetryTask(original.ID)
		assert.Equal(t, RetryTaskResultSuccess, result)

		// 修改新任务的 Data，原任务不受影响
		newData := newTask.Data.(*BatchDownloadTaskData)
		newData.Users[0] = "mutated"

		gotOriginal, _ := tm.GetTask(original.ID)
		originalData := gotOriginal.Data.(*BatchDownloadTaskData)
		assert.Equal(t, "user1", originalData.Users[0])
	})
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
