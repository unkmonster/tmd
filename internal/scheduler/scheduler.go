package scheduler

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"math/rand"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/unkmonster/tmd/internal/utils"
)

var ScheduleIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// randomIntervalDelay 根据 entry ID 的哈希值在 interval 中确定一个位置，
// 同一个 entry 始终得到相同的延迟，不同 entry 自然均匀散开，避免扎堆。
var randomIntervalDelay = func(interval time.Duration, entryID string) time.Duration {
	if interval <= time.Nanosecond {
		return interval
	}
	if entryID == "" {
		// 回退到随机延迟（用于没有 ID 的条目，如测试中的配置）
		return time.Duration(rand.Int63n(int64(interval-time.Nanosecond))) + time.Nanosecond
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(entryID))
	v := h.Sum64()
	// 取模限制到 [1ns, interval) 范围内
	pos := v % uint64(interval)
	if pos == 0 {
		pos = 1 // 至少 1ns，避免零延迟
	}
	return time.Duration(pos)
}

type ScheduleStatusChangeFunc func(statuses []ScheduleStatus)

type Scheduler struct {
	configPath     string
	downloadFunc   DownloadFunc
	entries        []ScheduleEntry
	parsed         []*ParsedSchedule
	statuses       []ScheduleStatus
	mu             sync.Mutex
	lifecycleMu    sync.Mutex
	generation     int64
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	started        bool
	firstStart     bool
	hasEverStarted bool
	OnStatusChange ScheduleStatusChangeFunc
}

func New(configPath string, downloadFunc DownloadFunc) (*Scheduler, error) {
	sc := &Scheduler{
		configPath:   configPath,
		downloadFunc: downloadFunc,
	}
	if err := sc.loadConfig(); err != nil {
		return nil, err
	}
	return sc, nil
}

func (sc *Scheduler) loadConfig() error {
	entries, parsed, statuses, err := sc.readConfig()
	if err != nil {
		return err
	}
	sc.entries = entries
	sc.parsed = parsed
	sc.statuses = statuses
	return nil
}

func (sc *Scheduler) readConfig() ([]ScheduleEntry, []*ParsedSchedule, []ScheduleStatus, error) {
	data, err := os.ReadFile(sc.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil, nil
		}
		return nil, nil, nil, fmt.Errorf("failed to read schedules config: %w", err)
	}

	var cfg ScheduleConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse schedules config: %w", err)
	}

	entries, err := NormalizeEntries(cfg.Schedules)
	if err != nil {
		return nil, nil, nil, err
	}

	parsed := make([]*ParsedSchedule, len(entries))
	statuses := make([]ScheduleStatus, len(entries))
	for i, entry := range entries {
		if err := ValidateEntry(entry); err != nil {
			return nil, nil, nil, fmt.Errorf("schedule #%d (%s): %w", i+1, entry.Name, err)
		}
		p, err := ParseSchedule(entry.Schedule)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("schedule #%d (%s): %w", i+1, entry.Name, err)
		}
		parsed[i] = p
		statuses[i] = ScheduleStatus{
			Entry:           entry,
			ScheduleDisplay: FormatScheduleDisplay(p, entry.Schedule),
		}
	}

	return entries, parsed, statuses, nil
}

func (sc *Scheduler) Start() {
	sc.lifecycleMu.Lock()
	defer sc.lifecycleMu.Unlock()

	activeCount, started := sc.startLocked(true)
	if !started {
		return
	}
	log.Infof("[scheduler] Scheduler started with %d active schedules (total: %d)", activeCount, len(sc.entries))
}

func (sc *Scheduler) Stop() {
	sc.lifecycleMu.Lock()
	defer sc.lifecycleMu.Unlock()

	if !sc.stopLocked() {
		log.Debugln("[scheduler] Stop: already stopped, skipping")
		return
	}
	log.Infoln("[scheduler] Scheduler stopped")
}

func (sc *Scheduler) startLocked(skipIfRunning bool) (int, bool) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if !sc.hasEverStarted {
		sc.firstStart = true
		sc.hasEverStarted = true
	}
	if sc.started && skipIfRunning {
		return 0, false
	}

	log.Debugln("[scheduler] Start: initializing context and launching goroutines")
	return sc.launchGoroutinesLocked(), true
}

