package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/unkmonster/tmd/internal/service"
)

func TestNewSSEProgressReporter(t *testing.T) {
	server := &Server{}
	reporter := NewSSEProgressReporter(server, "task_123")

	assert.NotNil(t, reporter)

	// 类型断言以访问内部字段
	sseReporter, ok := reporter.(*SSEProgressReporter)
	assert.True(t, ok)
	assert.Equal(t, server, sseReporter.server)
	assert.Equal(t, "task_123", sseReporter.taskID)
}

func TestSSEProgressReporter_OnProgress(t *testing.T) {
	tm := NewTaskManager()
	server := &Server{
		taskManager: tm,
		sseMgr:      newSSEManager(),
	}

	// 创建任务
	task := tm.CreateTask(TaskTypeUserDownload, nil)

	reporter := NewSSEProgressReporter(server, task.ID)

	progress := service.Progress{
		Stage:     "downloading",
		Total:     100,
		Completed: 50,
		Failed:    5,
		Current:   "user1",
	}

	reporter.OnProgress(task.ID, progress)

	// 验证任务进度已更新
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

	reporter := NewSSEProgressReporter(server, "non_existent_task")

	progress := service.Progress{
		Total:     100,
		Completed: 50,
	}

	// 不应该 panic
	reporter.OnProgress("non_existent_task", progress)
}

func TestSSEProgressReporter_OnComplete(t *testing.T) {
	tm := NewTaskManager()
	server := &Server{
		taskManager: tm,
		sseMgr:      newSSEManager(),
	}

	task := tm.CreateTask(TaskTypeUserDownload, nil)
	tm.UpdateTaskStatus(task.ID, TaskStatusRunning)

	reporter := NewSSEProgressReporter(server, task.ID)

	result := service.Result{
		Downloaded: 95,
		Failed:     5,
		Versioned:  10,
		Message:    "Download completed successfully",
	}

	reporter.OnComplete(task.ID, result)

	// 验证任务结果和状态
	updatedTask, ok := tm.GetTask(task.ID)
	assert.True(t, ok)
	assert.Equal(t, TaskStatusCompleted, updatedTask.Status)
	assert.NotNil(t, updatedTask.Result)
	assert.Equal(t, 95, updatedTask.Result.Downloaded)
	assert.Equal(t, 5, updatedTask.Result.Failed)
	assert.Equal(t, 10, updatedTask.Result.Versioned)
	assert.Equal(t, "Download completed successfully", updatedTask.Result.Message)
}

func TestSSEProgressReporter_OnError(t *testing.T) {
	tm := NewTaskManager()
	server := &Server{
		taskManager: tm,
		sseMgr:      newSSEManager(),
	}

	task := tm.CreateTask(TaskTypeUserDownload, nil)
	tm.UpdateTaskStatus(task.ID, TaskStatusRunning)

	reporter := NewSSEProgressReporter(server, task.ID)

	testErr := errors.New("download failed: network error")

	reporter.OnError(task.ID, testErr)

	// 验证任务错误状态
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
		sseMgr:      newSSEManager(),
	}

	task := tm.CreateTask(TaskTypeUserDownload, nil)
	reporter := NewSSEProgressReporter(server, task.ID)

	// 模拟多次进度更新
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

	// 验证最终进度
	updatedTask, _ := tm.GetTask(task.ID)
	assert.Equal(t, 100, updatedTask.Progress.Completed)
	assert.Equal(t, 100, updatedTask.Progress.Total)
}

func TestSSEManager_RegisterUnregister(t *testing.T) {
	mgr := newSSEManager()

	client := &sseClient{
		id:   "client_1",
		done: make(chan struct{}),
	}

	// 注册客户端
	mgr.register(client)
	assert.Len(t, mgr.clients, 1)

	// 重复注册同一个客户端（应该覆盖）
	mgr.register(client)
	assert.Len(t, mgr.clients, 1)

	// 注销客户端
	mgr.unregister(client.id)
	assert.Len(t, mgr.clients, 0)

	// 注销不存在的客户端（不应该 panic）
	mgr.unregister("non_existent")
}

func TestSSEManager_RegisterMultipleClients(t *testing.T) {
	mgr := newSSEManager()

	clients := []*sseClient{
		{id: "client_1", done: make(chan struct{})},
		{id: "client_2", done: make(chan struct{})},
		{id: "client_3", done: make(chan struct{})},
	}

	for _, c := range clients {
		mgr.register(c)
	}

	assert.Len(t, mgr.clients, 3)

	// 注销其中一个
	mgr.unregister("client_2")
	assert.Len(t, mgr.clients, 2)

	// 验证其他客户端仍然存在
	assert.NotNil(t, mgr.clients["client_1"])
	assert.NotNil(t, mgr.clients["client_3"])
}

