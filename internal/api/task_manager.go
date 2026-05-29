package api

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TaskManager 任务管理器
type TaskManager struct {
	tasks         map[string]*Task
	tasksSnapshot []*Task
	mu            sync.RWMutex
	stopCh        chan struct{}
	closeOnce     sync.Once
	eventBus      *EventBus
	idGenerator   func() string
}

// NewTaskManager 创建任务管理器
func NewTaskManager(eventBus *EventBus) *TaskManager {
	tm := &TaskManager{
		tasks:       make(map[string]*Task),
		stopCh:      make(chan struct{}),
		eventBus:    eventBus,
		idGenerator: generateTaskID,
	}
	go tm.cleanupLoop()
	return tm
}

func (tm *TaskManager) publishTasks() {
	if tm.eventBus == nil {
		return
	}
	tasks := tm.snapshotForPublish()
	tm.eventBus.PublishTasks(tasks)
}

// CreateTask 创建任务
func (tm *TaskManager) CreateTask(taskType TaskType, data interface{}) *Task {
	tm.mu.Lock()

	ctx, cancel := context.WithCancel(context.Background())
	task := &Task{
		ID:        tm.nextTaskIDLocked(),
		Type:      taskType,
		Status:    TaskStatusQueued,
		Data:      cloneTaskData(data),
		Progress:  &TaskProgress{},
		CreatedAt: time.Now(),
		Ctx:       ctx,
		Cancel:    cancel,
	}

	tm.tasks[task.ID] = task
	tm.rebuildSnapshotLocked()
	tm.mu.Unlock()

	tm.publishTasks()
	return task
}

func (tm *TaskManager) nextTaskIDLocked() string {
	for {
		id := tm.idGenerator()
		if _, exists := tm.tasks[id]; !exists {
			return id
		}
	}
}

// GetTask 获取任务
func (tm *TaskManager) GetTask(id string) (*Task, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, ok := tm.tasks[id]
	if !ok {
		return nil, false
	}

	return cloneTask(task), true
}

// GetAllTasks 返回缓存的只读快照浅拷贝，避免每次调用全量深拷贝
func (tm *TaskManager) GetAllTasks() []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make([]*Task, len(tm.tasksSnapshot))
	copy(result, tm.tasksSnapshot)
	return result
}

// snapshotForPublish 返回当前只读快照本身，供内部 SSE 广播复用，避免重复复制。
// tasksSnapshot 中的 Task 节点在 rebuildSnapshotLocked 时整体替换，旧快照随后保持只读。
func (tm *TaskManager) snapshotForPublish() []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return tm.tasksSnapshot
}

func (tm *TaskManager) rebuildSnapshotLocked() {
	tm.tasksSnapshot = make([]*Task, 0, len(tm.tasks))
	for _, task := range tm.tasks {
		tm.tasksSnapshot = append(tm.tasksSnapshot, cloneTask(task))
	}
	sort.Slice(tm.tasksSnapshot, func(i, j int) bool {
		return tm.tasksSnapshot[i].CreatedAt.After(tm.tasksSnapshot[j].CreatedAt)
	})
}

func cloneTask(task *Task) *Task {
	if task == nil {
		return nil
	}

	t := *task
	t.Data = cloneTaskData(task.Data)

	if task.Progress != nil {
		p := *task.Progress
		t.Progress = &p
	}
	if task.Result != nil {
		t.Result = cloneTaskResult(task.Result)
	}
	if task.StartedAt != nil {
		startedAt := *task.StartedAt
		t.StartedAt = &startedAt
	}
	if task.EndedAt != nil {
		endedAt := *task.EndedAt
		t.EndedAt = &endedAt
	}

	return &t
}

func cloneTaskResult(result *TaskResult) *TaskResult {
	if result == nil {
		return nil
	}

	clone := *result
	if result.Main != nil {
		main := *result.Main
		clone.Main = &main
	}
	if result.Profile != nil {
		profile := *result.Profile
		clone.Profile = &profile
	}
	return &clone
}