func (sc *Scheduler) stopLocked() bool {
	log.Debugln("[scheduler] Stop: acquiring mu...")
	sc.mu.Lock()
	if !sc.started {
		sc.mu.Unlock()
		return false
	}

	log.Debugln("[scheduler] Stop: cancelling context and waiting for goroutines")
	if sc.cancel != nil {
		sc.cancel()
	}
	sc.started = false
	sc.firstStart = false
	sc.mu.Unlock()

	sc.wg.Wait()
	return true
}

func (sc *Scheduler) launchGoroutinesLocked() int {
	sc.ctx, sc.cancel = context.WithCancel(context.Background())
	activeCount := 0
	for i, entry := range sc.entries {
		if !entry.Enabled {
			continue
		}
		if i >= len(sc.parsed) || i >= len(sc.statuses) {
			log.Warnf("[scheduler] launchGoroutinesLocked[%d]: inconsistent scheduler state, skipping entry %q", i, entry.Name)
			continue
		}
		activeCount++
		p := sc.parsed[i]
		var next time.Time
		switch p.Mode {
		case ScheduleModeInterval:
			next = time.Now().Add(p.Interval)
		case ScheduleModeDaily:
			next = nextDailyTrigger(p.SortedTimes)
		}
		sc.statuses[i].NextRunAt = &next
		idx := i
		sc.wg.Add(1)
		go func() {
			defer sc.wg.Done()
			sc.runLoop(idx)
		}()
	}
	sc.started = true
	return activeCount
}

func (sc *Scheduler) entrySnapshot(idx int) (ScheduleEntry, *ParsedSchedule, context.Context, bool, int64, bool) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if idx < 0 || idx >= len(sc.entries) || idx >= len(sc.parsed) || sc.parsed[idx] == nil {
		return ScheduleEntry{}, nil, nil, false, 0, false
	}
	ctx := sc.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return sc.entries[idx], sc.parsed[idx], ctx, sc.firstStart, sc.generation, true
}

func (sc *Scheduler) Reload() error {
	log.Debugln("[scheduler] Reload: acquiring lifecycleMu...")
	sc.lifecycleMu.Lock()
	log.Debugln("[scheduler] Reload: lifecycleMu acquired")

	wasStarted := sc.isRunningLockedLifecycle()
	log.Debugf("[scheduler] Reload: wasStarted=%v, stopping scheduler", wasStarted)

	if wasStarted && sc.stopLocked() {
		log.Infoln("[scheduler] Scheduler stopped (for reload)")
	}

	entries, parsed, statuses, err := sc.readConfig()
	if err != nil {
		log.Warnf("[scheduler] Reload: readConfig failed (wasStarted=%v): %v", wasStarted, err)
		if wasStarted {
			log.Debugln("[scheduler] Reload: recovering start after readConfig failure")
			activeCount, _ := sc.startLocked(false)
			log.Infof("[scheduler] Scheduler recovered with %d active schedules", activeCount)
		}
		callback, statusesCopy := sc.statusChangeSnapshot()
		sc.lifecycleMu.Unlock()
		notifyStatusChange(callback, statusesCopy)
		return err
	}

	sc.applyConfig(entries, parsed, statuses)

	if wasStarted {
		log.Debugln("[scheduler] Restarting scheduler after reload")
		log.Debugln("[scheduler] Reload: starting fresh goroutines")
		activeCount, _ := sc.startLocked(false)
		log.Infof("[scheduler] Scheduler restarted with %d active schedules", activeCount)
	}

	callback, statusesCopy := sc.statusChangeSnapshot()
	sc.lifecycleMu.Unlock()
	notifyStatusChange(callback, statusesCopy)
	logReloadSummary(entries)
	return nil
}

func (sc *Scheduler) IsRunning() bool {
	sc.lifecycleMu.Lock()
	defer sc.lifecycleMu.Unlock()
	return sc.isRunningLockedLifecycle()
}

func (sc *Scheduler) isRunningLockedLifecycle() bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.started
}

func (sc *Scheduler) GetStatuses() []ScheduleStatus {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	result := make([]ScheduleStatus, len(sc.statuses))
	copy(result, sc.statuses)
	return result
}

func (sc *Scheduler) Trigger(index int) (string, error) {
	idx, entry, err := sc.findEnabledEntry(index)
	if err != nil {
		return "", err
	}
	return sc.triggerEntry(idx, entry)
}

