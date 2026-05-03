package scheduler

import "time"

type ScheduleType string

const (
	ScheduleTypeList      ScheduleType = "list"
	ScheduleTypeUser      ScheduleType = "user"
	ScheduleTypeFollowing ScheduleType = "following"
)

type ScheduleMode string

const (
	ScheduleModeInterval ScheduleMode = "interval"
	ScheduleModeDaily    ScheduleMode = "daily"
)

type ScheduleConfig struct {
	Schedules []ScheduleEntry `yaml:"schedules" json:"schedules"`
}

type ScheduleEntry struct {
	ID          string       `yaml:"id" json:"id"`
	Type        ScheduleType `yaml:"type" json:"type"`
	Target      string       `yaml:"target" json:"target"`
	Name        string       `yaml:"name" json:"name"`
	Schedule    string       `yaml:"schedule" json:"schedule"`
	Enabled     bool         `yaml:"enabled" json:"enabled"`
	RunOnStart  bool         `yaml:"run_on_start" json:"run_on_start"`
	AutoFollow  bool         `yaml:"auto_follow" json:"auto_follow"`
	SkipProfile bool         `yaml:"skip_profile" json:"skip_profile"`
	NoRetry     bool         `yaml:"no_retry" json:"no_retry"`
}

type ParsedSchedule struct {
	Mode     ScheduleMode
	Interval time.Duration
	Times    []time.Time
}

type ScheduleStatus struct {
	Entry               ScheduleEntry `json:"entry"`
	ScheduleDisplay     string        `json:"schedule_display"`
	LastRunAt           *time.Time    `json:"last_run_at,omitempty"`
	NextRunAt           *time.Time    `json:"next_run_at,omitempty"`
	RunCount            int           `json:"run_count"`
	LastTaskID          string        `json:"last_task_id,omitempty"`
	LastError           string        `json:"last_error,omitempty"`
	ConsecutiveFailures int           `json:"consecutive_failures"`
}

type DownloadFunc func(entry ScheduleEntry) string
