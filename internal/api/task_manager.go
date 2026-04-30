package api

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusQueued    TaskStatus = "queued"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypeUserDownload       TaskType = "user_download"
	TaskTypeListDownload       TaskType = "list_download"
	TaskTypeFollowingDownload  TaskType = "following_download"
	TaskTypeProfileDownload    TaskType = "profile_download"
	TaskTypeMarkDownloaded     TaskType = "mark_downloaded"
	TaskTypeJsonFileDownload   TaskType = "json_file_download"
	TaskTypeJsonFolderDownload TaskType = "json_folder_download"
	TaskTypeBatchDownload      TaskType = "batch_download"
	TaskTypeListProfile        TaskType = "list_profile"
)

// Task 任务
type Task struct {
	ID        string             `json:"task_id"`
	Type      TaskType           `json:"type"`
	Status    TaskStatus         `json:"status"`
	Data      interface{}        `json:"data"`
	Progress  *TaskProgress      `json:"progress,omitempty"`
	Result    *TaskResult        `json:"result,omitempty"`
	Error     string             `json:"error,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
	StartedAt *time.Time         `json:"started_at,omitempty"`
	EndedAt   *time.Time         `json:"ended_at,omitempty"`
	Ctx       context.Context    `json:"-"`
	Cancel    context.CancelFunc `json:"-"`
}

// TaskProgress 任务进度
type TaskProgress struct {
	Stage     string `json:"stage"` // "syncing", "downloading", "retrying", "profile", "marking", "completed"
	Total     int    `json:"total"`
	Completed int    `json:"completed"`
	Failed    int    `json:"failed"`
	Current   string `json:"current"` // 当前处理的用户/列表
}

// TaskResult 任务结果
type TaskResult struct {
	Downloaded int    `json:"downloaded,omitempty"`
	Failed     int    `json:"failed,omitempty"`
	Versioned  int    `json:"versioned,omitempty"` // 版本化（旧文件已备份到 .versions）
	Message    string `json:"message,omitempty"`
}

// TaskManager 任务管理器
type TaskManager struct {
	tasks map[string]*Task
	mu    sync.RWMutex
}

// NewTaskManager 创建任务管理器
func NewTaskManager() *TaskManager {
	tm := &TaskManager{
		tasks: make(map[string]*Task),
	}
	// 启动清理 goroutine
	go tm.cleanupLoop()
	return tm
}

// CreateTask 创建任务
func (tm *TaskManager) CreateTask(taskType TaskType, data interface{}) *Task {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	task := &Task{
		ID:        generateTaskID(),
		Type:      taskType,
		Status:    TaskStatusQueued,
		Data:      data,
		Progress:  &TaskProgress{},
		CreatedAt: time.Now(),
		Ctx:       ctx,
		Cancel:    cancel,
	}

	tm.tasks[task.ID] = task
	return task
}

// GetTask 获取任务
func (tm *TaskManager) GetTask(id string) (*Task, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, ok := tm.tasks[id]
	return task, ok
}

// GetAllTasks 获取所有任务的深拷贝，避免并发序列化时的数据竞争
func (tm *TaskManager) GetAllTasks() []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tasks := make([]*Task, 0, len(tm.tasks))
	for _, task := range tm.tasks {
		// 浅拷贝 Task 本身
		t := *task
		
		// 深拷贝嵌套的指针对象
		if task.Progress != nil {
			p := *task.Progress
			t.Progress = &p
		}
		if task.Result != nil {
			r := *task.Result
			t.Result = &r
		}
		if task.StartedAt != nil {
			startedAt := *task.StartedAt
			t.StartedAt = &startedAt
		}
		if task.EndedAt != nil {
			endedAt := *task.EndedAt
			t.EndedAt = &endedAt
		}
		
		tasks = append(tasks, &t)
	}
	return tasks
}

// UpdateTaskStatus 更新任务状态
func (tm *TaskManager) UpdateTaskStatus(id string, status TaskStatus) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, ok := tm.tasks[id]
	if !ok {
		return false
	}

	task.Status = status
	now := time.Now()

	switch status {
	case TaskStatusRunning:
		task.StartedAt = &now
	case TaskStatusCompleted, TaskStatusFailed, TaskStatusCancelled:
		task.EndedAt = &now
	}

	return true
}

// SetTaskError 设置任务错误
func (tm *TaskManager) SetTaskError(id string, err error) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, ok := tm.tasks[id]
	if !ok {
		return false
	}

	if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed || task.Status == TaskStatusCancelled {
		return false
	}

	task.Status = TaskStatusFailed
	task.Error = err.Error()
	now := time.Now()
	task.EndedAt = &now

	return true
}

// UpdateTaskProgress 更新任务进度
func (tm *TaskManager) UpdateTaskProgress(id string, progress *TaskProgress) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, ok := tm.tasks[id]
	if !ok {
		return false
	}

	task.Progress = progress
	return true
}

// SetTaskResult 设置任务结果
func (tm *TaskManager) SetTaskResult(id string, result *TaskResult) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, ok := tm.tasks[id]
	if !ok {
		return false
	}

	task.Result = result
	return true
}

// CancelTask 取消任务
func (tm *TaskManager) CancelTask(id string) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, ok := tm.tasks[id]
	if !ok {
		return false
	}

	if task.Status != TaskStatusQueued && task.Status != TaskStatusRunning {
		return false
	}

	task.Status = TaskStatusCancelled
	task.Cancel()
	now := time.Now()
	task.EndedAt = &now

	return true
}

// cleanupLoop 定期清理过期任务
func (tm *TaskManager) cleanupLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		tm.cleanup()
	}
}

// cleanup 清理 8 小时前的已完成任务
func (tm *TaskManager) cleanup() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	cutoff := time.Now().Add(-8 * time.Hour)
	for id, task := range tm.tasks {
		if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed || task.Status == TaskStatusCancelled {
			if task.EndedAt != nil && task.EndedAt.Before(cutoff) {
				delete(tm.tasks, id)
			}
		}
	}
}

func generateTaskID() string {
	return "task_" + uuid.New().String()[:8]
}