func (sc *Scheduler) TriggerByID(id string) (string, error) {
	idx, entry, err := sc.findEnabledEntryByID(id)
	if err != nil {
		return "", err
	}
	return sc.triggerEntry(idx, entry)
}

func (sc *Scheduler) findEnabledEntry(index int) (int, ScheduleEntry, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if index < 0 || index >= len(sc.entries) {
		return 0, ScheduleEntry{}, fmt.Errorf("invalid schedule index: %d", index)
	}
	entry := sc.entries[index]
	if !entry.Enabled {
		return 0, ScheduleEntry{}, fmt.Errorf("schedule #%d (%s) is disabled", index+1, entry.Name)
	}
	return index, entry, nil
}

func (sc *Scheduler) findEnabledEntryByID(id string) (int, ScheduleEntry, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	index := -1
	var entry ScheduleEntry
	for i, e := range sc.entries {
		if e.ID == id {
			index = i
			entry = e
			break
		}
	}
	if index < 0 {
		return 0, ScheduleEntry{}, fmt.Errorf("schedule %q not found", id)
	}
	if !entry.Enabled {
		return 0, ScheduleEntry{}, fmt.Errorf("schedule %q (%s) is disabled", id, entry.Name)
	}
	return index, entry, nil
}

func (sc *Scheduler) triggerEntry(index int, entry ScheduleEntry) (taskID string, err error) {
	gen, ok := sc.tryAcquireExecution(index, entry, nil)
	if !ok {
		return "", fmt.Errorf("schedule %q is already triggering, please wait for the current trigger to complete", entry.Name)
	}

	var released bool
	defer func() {
		if r := recover(); r != nil {
			if !released {
				now := time.Now()
				sc.releaseAndUpdateStatus(index, entry, gen, func(status *ScheduleStatus) {
					status.LastRunAt = &now
					status.RunCount++
					status.LastError = fmt.Sprintf("panic: %v", r)
					status.ConsecutiveFailures++
				})
			}
			panic(r)
		}
	}()

	taskID = sc.downloadFunc(entry)
	if taskID == "" {
		if !sc.releaseAndUpdateStatus(index, entry, gen, func(status *ScheduleStatus) {
			now := time.Now()
			status.LastRunAt = &now
			status.RunCount++
			status.LastError = "download function returned empty task_id"
			status.ConsecutiveFailures++
		}) {
			log.Warnf("[scheduler] triggerEntry[%d]: status update rejected for empty-task_id failure on entry %q", index, entry.Name)
		}
		released = true
		log.Warnf("[scheduler] Manual trigger failed [%s]: target=%s name=%q (empty task_id)", entry.Type, entry.Target, entry.Name)
		return "", fmt.Errorf("download function returned empty task_id")
	}

	if !sc.releaseAndUpdateStatus(index, entry, gen, func(status *ScheduleStatus) {
		now := time.Now()
		status.LastRunAt = &now
		status.LastTaskID = taskID
		status.LastError = ""
		status.RunCount++
		status.ConsecutiveFailures = 0
	}) {
		log.Warnf("[scheduler] triggerEntry[%d]: status update rejected for successful trigger on entry %q (task_id=%s)", index, entry.Name, taskID)
	}
	released = true

	log.Infof("[scheduler] Manual trigger [%s]: target=%s name=%q task_id=%s", entry.Type, entry.Target, entry.Name, taskID)
	return taskID, nil
}

func (sc *Scheduler) runLoop(idx int) {
	entry, parsed, ctx, firstStart, gen, ok := sc.entrySnapshot(idx)
	if !ok {
		log.Warnf("[scheduler] runLoop[%d]: invalid schedule index, exiting", idx)
		return
	}

	switch parsed.Mode {
	case ScheduleModeInterval:
		sc.runIntervalLoop(ctx, idx, entry, parsed, firstStart, gen)
	case ScheduleModeDaily:
		sc.runDailyLoop(ctx, idx, entry, parsed, gen)
	}
}

func (sc *Scheduler) isStale(gen int64) bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return gen != sc.generation
}

