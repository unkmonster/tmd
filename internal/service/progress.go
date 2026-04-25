package service

// Progress 下载进度
type Progress struct {
	Stage     string // "syncing", "downloading", "retrying", "profile", "marking", "completed"
	Total     int
	Completed int
	Failed    int
	Current   string // 当前处理的用户/列表
}

// Result 执行结果
type Result struct {
	Downloaded int
	Failed     int
	Skipped    int
	Message    string
}

// ProgressReporter 进度报告接口
type ProgressReporter interface {
	OnProgress(taskID string, p Progress)
	OnComplete(taskID string, r Result)
	OnError(taskID string, err error)
}

// NopReporter 空报告器（用于不需要进度报告的场景）
type NopReporter struct{}

func (n *NopReporter) OnProgress(taskID string, p Progress) {}
func (n *NopReporter) OnComplete(taskID string, r Result)   {}
func (n *NopReporter) OnError(taskID string, err error)     {}

// LogReporter 日志报告器（用于 CLI 模式）
type LogReporter struct {
	logger func(format string, args ...interface{})
}

func NewLogReporter(logger func(format string, args ...interface{})) ProgressReporter {
	return &LogReporter{logger: logger}
}

func (l *LogReporter) OnProgress(taskID string, p Progress) {
	if l.logger == nil {
		return
	}
	switch p.Stage {
	case "syncing":
		l.logger("[%s] Syncing: %s", taskID, p.Current)
	case "downloading":
		if p.Total > 0 {
			l.logger("[%s] Downloading: %s (%d/%d)", taskID, p.Current, p.Completed, p.Total)
		} else {
			l.logger("[%s] Downloading: %s", taskID, p.Current)
		}
	case "profile":
		l.logger("[%s] Downloading profiles...", taskID)
	case "marking":
		l.logger("[%s] Marking: %s", taskID, p.Current)
	case "preparing":
		l.logger("[%s] Preparing...", taskID)
	default:
		l.logger("[%s] %s: %s", taskID, p.Stage, p.Current)
	}
}

func (l *LogReporter) OnComplete(taskID string, r Result) {
	if l.logger == nil {
		return
	}
	if r.Downloaded > 0 || r.Failed > 0 {
		l.logger("[%s] Completed: %d downloaded, %d failed, %d skipped", taskID, r.Downloaded, r.Failed, r.Skipped)
	} else {
		l.logger("[%s] Completed: %s", taskID, r.Message)
	}
}

func (l *LogReporter) OnError(taskID string, err error) {
	if l.logger == nil {
		return
	}
	l.logger("[%s] Error: %v", taskID, err)
}
