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
	tm := NewTaskManager()
	assert.NotNil(t, tm)
	assert.NotNil(t, tm.tasks)
	assert.Empty(t, tm.tasks)
}

func TestTaskManager_CreateTask(t *testing.T) {
	tm := NewTaskManager()

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

func TestTaskManager_GetTask(t *testing.T) {
	tm := NewTaskManager()

	// 创建任务
	task := tm.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{ScreenName: "testuser"})

	// 获取存在的任务
	got, ok := tm.GetTask(task.ID)
	assert.True(t, ok)
	assert.Equal(t, task, got)

	// 获取不存在的任务
	got, ok = tm.GetTask("non_existent_task")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestTaskManager_GetAllTasks(t *testing.T) {
	tm := NewTaskManager()

	// 初始为空
	tasks := tm.GetAllTasks()
	assert.Empty(t, tasks)

	// 创建多个任务
	task1 := tm.CreateTask(TaskTypeUserDownload, nil)
	task2 := tm.CreateTask(TaskTypeListDownload, nil)
	task3 := tm.CreateTask(TaskTypeBatchDownload, nil)

	tasks = tm.GetAllTasks()
	assert.Len(t, tasks, 3)

	// 验证包含所有任务
	taskIDs := make(map[string]bool)
	for _, task := range tasks {
		taskIDs[task.ID] = true
	}
	assert.True(t, taskIDs[task1.ID])
	assert.True(t, taskIDs[task2.ID])
	assert.True(t, taskIDs[task3.ID])
}

func TestTaskManager_UpdateTaskStatus(t *testing.T) {
	tm := NewTaskManager()
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
			wantStarted: false, // 从 queued 直接到 completed，StartedAt 不会被设置
			wantEnded:   true,
			expectedOk:  true,
		},
		{
			name:        "设置为失败",
			status:      TaskStatusFailed,
			wantStarted: false, // 从 queued 直接到 failed，StartedAt 不会被设置
			wantEnded:   true,
			expectedOk:  true,
		},
		{
			name:        "设置为已取消",
			status:      TaskStatusCancelled,
			wantStarted: false, // 从 queued 直接到 cancelled，StartedAt 不会被设置
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
			// 为每个测试创建新任务
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
	tm := NewTaskManager()

	ok := tm.UpdateTaskStatus("non_existent", TaskStatusRunning)
	assert.False(t, ok)
}

func TestTaskManager_SetTaskError(t *testing.T) {
	tm := NewTaskManager()
	task := tm.CreateTask(TaskTypeUserDownload, nil)

	err := assert.AnError
	ok := tm.SetTaskError(task.ID, err)
	assert.True(t, ok)

	got, _ := tm.GetTask(task.ID)
	assert.Equal(t, TaskStatusFailed, got.Status)
	assert.Equal(t, err.Error(), got.Error)
	assert.NotNil(t, got.EndedAt)

	// 测试不存在的任务
	ok = tm.SetTaskError("non_existent", err)
	assert.False(t, ok)
}

func TestTaskManager_UpdateTaskProgress(t *testing.T) {
	tm := NewTaskManager()
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

	// 测试不存在的任务
	ok = tm.UpdateTaskProgress("non_existent", progress)
	assert.False(t, ok)
}

func TestTaskManager_SetTaskResult(t *testing.T) {
	tm := NewTaskManager()
	task := tm.CreateTask(TaskTypeUserDownload, nil)

	result := &TaskResult{
		Downloaded: 100,
		Failed:     5,
		Skipped:    10,
		Message:    "Download completed",
	}

	ok := tm.SetTaskResult(task.ID, result)
	assert.True(t, ok)

	got, _ := tm.GetTask(task.ID)
	assert.Equal(t, result, got.Result)
	assert.Equal(t, 100, got.Result.Downloaded)
	assert.Equal(t, 5, got.Result.Failed)
	assert.Equal(t, 10, got.Result.Skipped)
	assert.Equal(t, "Download completed", got.Result.Message)

	// 测试不存在的任务
	ok = tm.SetTaskResult("non_existent", result)
	assert.False(t, ok)
}

func TestTaskManager_CancelTask(t *testing.T) {
	tm := NewTaskManager()

	t.Run("取消队列中的任务", func(t *testing.T) {
		task := tm.CreateTask(TaskTypeUserDownload, nil)
		assert.Equal(t, TaskStatusQueued, task.Status)

		ok := tm.CancelTask(task.ID)
		assert.True(t, ok)

		got, _ := tm.GetTask(task.ID)
		assert.Equal(t, TaskStatusCancelled, got.Status)
		assert.NotNil(t, got.EndedAt)

		// 验证 context 被取消
		select {
		case <-got.Ctx.Done():
			// 预期行为
		case <-time.After(time.Second):
			t.Error("Context should be cancelled")
		}
	})

	t.Run("取消运行中的任务", func(t *testing.T) {
		task := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.UpdateTaskStatus(task.ID, TaskStatusRunning)

		ok := tm.CancelTask(task.ID)
		assert.True(t, ok)

		got, _ := tm.GetTask(task.ID)
		assert.Equal(t, TaskStatusCancelled, got.Status)
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
	tm := NewTaskManager()
	var wg sync.WaitGroup
	numGoroutines := 100

	// 并发创建任务
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

	// 并发更新任务状态
	wg.Add(numGoroutines)
	for _, task := range tasks {
		go func(taskID string) {
			defer wg.Done()
			tm.UpdateTaskStatus(taskID, TaskStatusRunning)
			tm.UpdateTaskProgress(taskID, &TaskProgress{Total: 100, Completed: 50})
		}(task.ID)
	}
	wg.Wait()

	// 验证所有任务都被更新
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
	tm := NewTaskManager()
	task := tm.CreateTask(TaskTypeUserDownload, nil)

	// 验证 context 初始状态
	assert.NoError(t, task.Ctx.Err())

	// 取消任务
	tm.CancelTask(task.ID)

	// 验证 context 被取消
	assert.Error(t, task.Ctx.Err())
	assert.Equal(t, context.Canceled, task.Ctx.Err())
}

func TestTaskManager_TaskLifecycle(t *testing.T) {
	tm := NewTaskManager()

	// 1. 创建任务
	task := tm.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{ScreenName: "testuser"})
	assert.Equal(t, TaskStatusQueued, task.Status)

	// 2. 开始任务
	tm.UpdateTaskStatus(task.ID, TaskStatusRunning)
	task, _ = tm.GetTask(task.ID)
	assert.Equal(t, TaskStatusRunning, task.Status)
	assert.NotNil(t, task.StartedAt)

	// 3. 更新进度
	tm.UpdateTaskProgress(task.ID, &TaskProgress{Total: 100, Completed: 30, Failed: 2})
	task, _ = tm.GetTask(task.ID)
	assert.Equal(t, 30, task.Progress.Completed)

	// 4. 更新更多进度
	tm.UpdateTaskProgress(task.ID, &TaskProgress{Total: 100, Completed: 100, Failed: 2})
	task, _ = tm.GetTask(task.ID)
	assert.Equal(t, 100, task.Progress.Completed)

	// 5. 设置结果并完成任务
	tm.SetTaskResult(task.ID, &TaskResult{Downloaded: 98, Failed: 2, Message: "Completed"})
	tm.UpdateTaskStatus(task.ID, TaskStatusCompleted)
	task, _ = tm.GetTask(task.ID)
	assert.Equal(t, TaskStatusCompleted, task.Status)
	assert.NotNil(t, task.EndedAt)
	assert.Equal(t, 98, task.Result.Downloaded)
	assert.Equal(t, "Completed", task.Result.Message)
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
	// 注意：cleanup 函数在后台运行，这里主要测试其存在性
	tm := NewTaskManager()
	assert.NotNil(t, tm)

	// 创建一些任务
	for i := 0; i < 5; i++ {
		task := tm.CreateTask(TaskTypeUserDownload, nil)
		tm.UpdateTaskStatus(task.ID, TaskStatusCompleted)
	}

	// 验证任务存在
	tasks := tm.GetAllTasks()
	assert.Len(t, tasks, 5)
}