func (sc *Scheduler) runIntervalLoop(ctx context.Context, idx int, entry ScheduleEntry, parsed *ParsedSchedule, firstStart bool, gen int64) {
	if sc.isStale(gen) {
		log.Warnf("[scheduler] runIntervalLoop[%d]: stale generation detected on entry, exiting", idx)
		return
	}
	if entry.RunOnStart && firstStart {
		sc.execute(idx, entry, gen)
	} else {
		delay := randomIntervalDelay(parsed.Interval, entry.ID)
		if !sc.waitInterval(ctx, idx, entry, delay, gen) {
			return
		}
	}

	ticker := time.NewTicker(parsed.Interval)
	defer ticker.Stop()
	sc.updateNextRunAt(idx, entry, time.Now().Add(parsed.Interval), gen)

	for {
		select {
		case <-ticker.C:
			if sc.isStale(gen) {
				log.Warnf("[scheduler] runIntervalLoop[%d]: stale generation at tick, exiting", idx)
				return
			}
			sc.execute(idx, entry, gen)
			sc.updateNextRunAt(idx, entry, time.Now().Add(parsed.Interval), gen)
		case <-ctx.Done():
			return
		}
	}
}

func (sc *Scheduler) waitInterval(ctx context.Context, idx int, entry ScheduleEntry, delay time.Duration, gen int64) bool {
	sc.updateNextRunAt(idx, entry, time.Now().Add(delay), gen)

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		sc.execute(idx, entry, gen)
		return true
	case <-ctx.Done():
		return false
	}
}

func (sc *Scheduler) runDailyLoop(ctx context.Context, idx int, entry ScheduleEntry, parsed *ParsedSchedule, gen int64) {
	for {
		next := nextDailyTrigger(parsed.SortedTimes)
		if next.IsZero() {
			log.Warnf("[scheduler] runDailyLoop[%d]: empty times, stopping daily loop", idx)
			return
		}
		sc.updateNextRunAt(idx, entry, next, gen)

		timer := time.NewTimer(time.Until(next))
		select {
		case <-timer.C:
			if sc.isStale(gen) {
				log.Warnf("[scheduler] runDailyLoop[%d]: stale generation at daily trigger, exiting", idx)
				return
			}
			sc.execute(idx, entry, gen)
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
	}
}

func (sc *Scheduler) updateNextRunAt(idx int, entry ScheduleEntry, next time.Time, gen int64) {
	if sc.isStale(gen) {
		log.Warnf("[scheduler] updateNextRunAt[%d]: skipping stale update for entry %q", idx, entry.Name)
		return
	}
	if !sc.updateStatus(idx, entry, func(status *ScheduleStatus) {
		status.NextRunAt = &next
	}) {
		log.Warnf("[scheduler] updateNextRunAt[%d]: status update rejected (entry %q may have been reloaded)", idx, entry.Name)
	}
}

func (sc *Scheduler) execute(idx int, entry ScheduleEntry, gen int64) {
	if sc.isStale(gen) {
		log.Warnf("[scheduler] execute[%d]: skipping stale execution for entry %q", idx, entry.Name)
		return
	}
	acquiredGen, ok := sc.tryAcquireExecution(idx, entry, &gen)
	if !ok {
		log.Infof("[scheduler] execute[%d]: entry %q is already triggering, skipping duplicate trigger", idx, entry.Name)
		return
	}

	var taskID string
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[scheduler] execute[%d]: panic in downloadFunc for entry %q: %v", idx, entry.Name, r)
			now := time.Now()
			sc.releaseAndUpdateStatus(idx, entry, acquiredGen, func(status *ScheduleStatus) {
				status.LastRunAt = &now
				status.RunCount++
				status.LastError = fmt.Sprintf("panic: %v", r)
				status.ConsecutiveFailures++
			})
		}
	}()

	taskID = sc.downloadFunc(entry)

	if !sc.releaseAndUpdateStatus(idx, entry, acquiredGen, func(status *ScheduleStatus) {
		now := time.Now()
		status.LastRunAt = &now
		status.RunCount++
		if taskID != "" {
			status.LastTaskID = taskID
			status.LastError = ""
			status.ConsecutiveFailures = 0
		} else {
			status.LastError = "download function returned empty task_id"
			status.ConsecutiveFailures++
		}
	}) {
		log.Warnf("[scheduler] execute[%d]: status update rejected for entry %q (reloaded?), task_id=%s", idx, entry.Name, taskID)
	}

	if taskID != "" {
		log.Infof("[scheduler] Scheduled download triggered [%s]: target=%s name=%q task_id=%s", entry.Type, entry.Target, entry.Name, taskID)
	} else {
		log.Warnf("[scheduler] Scheduled download failed [%s]: target=%s name=%q (empty task_id)", entry.Type, entry.Target, entry.Name)
	}
}

