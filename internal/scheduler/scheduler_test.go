package scheduler

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestTriggerSkipsStatusUpdateWhenReloadReplacesEntries(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "schedules.yaml")
	initial := []byte(`schedules:
  - type: user
    target: alice
    name: Alice
    schedule: "interval:1h"
    enabled: true
`)
	if err := os.WriteFile(configPath, initial, 0600); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	sc, err := New(configPath, func(entry ScheduleEntry) string {
		close(started)
		<-release
		return "task-1"
	})
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}
	t.Cleanup(sc.Stop)

	done := make(chan error, 1)
	go func() {
		_, err := sc.Trigger(0)
		done <- err
	}()

	<-started
	if err := os.WriteFile(configPath, []byte("schedules: []\n"), 0600); err != nil {
		t.Fatalf("write replacement config: %v", err)
	}
	if err := sc.Reload(); err != nil {
		t.Fatalf("reload scheduler: %v", err)
	}
	close(release)

	if err := <-done; err != nil {
		t.Fatalf("trigger returned error: %v", err)
	}
	if statuses := sc.GetStatuses(); len(statuses) != 0 {
		t.Fatalf("expected reloaded empty statuses, got %d", len(statuses))
	}
}

func TestStartIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "schedules.yaml")
	if err := os.WriteFile(configPath, []byte(`schedules:
  - type: user
    target: alice
    name: Alice
    schedule: "interval:1h"
    enabled: true
    run_on_start: true
`), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var calls atomic.Int32
	sc, err := New(configPath, func(entry ScheduleEntry) string {
		calls.Add(1)
		return "task-1"
	})
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}
	t.Cleanup(sc.Stop)

	sc.Start()
	sc.Start()

	waitFor(t, 200*time.Millisecond, func() bool {
		return calls.Load() >= 1
	})
	time.Sleep(50 * time.Millisecond)

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected one immediate scheduled run after duplicate Start, got %d", got)
	}
}

func TestReloadDoesNotStartStoppedScheduler(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "schedules.yaml")
	content := []byte(`schedules:
  - type: user
    target: alice
    name: Alice
    schedule: "interval:1h"
    enabled: true
    run_on_start: true
`)
	if err := os.WriteFile(configPath, content, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var calls atomic.Int32
	sc, err := New(configPath, func(entry ScheduleEntry) string {
		calls.Add(1)
		return "task-1"
	})
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}
	t.Cleanup(sc.Stop)

	if err := sc.Reload(); err != nil {
		t.Fatalf("reload scheduler: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if got := calls.Load(); got != 0 {
		t.Fatalf("expected Reload on a stopped scheduler not to start jobs, got %d calls", got)
	}
}

func TestReloadDoesNotTriggerRunOnStart(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "schedules.yaml")
	content := []byte(`schedules:
  - type: user
    target: alice
    name: Alice
    schedule: "interval:1h"
    enabled: true
    run_on_start: true
`)
	if err := os.WriteFile(configPath, content, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var calls atomic.Int32
	sc, err := New(configPath, func(entry ScheduleEntry) string {
		calls.Add(1)
		return "task-1"
	})
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}
	t.Cleanup(sc.Stop)

	sc.Start()
	waitFor(t, 200*time.Millisecond, func() bool {
		return calls.Load() >= 1
	})

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected one immediate run on first Start, got %d", got)
	}

	if err := sc.Reload(); err != nil {
		t.Fatalf("reload scheduler: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected Reload not to trigger run_on_start, got %d calls", got)
	}
}

func TestStopStartDoesNotTriggerRunOnStart(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "schedules.yaml")
	content := []byte(`schedules:
  - type: user
    target: alice
    name: Alice
    schedule: "interval:1h"
    enabled: true
    run_on_start: true
`)
	if err := os.WriteFile(configPath, content, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var calls atomic.Int32
	sc, err := New(configPath, func(entry ScheduleEntry) string {
		calls.Add(1)
		return "task-1"
	})
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}

	sc.Start()
	waitFor(t, 200*time.Millisecond, func() bool {
		return calls.Load() >= 1
	})

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected one immediate run on first Start, got %d", got)
	}

	sc.Stop()
	time.Sleep(50 * time.Millisecond)

	sc.Start()
	time.Sleep(50 * time.Millisecond)

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected Stop+Start not to trigger run_on_start again, got %d calls", got)
	}

	sc.Stop()
}

