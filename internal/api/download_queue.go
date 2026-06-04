package api

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/unkmonster/tmd/internal/service"
)

const downloadCancelGrace = 10 * time.Second

type downloadJob struct {
	task   *Task
	taskID string
	run    func(ctx context.Context, taskID string, reporter service.ProgressReporter) error
	keys   []targetKey
}

type detachedRun struct {
	taskID    string
	startedAt time.Time
	keys      []targetKey
}

type DownloadQueue struct {
	server      *Server
	cancelGrace time.Duration

	mu          sync.Mutex
	cond        *sync.Cond
	pending     []*downloadJob
	heldTargets map[targetKey]string
	detached    map[string]*detachedRun
	closed      bool

	workerDone chan struct{}
	detachedWG sync.WaitGroup
}

func NewDownloadQueue(server *Server) *DownloadQueue {
	q := &DownloadQueue{
		server:      server,
		cancelGrace: downloadCancelGrace,
		pending:     make([]*downloadJob, 0),
		heldTargets: make(map[targetKey]string),
		detached:    make(map[string]*detachedRun),
		workerDone:  make(chan struct{}),
	}
	q.cond = sync.NewCond(&q.mu)
	go q.workerLoop()
	return q
}

func (q *DownloadQueue) Enqueue(task *Task, run func(ctx context.Context, taskID string, reporter service.ProgressReporter) error) {
	if task == nil || run == nil {
		return
	}

	job := &downloadJob{
		task:   task,
		taskID: task.ID,
		run:    run,
		keys:   taskTargetKeys(task),
	}

	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.pending = append(q.pending, job)
	q.mu.Unlock()
	q.cond.Signal()
}

// Wakeup 唤醒可能正在 cond.Wait 上休眠的 worker，用于任务取消后让 worker 检查清理。
func (q *DownloadQueue) Wakeup() {
	q.cond.Signal()
}

func (q *DownloadQueue) CloseAndWait(timeout time.Duration) {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.closed = true
	q.mu.Unlock()
	q.cond.Broadcast()

	if timeout <= 0 {
		<-q.workerDone
		q.detachedWG.Wait()
		return
	}

	select {
	case <-q.workerDone:
	case <-time.After(timeout):
		log.Warnf("[download-queue] worker exit timed out after %s", timeout)
	}

	detachedDone := make(chan struct{})
	go func() {
		q.detachedWG.Wait()
		close(detachedDone)
	}()
	select {
	case <-detachedDone:
	case <-time.After(timeout):
		log.Warnf("[download-queue] detached runs still active after %s", timeout)
	}
}

func (q *DownloadQueue) workerLoop() {
	defer close(q.workerDone)
	for {
		job := q.nextJob()
		if job == nil {
			return
		}
		q.executeJob(job)
	}
}

func (q *DownloadQueue) nextJob() *downloadJob {
	q.mu.Lock()
	defer q.mu.Unlock()

	for {
		if q.closed && len(q.pending) == 0 {
			return nil
		}

		idx, job := q.nextRunnableLocked()
		if job != nil {
			q.pending = append(q.pending[:idx], q.pending[idx+1:]...)
			for _, key := range job.keys {
				q.heldTargets[key] = job.taskID
			}
			return job
		}
		q.cond.Wait()
	}
}

func (q *DownloadQueue) nextRunnableLocked() (int, *downloadJob) {
	q.cleanupStaleTargetsLocked()

	for i := 0; i < len(q.pending); {
		job := q.pending[i]
		snapshot, ok := q.server.taskManager.GetTask(job.taskID)
		if !ok || snapshot.Status != TaskStatusQueued {
			q.pending = append(q.pending[:i], q.pending[i+1:]...)
			continue
		}
		if q.conflictsLocked(job.keys) {
			i++
			continue
		}
		return i, job
	}
	return -1, nil
}

func (q *DownloadQueue) conflictsLocked(keys []targetKey) bool {
	for _, incoming := range keys {
		for held := range q.heldTargets {
			if targetsConflict(incoming, held) {
				return true
			}
		}
	}
	return false
}

func (q *DownloadQueue) cleanupStaleTargetsLocked() {
	for key, ownerID := range q.heldTargets {
		if _, ok := q.detached[ownerID]; ok {
			continue
		}
		snapshot, ok := q.server.taskManager.GetTask(ownerID)
		if !ok || isTerminalStatus(snapshot.Status) {
			delete(q.heldTargets, key)
		}
	}
}

func (q *DownloadQueue) executeJob(job *downloadJob) {
	if !q.server.taskManager.UpdateTaskStatus(job.taskID, TaskStatusRunning) {
		q.releaseTargets(job.taskID)
		return
	}

	reporter := NewSSEProgressReporter(q.server)
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("download task panic: %v", r)
			}
		}()
		done <- job.run(job.task.Ctx, job.taskID, reporter)
	}()

	select {
	case err := <-done:
		q.finishJob(job, err)
	case <-job.task.Ctx.Done():
		timer := time.NewTimer(q.cancelGrace)
		defer timer.Stop()
		select {
		case err := <-done:
			q.finishJob(job, err)
		case <-timer.C:
			q.detachJob(job, done)
		}
	}
}

func (q *DownloadQueue) finishJob(job *downloadJob, err error) {
	q.handleRunResult(job.taskID, err)
	q.releaseTargets(job.taskID)
}

func (q *DownloadQueue) detachJob(job *downloadJob, done <-chan error) {
	q.mu.Lock()
	q.detached[job.taskID] = &detachedRun{
		taskID:    job.taskID,
		startedAt: time.Now(),
		keys:      append([]targetKey(nil), job.keys...),
	}
	q.mu.Unlock()

	q.detachedWG.Add(1)
	go func() {
		defer q.detachedWG.Done()
		err := <-done
		q.handleRunResult(job.taskID, err)
		q.mu.Lock()
		delete(q.detached, job.taskID)
		q.mu.Unlock()
		q.releaseTargets(job.taskID)
	}()
}

func (q *DownloadQueue) handleRunResult(taskID string, err error) {
	if err == nil {
		return
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	taskSnapshot, ok := q.server.taskManager.GetTask(taskID)
	if !ok || isTerminalStatus(taskSnapshot.Status) {
		return
	}
	q.server.taskManager.SetTaskError(taskID, err)
}

func (q *DownloadQueue) releaseTargets(taskID string) {
	q.mu.Lock()
	for key, owner := range q.heldTargets {
		if owner == taskID {
			delete(q.heldTargets, key)
		}
	}
	q.mu.Unlock()
	q.cond.Broadcast()
}