func (sc *Scheduler) updateStatus(idx int, entry ScheduleEntry, update func(*ScheduleStatus)) bool {
	sc.mu.Lock()
	if idx < 0 || idx >= len(sc.entries) || idx >= len(sc.statuses) {
		sc.mu.Unlock()
		return false
	}
	if sc.entries[idx].ID != entry.ID {
		sc.mu.Unlock()
		return false
	}
	update(&sc.statuses[idx])
	callback, statuses := sc.statusChangeSnapshotLocked()
	sc.mu.Unlock()

	notifyStatusChange(callback, statuses)
	return true
}

func (sc *Scheduler) tryAcquireExecution(idx int, entry ScheduleEntry, expectedGen *int64) (int64, bool) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if expectedGen != nil && sc.generation != *expectedGen {
		return 0, false
	}
	if idx < 0 || idx >= len(sc.statuses) || idx >= len(sc.entries) {
		return 0, false
	}
	if sc.entries[idx].ID != entry.ID {
		return 0, false
	}
	if sc.statuses[idx].Triggering {
		return 0, false
	}
	sc.statuses[idx].Triggering = true
	return sc.generation, true
}

func (sc *Scheduler) releaseAndUpdateStatus(idx int, entry ScheduleEntry, gen int64, update func(*ScheduleStatus)) bool {
	sc.mu.Lock()

	if sc.generation != gen {
		sc.mu.Unlock()
		return false
	}
	if idx < 0 || idx >= len(sc.statuses) || idx >= len(sc.entries) {
		sc.mu.Unlock()
		return false
	}
	if sc.entries[idx].ID != entry.ID {
		sc.mu.Unlock()
		return false
	}
	update(&sc.statuses[idx])
	sc.statuses[idx].Triggering = false

	callback, statuses := sc.statusChangeSnapshotLocked()
	sc.mu.Unlock()

	notifyStatusChange(callback, statuses)
	return true
}

func (sc *Scheduler) applyConfig(entries []ScheduleEntry, parsed []*ParsedSchedule, statuses []ScheduleStatus) {
	log.Debugf("[scheduler] Reload: swapping %d entries into scheduler (generation++)", len(entries))
	sc.mu.Lock()
	sc.entries = entries
	sc.parsed = parsed
	sc.statuses = statuses
	sc.generation++
	sc.mu.Unlock()
}

func (sc *Scheduler) statusChangeSnapshot() (ScheduleStatusChangeFunc, []ScheduleStatus) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.statusChangeSnapshotLocked()
}

func (sc *Scheduler) statusChangeSnapshotLocked() (ScheduleStatusChangeFunc, []ScheduleStatus) {
	callback := sc.OnStatusChange
	if callback == nil {
		return nil, nil
	}

	statuses := make([]ScheduleStatus, len(sc.statuses))
	copy(statuses, sc.statuses)
	return callback, statuses
}

func notifyStatusChange(callback ScheduleStatusChangeFunc, statuses []ScheduleStatus) {
	if callback != nil {
		callback(statuses)
	}
}

func logReloadSummary(entries []ScheduleEntry) {
	activeCount := 0
	for _, entry := range entries {
		if entry.Enabled {
			activeCount++
		}
	}
	log.Infof("[scheduler] Config reloaded: %d schedules (%d active)", len(entries), activeCount)
}

func nextDailyTrigger(times []time.Time) time.Time {
	if len(times) == 0 {
		return time.Time{}
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for _, t := range times {
		candidate := time.Date(today.Year(), today.Month(), today.Day(), t.Hour(), t.Minute(), 0, 0, today.Location())
		if candidate.After(now) {
			return candidate
		}
	}

	first := times[0]
	tomorrow := today.AddDate(0, 0, 1)
	return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), first.Hour(), first.Minute(), 0, 0, tomorrow.Location())
}

