package scheduler

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
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
)

var scheduleIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

var randomIntervalDelay = func(interval time.Duration) time.Duration {
	if interval <= time.Nanosecond {
		return interval
	}
	return time.Duration(rand.Int63n(int64(interval-time.Nanosecond))) + time.Nanosecond
}

type Scheduler struct {
	configPath   string
	downloadFunc DownloadFunc
	entries      []ScheduleEntry
	parsed       []*ParsedSchedule
	statuses     []ScheduleStatus
	mu           sync.Mutex
	lifecycleMu  sync.Mutex
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	started      bool
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
	sc.startLocked()
}

func (sc *Scheduler) startLocked() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.started {
		return
	}

	sc.ctx, sc.cancel = context.WithCancel(context.Background())

	activeCount := 0
	for i, entry := range sc.entries {
		if !entry.Enabled {
			continue
		}
		activeCount++
		parsed := sc.parsed[i]
		var next time.Time
		switch parsed.Mode {
		case ScheduleModeInterval:
			next = time.Now().Add(parsed.Interval)
		case ScheduleModeDaily:
			next = nextDailyTrigger(parsed.Times)
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

	log.Infof("[scheduler] Scheduler started with %d active schedules (total: %d)", activeCount, len(sc.entries))
}

func (sc *Scheduler) Stop() {
	sc.lifecycleMu.Lock()
	defer sc.lifecycleMu.Unlock()
	sc.stopLocked()
}

func (sc *Scheduler) stopLocked() {
	sc.mu.Lock()
	if !sc.started {
		sc.mu.Unlock()
		return
	}
	if sc.cancel != nil {
		sc.cancel()
	}
	sc.started = false
	sc.cancel = nil
	sc.ctx = nil
	sc.mu.Unlock()

	sc.wg.Wait()
	log.Infoln("[scheduler] Scheduler stopped")
}

func (sc *Scheduler) Reload() error {
	sc.lifecycleMu.Lock()
	defer sc.lifecycleMu.Unlock()

	wasStarted := sc.isStartedLocked()
	sc.stopLocked()

	entries, parsed, statuses, err := sc.readConfig()
	if err != nil {
		if wasStarted {
			sc.startLocked()
		}
		return err
	}

	sc.mu.Lock()
	sc.entries = entries
	sc.parsed = parsed
	sc.statuses = statuses
	sc.mu.Unlock()

	if wasStarted {
		sc.startLocked()
	}
	activeCount := 0
	for _, e := range entries {
		if e.Enabled {
			activeCount++
		}
	}
	log.Infof("[scheduler] Config reloaded: %d schedules (%d active)", len(entries), activeCount)
	return nil
}

func (sc *Scheduler) isStartedLocked() bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.started
}

func (sc *Scheduler) IsRunning() bool {
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
	sc.mu.Lock()
	if index < 0 || index >= len(sc.entries) {
		sc.mu.Unlock()
		return "", fmt.Errorf("invalid schedule index: %d", index)
	}
	entry := sc.entries[index]
	if !entry.Enabled {
		sc.mu.Unlock()
		return "", fmt.Errorf("schedule #%d (%s) is disabled", index+1, entry.Name)
	}
	sc.mu.Unlock()

	return sc.triggerEntry(index, entry)
}

func (sc *Scheduler) TriggerByID(id string) (string, error) {
	sc.mu.Lock()
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
		sc.mu.Unlock()
		return "", fmt.Errorf("schedule %q not found", id)
	}
	if !entry.Enabled {
		sc.mu.Unlock()
		return "", fmt.Errorf("schedule %q (%s) is disabled", id, entry.Name)
	}
	sc.mu.Unlock()

	return sc.triggerEntry(index, entry)
}

func (sc *Scheduler) triggerEntry(index int, entry ScheduleEntry) (string, error) {
	taskID := sc.downloadFunc(entry)
	if taskID == "" {
		sc.updateStatus(index, entry, func(status *ScheduleStatus) {
			now := time.Now()
			status.LastRunAt = &now
			status.RunCount++
			status.LastError = "download function returned empty task_id"
			status.ConsecutiveFailures++
		})
		log.Warnf("[scheduler] Manual trigger failed [%s]: target=%s name=%q (empty task_id)", entry.Type, entry.Target, entry.Name)
		return "", fmt.Errorf("download function returned empty task_id")
	}

	sc.updateStatus(index, entry, func(status *ScheduleStatus) {
		now := time.Now()
		status.LastRunAt = &now
		status.LastTaskID = taskID
		status.LastError = ""
		status.RunCount++
		status.ConsecutiveFailures = 0
	})

	log.Infof("[scheduler] Manual trigger [%s]: target=%s name=%q task_id=%s", entry.Type, entry.Target, entry.Name, taskID)
	return taskID, nil
}

