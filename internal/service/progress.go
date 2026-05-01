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
	Versioned  int // 版本化（旧文件已备份到 .versions）
	Message    string
}

// ProgressReporter 进度报告接口
//
// OnError 仅用于最终任务状态上报。
// 对于 service 层的 fatal error，应直接返回 error，由外层编排代码统一决定是否调用 OnError/SetTaskError。
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
	if p.Stage == "downloading" {
		return
	}
	switch p.Stage {
	case "syncing":
		l.logger("[%s] Syncing: %s", taskID, p.Current)
	case "retrying":
		if p.Total > 0 {
			l.logger("[%s] Retrying failed tweets (%d/%d, remaining=%d)", taskID, p.Completed, p.Total, p.Failed)
		} else {
			l.logger("[%s] Retrying failed tweets...", taskID)
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
	if r.Downloaded != 0 || r.Failed != 0 || r.Versioned != 0 {
		l.logger(
			"[%s] Completed (downloaded=%d, failed=%d, versioned=%d)",
			taskID,
			r.Downloaded,
			r.Failed,
			r.Versioned,
		)
		return
	}
	l.logger("[%s] Completed: %s", taskID, r.Message)
}

func (l *LogReporter) OnError(taskID string, err error) {
	if l.logger == nil {
		return
	}
	l.logger("[%s] Error: %v", taskID, err)
}