func canonicalizeScheduleEntry(entry ScheduleEntry) ScheduleEntry {
	entry.ID = strings.TrimSpace(entry.ID)
	entry.Name = strings.TrimSpace(entry.Name)
	entry.Schedule = strings.TrimSpace(entry.Schedule)

	if entry.Type == ScheduleTypeMixed {
		entry.Target = ""
		entry.Users = canonicalizeScreenNameSlice(entry.Users)
		entry.Lists = trimStringSliceKeepEmpty(entry.Lists)
		entry.FollowingNames = canonicalizeScreenNameSlice(entry.FollowingNames)
		return entry
	}

	entry.Target = strings.TrimSpace(entry.Target)
	entry.Users = nil
	entry.Lists = nil
	entry.FollowingNames = nil
	return entry
}

func canonicalizeScreenNameSlice(values []string) []string {
	if values == nil {
		return nil
	}
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = utils.NormalizeScreenName(strings.TrimSpace(value))
	}
	return out
}

func trimStringSliceKeepEmpty(values []string) []string {
	if values == nil {
		return nil
	}
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = strings.TrimSpace(value)
	}
	return out
}

func ValidateEntry(entry ScheduleEntry) error {
	entry = canonicalizeScheduleEntry(entry)

	switch entry.Type {
	case ScheduleTypeList, ScheduleTypeUser, ScheduleTypeFollowing, ScheduleTypeMixed:
	default:
		return fmt.Errorf("invalid type %q (must be list, user, following, or mixed)", entry.Type)
	}

	if entry.Type == ScheduleTypeMixed {
		hasUsers := len(entry.Users) > 0
		hasLists := len(entry.Lists) > 0
		hasFollowing := len(entry.FollowingNames) > 0
		if !hasUsers && !hasLists && !hasFollowing {
			return fmt.Errorf("mixed type requires at least one of users, lists, or following_names")
		}

		for i, name := range entry.Users {
			if name == "" {
				return fmt.Errorf("mixed users[%d]: empty value", i)
			}
			if !utils.IsValidScreenName(name) {
				return fmt.Errorf("mixed users[%d]: invalid screen name format %q", i, name)
			}
		}
		for i, raw := range entry.Lists {
			if raw == "" {
				return fmt.Errorf("mixed lists[%d]: empty value", i)
			}
			listID, err := strconv.ParseUint(raw, 10, 64)
			if err != nil || listID == 0 {
				return fmt.Errorf("mixed lists[%d]: invalid list_id %q (must be a positive integer)", i, raw)
			}
		}
		for i, name := range entry.FollowingNames {
			if name == "" {
				return fmt.Errorf("mixed following_names[%d]: empty value", i)
			}
			if !utils.IsValidScreenName(name) {
				return fmt.Errorf("mixed following_names[%d]: invalid screen name format %q", i, name)
			}
		}
		return nil
	}

	if strings.TrimSpace(entry.Target) == "" {
		return fmt.Errorf("target cannot be empty")
	}
	if entry.Type == ScheduleTypeList {
		listID, err := strconv.ParseUint(entry.Target, 10, 64)
		if err != nil || listID == 0 {
			return fmt.Errorf("invalid list_id %q: must be a positive integer", entry.Target)
		}
	}
	if entry.Type == ScheduleTypeUser || entry.Type == ScheduleTypeFollowing {
		if !utils.IsValidScreenName(entry.Target) {
			return fmt.Errorf("invalid screen_name %q: must be 1-15 characters (letters, digits, underscores)", entry.Target)
		}
	}
	return nil
}

func NormalizeEntries(entries []ScheduleEntry) ([]ScheduleEntry, error) {
	normalized := make([]ScheduleEntry, len(entries))
	copy(normalized, entries)

	used := make(map[string]struct{}, len(entries))
	for i := range normalized {
		normalized[i] = canonicalizeScheduleEntry(normalized[i])
		id := normalized[i].ID
		if id == "" {
			id = uniqueScheduleID(normalized[i], used)
		} else if !ScheduleIDPattern.MatchString(id) {
			return nil, fmt.Errorf("schedule #%d (%s): invalid id %q (use letters, numbers, '_' or '-')", i+1, normalized[i].Name, id)
		} else if _, exists := used[id]; exists {
			return nil, fmt.Errorf("schedule #%d (%s): duplicate id %q", i+1, normalized[i].Name, id)
		}
		normalized[i].ID = id
		used[id] = struct{}{}
	}

	return normalized, nil
}

func NewEntryID(entry ScheduleEntry, used map[string]struct{}) string {
	if used == nil {
		used = map[string]struct{}{}
	}
	return uniqueScheduleID(entry, used)
}