func (sc *Scheduler) runLoop(idx int) {
	sc.mu.Lock()
	entry := sc.entries[idx]
	parsed := sc.parsed[idx]
	ctx := sc.ctx
	sc.mu.Unlock()

	switch parsed.Mode {
	case ScheduleModeInterval:
		sc.runIntervalLoop(ctx, idx, entry, parsed)
	case ScheduleModeDaily:
		sc.runDailyLoop(ctx, idx, entry, parsed)
	}
}

func (sc *Scheduler) runIntervalLoop(ctx context.Context, idx int, entry ScheduleEntry, parsed *ParsedSchedule) {
	if entry.RunOnStart {
		sc.execute(idx, entry)
	} else {
		delay := randomIntervalDelay(parsed.Interval)
		if !sc.waitInterval(ctx, idx, entry, delay) {
			return
		}
	}

	ticker := time.NewTicker(parsed.Interval)
	defer ticker.Stop()
	sc.updateNextRunAt(idx, entry, time.Now().Add(parsed.Interval))

	for {
		select {
		case <-ticker.C:
			sc.execute(idx, entry)
			sc.updateNextRunAt(idx, entry, time.Now().Add(parsed.Interval))
		case <-ctx.Done():
			return
		}
	}
}

func (sc *Scheduler) waitInterval(ctx context.Context, idx int, entry ScheduleEntry, delay time.Duration) bool {
	sc.updateNextRunAt(idx, entry, time.Now().Add(delay))

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		sc.execute(idx, entry)
		return true
	case <-ctx.Done():
		return false
	}
}

func (sc *Scheduler) runDailyLoop(ctx context.Context, idx int, entry ScheduleEntry, parsed *ParsedSchedule) {
	for {
		next := nextDailyTrigger(parsed.Times)
		sc.updateNextRunAt(idx, entry, next)

		timer := time.NewTimer(time.Until(next))
		select {
		case <-timer.C:
			sc.execute(idx, entry)
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

func (sc *Scheduler) updateNextRunAt(idx int, entry ScheduleEntry, next time.Time) {
	sc.updateStatus(idx, entry, func(status *ScheduleStatus) {
		status.NextRunAt = &next
	})
}

func (sc *Scheduler) execute(idx int, entry ScheduleEntry) {
	taskID := sc.downloadFunc(entry)

	sc.updateStatus(idx, entry, func(status *ScheduleStatus) {
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
	})

	if taskID != "" {
		log.Infof("[scheduler] Scheduled download triggered [%s]: target=%s name=%q task_id=%s", entry.Type, entry.Target, entry.Name, taskID)
	} else {
		log.Warnf("[scheduler] Scheduled download failed [%s]: target=%s name=%q (empty task_id)", entry.Type, entry.Target, entry.Name)
	}
}

func (sc *Scheduler) updateStatus(idx int, entry ScheduleEntry, update func(*ScheduleStatus)) bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if idx < 0 || idx >= len(sc.entries) || idx >= len(sc.statuses) {
		return false
	}
	if sc.entries[idx] != entry {
		return false
	}
	update(&sc.statuses[idx])
	return true
}

func nextDailyTrigger(times []time.Time) time.Time {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	sorted := make([]time.Time, len(times))
	copy(sorted, times)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Hour()*60+sorted[i].Minute() < sorted[j].Hour()*60+sorted[j].Minute()
	})

	for _, t := range sorted {
		candidate := time.Date(today.Year(), today.Month(), today.Day(), t.Hour(), t.Minute(), 0, 0, today.Location())
		if candidate.After(now) {
			return candidate
		}
	}

	first := sorted[0]
	tomorrow := today.Add(24 * time.Hour)
	return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), first.Hour(), first.Minute(), 0, 0, tomorrow.Location())
}

func ValidateEntry(entry ScheduleEntry) error {
	switch entry.Type {
	case ScheduleTypeList, ScheduleTypeUser, ScheduleTypeFollowing:
	default:
		return fmt.Errorf("invalid type %q (must be list, user, or following)", entry.Type)
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
	return nil
}

func NormalizeEntries(entries []ScheduleEntry) ([]ScheduleEntry, error) {
	normalized := make([]ScheduleEntry, len(entries))
	copy(normalized, entries)

	used := make(map[string]struct{}, len(entries))
	for i := range normalized {
		id := strings.TrimSpace(normalized[i].ID)
		if id == "" {
			id = uniqueScheduleID(normalized[i], used)
		} else if !scheduleIDPattern.MatchString(id) {
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
	key := fmt.Sprintf("%s\n%s\n%s\n%s\n%t\n%t\n%t\n%t",
		entry.Type,
		strings.TrimSpace(entry.Target),
		strings.TrimSpace(entry.Name),
		strings.TrimSpace(entry.Schedule),
		entry.RunOnStart,
		entry.AutoFollow,
		entry.SkipProfile,
		entry.NoRetry,
	)
	sum := sha1.Sum([]byte(key))
	return "sch_" + hex.EncodeToString(sum[:])[:12]
}

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
		}
		if len(times) == 0 {
			return nil, fmt.Errorf("no daily times specified")
		}
		return &ParsedSchedule{Mode: ScheduleModeDaily, Times: times}, nil
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
				return "每天"
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