func TestIntervalDefaultDelaysFirstRun(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "schedules.yaml")
	if err := os.WriteFile(configPath, []byte(`schedules:
  - type: user
    target: alice
    name: Alice
    schedule: "interval:1h"
    enabled: true
`), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldRandomIntervalDelay := randomIntervalDelay
	randomIntervalDelay = func(interval time.Duration) time.Duration {
		return 80 * time.Millisecond
	}
	t.Cleanup(func() {
		randomIntervalDelay = oldRandomIntervalDelay
	})

	var calls atomic.Int32
	sc, err := New(configPath, func(entry ScheduleEntry) string {
		calls.Add(1)
		return "task-1"
	})
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}
	t.Cleanup(sc.Stop)

	sc.Start()
	time.Sleep(30 * time.Millisecond)
	if got := calls.Load(); got != 0 {
		t.Fatalf("expected default run_on_start=false to delay first run, got %d calls", got)
	}

	waitFor(t, 200*time.Millisecond, func() bool {
		return calls.Load() == 1
	})
}

func TestTriggerReturnsErrorWhenDownloadFuncReturnsEmptyTaskID(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "schedules.yaml")
	if err := os.WriteFile(configPath, []byte(`schedules:
  - type: user
    target: alice
    name: Alice
    schedule: "interval:1h"
    enabled: true
`), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	sc, err := New(configPath, func(entry ScheduleEntry) string {
		return ""
	})
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}

	taskID, err := sc.Trigger(0)
	if err == nil {
		t.Fatal("expected empty task_id to return an error")
	}
	if taskID != "" {
		t.Fatalf("expected empty task id, got %q", taskID)
	}

	statuses := sc.GetStatuses()
	if len(statuses) != 1 {
		t.Fatalf("expected one status, got %d", len(statuses))
	}
	if statuses[0].RunCount != 1 {
		t.Fatalf("expected failed trigger to increment run count, got %d", statuses[0].RunCount)
	}
	if statuses[0].LastError == "" {
		t.Fatal("expected failed trigger to record last error")
	}
}

func TestValidateEntryRejectsZeroListID(t *testing.T) {
	err := ValidateEntry(ScheduleEntry{
		Type:     ScheduleTypeList,
		Target:   "0",
		Schedule: "interval:1h",
		Enabled:  true,
	})
	if err == nil {
		t.Fatal("expected list target 0 to be invalid")
	}
}

func TestNormalizeEntriesAssignsStableUniqueIDs(t *testing.T) {
	entries, err := NormalizeEntries([]ScheduleEntry{
		{Type: ScheduleTypeUser, Target: "alice", Name: "Alice", Schedule: "interval:1h", Enabled: true},
		{Type: ScheduleTypeUser, Target: "alice", Name: "Alice", Schedule: "interval:1h", Enabled: true},
	})
	if err != nil {
		t.Fatalf("normalize entries: %v", err)
	}
	if entries[0].ID == "" || entries[1].ID == "" {
		t.Fatalf("expected generated ids, got %q and %q", entries[0].ID, entries[1].ID)
	}
	if entries[0].ID == entries[1].ID {
		t.Fatalf("expected duplicate entries to receive unique ids, got %q", entries[0].ID)
	}
}

func TestNormalizeEntriesRejectsDuplicateExplicitIDs(t *testing.T) {
	_, err := NormalizeEntries([]ScheduleEntry{
		{ID: "same", Type: ScheduleTypeUser, Target: "alice", Schedule: "interval:1h", Enabled: true},
		{ID: "same", Type: ScheduleTypeUser, Target: "bob", Schedule: "interval:1h", Enabled: true},
	})
	if err == nil {
		t.Fatal("expected duplicate explicit ids to be rejected")
	}
}

func TestParseScheduleTrimsValues(t *testing.T) {
	interval, err := ParseSchedule("interval: 2h")
	if err != nil {
		t.Fatalf("parse interval: %v", err)
	}
	if interval.Interval != 2*time.Hour {
		t.Fatalf("expected 2h interval, got %s", interval.Interval)
	}

	daily, err := ParseSchedule("daily: 07:00, 21:00")
	if err != nil {
		t.Fatalf("parse daily: %v", err)
	}
	if len(daily.Times) != 2 {
		t.Fatalf("expected two daily times, got %d", len(daily.Times))
	}
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}