func uniqueScheduleID(entry ScheduleEntry, used map[string]struct{}) string {
	base := scheduleIDBase(entry)
	id := base
	for suffix := 2; ; suffix++ {
		if _, exists := used[id]; !exists {
			return id
		}
		id = fmt.Sprintf("%s-%d", base, suffix)
	}
}

func scheduleIDBase(entry ScheduleEntry) string {
	entry = canonicalizeScheduleEntry(entry)

	targetsKey := entry.Target
	if entry.Type == ScheduleTypeMixed {
		parts := make([]string, 0, len(entry.Users)+len(entry.Lists)+len(entry.FollowingNames))
		for _, value := range entry.Users {
			parts = append(parts, "u:"+value)
		}
		for _, value := range entry.Lists {
			parts = append(parts, "l:"+value)
		}
		for _, value := range entry.FollowingNames {
			parts = append(parts, "f:"+value)
		}
		sort.Strings(parts)
		targetsKey = strings.Join(parts, "|")
	}

	key := fmt.Sprintf("%s\n%s\n%s\n%s\n%t\n%t\n%t\n%t\n%t",
		entry.Type,
		targetsKey,
		entry.Name,
		entry.Schedule,
		entry.RunOnStart,
		entry.AutoFollow,
		entry.FollowMembers,
		entry.SkipProfile,
		entry.NoRetry,
	)
	sum := sha1.Sum([]byte(key))
	return "sch_" + hex.EncodeToString(sum[:])[:12]
}

const maxDailyTimes = 96

func ParseSchedule(raw string) (*ParsedSchedule, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "interval:") {
		intervalStr := strings.TrimSpace(strings.TrimPrefix(raw, "interval:"))
		d, err := time.ParseDuration(intervalStr)
		if err != nil {
			return nil, fmt.Errorf("invalid interval %q: %w", intervalStr, err)
		}
		if d < time.Minute {
			return nil, fmt.Errorf("interval too short: %s (minimum 1m)", d)
		}
		return &ParsedSchedule{Mode: ScheduleModeInterval, Interval: d}, nil
	}
	if strings.HasPrefix(raw, "daily:") {
		timesStr := strings.TrimSpace(strings.TrimPrefix(raw, "daily:"))
		var times []time.Time
		for _, ts := range strings.Split(timesStr, ",") {
			ts = strings.TrimSpace(ts)
			t, err := time.Parse("15:04", ts)
			if err != nil {
				return nil, fmt.Errorf("invalid daily time %q: %w", ts, err)
			}
			times = append(times, t)
			if len(times) > maxDailyTimes {
				return nil, fmt.Errorf("too many daily times: %d (maximum %d)", len(times), maxDailyTimes)
			}
		}
		if len(times) == 0 {
			return nil, fmt.Errorf("no daily times specified")
		}
		sorted := make([]time.Time, len(times))
		copy(sorted, times)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })
		return &ParsedSchedule{Mode: ScheduleModeDaily, Times: times, SortedTimes: sorted}, nil
	}
	return nil, fmt.Errorf("unknown schedule format: %q (use 'interval:2h' or 'daily:07:00,21:00')", raw)
}

func FormatScheduleDisplay(parsed *ParsedSchedule, raw string) string {
	switch parsed.Mode {
	case ScheduleModeInterval:
		d := int(parsed.Interval.Hours()) / 24
		h := int(parsed.Interval.Hours()) % 24
		m := int(parsed.Interval.Minutes()) % 60
		if d > 0 && h == 0 && m == 0 {
			if d == 1 {
				return "每 24 小时"
			}
			return fmt.Sprintf("每 %d 天", d)
		}
		if d > 0 {
			parts := []string{fmt.Sprintf("%d 天", d)}
			if h > 0 {
				parts = append(parts, fmt.Sprintf("%d 小时", h))
			}
			return "每 " + strings.Join(parts, " ")
		}
		if h > 0 && m > 0 {
			return fmt.Sprintf("每 %d 小时 %d 分钟", h, m)
		} else if h > 0 {
			return fmt.Sprintf("每 %d 小时", h)
		}
		return fmt.Sprintf("每 %d 分钟", m)
	case ScheduleModeDaily:
		parts := make([]string, len(parsed.Times))
		for i, t := range parsed.Times {
			parts[i] = t.Format("15:04")
		}
		return "每天 " + strings.Join(parts, ", ")
	}
	return raw
}