func cloneTaskData(data interface{}) interface{} {
	switch v := data.(type) {
	case nil:
		return nil
	case *UserDownloadTaskData:
		if v == nil {
			return (*UserDownloadTaskData)(nil)
		}
		copied := *v
		return &copied
	case *ListDownloadTaskData:
		if v == nil {
			return (*ListDownloadTaskData)(nil)
		}
		copied := *v
		return &copied
	case *FollowingDownloadTaskData:
		if v == nil {
			return (*FollowingDownloadTaskData)(nil)
		}
		copied := *v
		return &copied
	case *ProfileDownloadTaskData:
		if v == nil {
			return (*ProfileDownloadTaskData)(nil)
		}
		copied := *v
		return &copied
	case *MarkDownloadedTaskData:
		if v == nil {
			return (*MarkDownloadedTaskData)(nil)
		}
		copied := *v
		copied.Timestamp = cloneTimePtr(v.Timestamp)
		return &copied
	case *FollowingMarkDownloadedTaskData:
		if v == nil {
			return (*FollowingMarkDownloadedTaskData)(nil)
		}
		copied := *v
		copied.Timestamp = cloneTimePtr(v.Timestamp)
		return &copied
	case *ListMarkDownloadedTaskData:
		if v == nil {
			return (*ListMarkDownloadedTaskData)(nil)
		}
		copied := *v
		copied.Timestamp = cloneTimePtr(v.Timestamp)
		return &copied
	case *JsonFileDownloadTaskData:
		if v == nil {
			return (*JsonFileDownloadTaskData)(nil)
		}
		copied := *v
		copied.Paths = append([]string(nil), v.Paths...)
		return &copied
	case *JsonFolderDownloadTaskData:
		if v == nil {
			return (*JsonFolderDownloadTaskData)(nil)
		}
		copied := *v
		copied.Paths = append([]string(nil), v.Paths...)
		return &copied
	case *BatchDownloadTaskData:
		if v == nil {
			return (*BatchDownloadTaskData)(nil)
		}
		copied := *v
		copied.Users = append([]string(nil), v.Users...)
		copied.Lists = append([]StringUint64(nil), v.Lists...)
		copied.FollowingNames = append([]string(nil), v.FollowingNames...)
		return &copied
	case *ListProfileTaskData:
		if v == nil {
			return (*ListProfileTaskData)(nil)
		}
		copied := *v
		return &copied
	default:
		// 未知类型暂时回退为浅拷贝；调用方需要保证这类 Data 在任务创建后只读。
		return data
	}
}

func cloneTimePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}

	copied := *t
	return &copied
}

func isTerminalStatus(status TaskStatus) bool {
	return status == TaskStatusCompleted || status == TaskStatusFailed || status == TaskStatusCancelled
}

func canTransitionStatus(from, to TaskStatus) bool {
	if from == to {
		return !isTerminalStatus(from)
	}

	if isTerminalStatus(from) {
		return false
	}

	switch from {
	case TaskStatusQueued:
		return to == TaskStatusRunning || to == TaskStatusCompleted || to == TaskStatusFailed || to == TaskStatusCancelled
	case TaskStatusRunning:
		return to == TaskStatusCompleted || to == TaskStatusFailed || to == TaskStatusCancelled
	default:
		return false
	}
}

func applyTerminalProgress(task *Task, status TaskStatus, _ *TaskResult) {
	if task.Progress == nil {
		task.Progress = &TaskProgress{}
	}

	switch status {
	case TaskStatusCompleted:
		task.Progress.Stage = "completed"
		task.Progress.Current = ""
		if task.Progress.Total > 0 {
			task.Progress.Completed = task.Progress.Total
		}
	case TaskStatusFailed, TaskStatusCancelled:
		task.Progress.Stage = ""
		task.Progress.Current = ""
	}
}

// UpdateTaskStatus 更新任务状态
func (tm *TaskManager) UpdateTaskStatus(id string, status TaskStatus) bool {
	tm.mu.Lock()
	task, ok := tm.tasks[id]
	if !ok {
		tm.mu.Unlock()
		return false
	}

	if !canTransitionStatus(task.Status, status) {
		tm.mu.Unlock()
		return false
	}

	previousStatus := task.Status
	task.Status = status
	now := time.Now()

	switch status {
	case TaskStatusRunning:
		task.StartedAt = &now
	case TaskStatusCompleted, TaskStatusFailed, TaskStatusCancelled:
		applyTerminalProgress(task, status, task.Result)
		task.EndedAt = &now
		if task.Cancel != nil {
			task.Cancel()
		}
	}
	tm.rebuildSnapshotLocked()
	tm.mu.Unlock()

	tm.publishTasks()

	if tm.eventBus != nil && !isTerminalStatus(previousStatus) {
		switch status {
		case TaskStatusCompleted:
			tm.eventBus.PublishNotification("task_completed", taskTypeName(task.Type)+" completed", map[string]string{"task_id": id})
		case TaskStatusFailed:
			tm.eventBus.PublishNotification("task_failed", taskTypeName(task.Type)+" failed", map[string]string{"task_id": id})
		}
	}

	return true
}