func TestSSEManager_ConcurrentAccess(t *testing.T) {
	mgr := newSSEManager()

	// 并发注册
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(i int) {
			client := &sseClient{
				id:   fmt.Sprintf("client_%d", i),
				done: make(chan struct{}),
			}
			mgr.register(client)
			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	assert.Len(t, mgr.clients, 100)

	// 并发注销
	for i := 0; i < 100; i++ {
		go func(i int) {
			mgr.unregister(fmt.Sprintf("client_%d", i))
			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	assert.Len(t, mgr.clients, 0)
}

func TestSSEEvent_Marshal(t *testing.T) {
	tests := []struct {
		name  string
		event SSEEvent
	}{
		{
			name: "进度事件",
			event: SSEEvent{
				Type:   "progress",
				TaskID: "task_123",
				Progress: &TaskProgress{
					Stage:     "downloading",
					Total:     100,
					Completed: 50,
					Failed:    5,
					Current:   "user1",
				},
				Timestamp: 1234567890,
			},
		},
		{
			name: "完成事件",
			event: SSEEvent{
				Type:   "complete",
				TaskID: "task_123",
				Result: &TaskResult{
					Downloaded: 95,
					Failed:     5,
					Versioned:  10,
					Message:    "Done",
				},
				Timestamp: 1234567890,
			},
		},
		{
			name: "错误事件",
			event: SSEEvent{
				Type:      "error",
				TaskID:    "task_123",
				Error:     "something went wrong",
				Timestamp: 1234567890,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.event)
			assert.NoError(t, err)

			var decoded map[string]interface{}
			err = json.Unmarshal(data, &decoded)
			assert.NoError(t, err)
			assert.Equal(t, tt.event.Type, decoded["type"])
			assert.Equal(t, tt.event.TaskID, decoded["task_id"])
		})
	}
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
		Downloaded: 100,
		Failed:     5,
		Versioned:  10,
		Message:    "Task completed",
	}

	data, err := json.Marshal(r)
	assert.NoError(t, err)

	var decoded TaskResult
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, r, decoded)
}

func TestServer_broadcastProgress(t *testing.T) {
	server := &Server{
		sseMgr: newSSEManager(),
	}

	progress := service.Progress{
		Stage:     "downloading",
		Total:     100,
		Completed: 50,
		Failed:    5,
		Current:   "user1",
	}

	// 不应该 panic
	server.broadcastProgress("task_123", progress)
}

func TestServer_broadcastComplete(t *testing.T) {
	server := &Server{
		sseMgr: newSSEManager(),
	}

	result := service.Result{
		Downloaded: 95,
		Failed:     5,
		Versioned:  10,
		Message:    "Completed",
	}

	// 不应该 panic
	server.broadcastComplete("task_123", result)
}

func TestServer_broadcastError(t *testing.T) {
	server := &Server{
		sseMgr: newSSEManager(),
	}

	testErr := errors.New("test error")

	// 不应该 panic
	server.broadcastError("task_123", testErr)
}

func TestSSEProgressReporter_InterfaceCompliance(t *testing.T) {
	// 验证 SSEProgressReporter 实现了 ProgressReporter 接口
	var _ service.ProgressReporter = (*SSEProgressReporter)(nil)
}

func TestSSEProgressReporter_CompleteWorkflow(t *testing.T) {
	tm := NewTaskManager()
	server := &Server{
		taskManager: tm,
		sseMgr:      newSSEManager(),
	}

	// 创建任务
	task := tm.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{ScreenName: "testuser"})

	reporter := NewSSEProgressReporter(server, task.ID)

	// 1. 报告进度
	reporter.OnProgress(task.ID, service.Progress{
		Stage:     "syncing",
		Total:     100,
		Completed: 0,
		Current:   "starting",
	})

	// 2. 报告更多进度
	reporter.OnProgress(task.ID, service.Progress{
		Stage:     "downloading",
		Total:     100,
		Completed: 50,
		Current:   "halfway",
	})

	// 3. 报告完成
	reporter.OnComplete(task.ID, service.Result{
		Downloaded: 100,
		Failed:     0,
		Versioned:  0,
		Message:    "All done",
	})

	// 验证最终状态
	finalTask, _ := tm.GetTask(task.ID)
	assert.Equal(t, TaskStatusCompleted, finalTask.Status)
	assert.Equal(t, 100, finalTask.Result.Downloaded)
	assert.Equal(t, "All done", finalTask.Result.Message)
}

func TestSSEProgressReporter_ErrorWorkflow(t *testing.T) {
	tm := NewTaskManager()
	server := &Server{
		taskManager: tm,
		sseMgr:      newSSEManager(),
	}

	task := tm.CreateTask(TaskTypeUserDownload, nil)
	reporter := NewSSEProgressReporter(server, task.ID)

	// 报告一些进度
	reporter.OnProgress(task.ID, service.Progress{
		Total:     100,
		Completed: 30,
	})

	// 报告错误
	testErr := errors.New("network timeout")
	reporter.OnError(task.ID, testErr)

	// 验证错误状态
	finalTask, _ := tm.GetTask(task.ID)
	assert.Equal(t, TaskStatusFailed, finalTask.Status)
	assert.Equal(t, testErr.Error(), finalTask.Error)
	assert.Equal(t, 30, finalTask.Progress.Completed)
}
