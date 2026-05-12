package api

import (
	"strings"

	"github.com/unkmonster/tmd/internal/utils"
)

type targetKey struct {
	scope string
	value string
}

const (
	targetScopeUser = "user"
	targetWildcard  = "*"
)

var wildcardUserTarget = targetKey{scope: targetScopeUser, value: targetWildcard}

func normalizeTargetScreenName(raw string) (string, bool) {
	name := utils.NormalizeScreenName(strings.TrimSpace(raw))
	if !utils.IsValidScreenName(name) {
		return "", false
	}
	return name, true
}

func dedupeTargetKeys(keys []targetKey) []targetKey {
	if len(keys) == 0 {
		return nil
	}
	seen := make(map[targetKey]struct{}, len(keys))
	out := make([]targetKey, 0, len(keys))
	for _, key := range keys {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func taskTargetKeys(task *Task) []targetKey {
	if task == nil {
		return []targetKey{wildcardUserTarget}
	}

	switch task.Type {
	case TaskTypeUserDownload, TaskTypeProfileDownload:
		switch data := task.Data.(type) {
		case *UserDownloadTaskData:
			if name, ok := normalizeTargetScreenName(data.ScreenName); ok {
				return []targetKey{{scope: targetScopeUser, value: name}}
			}
		case *ProfileDownloadTaskData:
			if name, ok := normalizeTargetScreenName(data.ScreenName); ok {
				return []targetKey{{scope: targetScopeUser, value: name}}
			}
		}
		return []targetKey{wildcardUserTarget}

	case TaskTypeMarkDownloaded:
		switch data := task.Data.(type) {
		case *MarkDownloadedTaskData:
			if name, ok := normalizeTargetScreenName(data.ScreenName); ok {
				return []targetKey{{scope: targetScopeUser, value: name}}
			}
		case *FollowingMarkDownloadedTaskData:
			return []targetKey{wildcardUserTarget}
		case *ListMarkDownloadedTaskData:
			return []targetKey{wildcardUserTarget}
		}
		return []targetKey{wildcardUserTarget}

	case TaskTypeFollowingDownload, TaskTypeListDownload, TaskTypeListProfile, TaskTypeJsonFileDownload, TaskTypeJsonFolderDownload:
		return []targetKey{wildcardUserTarget}

	case TaskTypeBatchDownload:
		data, ok := task.Data.(*BatchDownloadTaskData)
		if !ok || data == nil {
			return []targetKey{wildcardUserTarget}
		}
		if len(data.Lists) > 0 || len(data.FollowingNames) > 0 {
			return []targetKey{wildcardUserTarget}
		}

		keys := make([]targetKey, 0, len(data.Users))
		for _, raw := range data.Users {
			name, ok := normalizeTargetScreenName(raw)
			if !ok {
				return []targetKey{wildcardUserTarget}
			}
			keys = append(keys, targetKey{scope: targetScopeUser, value: name})
		}
		keys = dedupeTargetKeys(keys)
		if len(keys) == 0 {
			return []targetKey{wildcardUserTarget}
		}
		return keys

	default:
		return []targetKey{wildcardUserTarget}
	}
}

func targetsConflict(a, b targetKey) bool {
	if a.scope != b.scope {
		return false
	}
	return a.value == targetWildcard || b.value == targetWildcard || a.value == b.value
}