// SetTaskError 设置任务错误
func (tm *TaskManager) SetTaskError(id string, err error) bool {
	tm.mu.Lock()
	task, ok := tm.tasks[id]
	if !ok {
		tm.mu.Unlock()
		return false
	}

	if isTerminalStatus(task.Status) {
		tm.mu.Unlock()
		return false
	}

	task.Status = TaskStatusFailed
	task.Error = err.Error()
	applyTerminalProgress(task, TaskStatusFailed, nil)
	now := time.Now()
	task.EndedAt = &now
	if task.Cancel != nil {
		task.Cancel()
	}
	tm.rebuildSnapshotLocked()
	tm.mu.Unlock()

	tm.publishTasks()
	if tm.eventBus != nil {
		tm.eventBus.PublishNotification("task_failed", taskTypeName(task.Type)+" failed: "+err.Error(), map[string]string{"task_id": id})
	}

	return true
}

// UpdateTaskProgress 更新任务进度
func (tm *TaskManager) UpdateTaskProgress(id string, progress *TaskProgress) bool {
	tm.mu.Lock()
	task, ok := tm.tasks[id]
	if !ok {
		tm.mu.Unlock()
		return false
	}
	if isTerminalStatus(task.Status) {
		tm.mu.Unlock()
		return false
	}

	task.Progress = progress
	tm.rebuildSnapshotLocked()
	tm.mu.Unlock()

	tm.publishTasks()
	return true
}

// CompleteTask 自动完成任务并设置结果，避免 SSE 竞态条件
func (tm *TaskManager) CompleteTask(id string, result *TaskResult) bool {
	tm.mu.Lock()
	task, ok := tm.tasks[id]
	if !ok {
		tm.mu.Unlock()
		return false
	}

	if !canTransitionStatus(task.Status, TaskStatusCompleted) {
		tm.mu.Unlock()
		return false
	}

	task.Result = result
	task.Status = TaskStatusCompleted
	applyTerminalProgress(task, TaskStatusCompleted, result)
	now := time.Now()
	task.EndedAt = &now
	if task.Cancel != nil {
		task.Cancel()
	}
	tm.rebuildSnapshotLocked()
	tm.mu.Unlock()

	tm.publishTasks()
	if tm.eventBus != nil {
		tm.eventBus.PublishNotification("task_completed", taskTypeName(task.Type)+" completed", map[string]string{"task_id": id})
	}

	return true
}

// CancelTask 取消任务
func (tm *TaskManager) CancelTask(id string) CancelTaskResult {
	tm.mu.Lock()
	task, ok := tm.tasks[id]
	if !ok {
		tm.mu.Unlock()
		return CancelTaskResultNotFound
	}

	if task.Status != TaskStatusQueued && task.Status != TaskStatusRunning {
		tm.mu.Unlock()
		return CancelTaskResultNotCancellable
	}

	task.Status = TaskStatusCancelled
	applyTerminalProgress(task, TaskStatusCancelled, nil)
	task.Cancel()
	now := time.Now()
	task.EndedAt = &now
	tm.rebuildSnapshotLocked()
	tm.mu.Unlock()

	tm.publishTasks()
	if tm.eventBus != nil {
		tm.eventBus.PublishNotification("task_cancelled", taskTypeName(task.Type)+" cancelled", map[string]string{"task_id": id})
	}
	return CancelTaskResultCancelled
}

// CancelAllTasks 取消所有正在运行或排队中的任务
func (tm *TaskManager) CancelAllTasks() {
	tm.mu.Lock()
	now := time.Now()
	for _, task := range tm.tasks {
		if task.Status == TaskStatusQueued || task.Status == TaskStatusRunning {
			task.Status = TaskStatusCancelled
			applyTerminalProgress(task, TaskStatusCancelled, nil)
			task.Cancel()
			task.EndedAt = &now
		}
	}
	tm.rebuildSnapshotLocked()
	tm.mu.Unlock()

	tm.publishTasks()
}

// Close 停止后台清理 goroutine
func (tm *TaskManager) Close() {
	tm.closeOnce.Do(func() {
		close(tm.stopCh)
	})
}

// cleanupLoop 定期清理过期任务
func (tm *TaskManager) cleanupLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tm.cleanup()
		case <-tm.stopCh:
			return
		}
	}
}

// cleanup 清理 8 小时前的已完成任务
func (tm *TaskManager) cleanup() {
	tm.mu.Lock()
	cutoff := time.Now().Add(-8 * time.Hour)
	for id, task := range tm.tasks {
		if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed || task.Status == TaskStatusCancelled {
			if task.EndedAt != nil && task.EndedAt.Before(cutoff) {
				delete(tm.tasks, id)
			}
		}
	}
	tm.rebuildSnapshotLocked()
	tm.mu.Unlock()

	tm.publishTasks()
}

func generateTaskID() string {
	return "task_" + uuid.NewString()
}
