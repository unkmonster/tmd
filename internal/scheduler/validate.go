package scheduler

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/unkmonster/tmd/internal/utils"
)

// ScheduleIDPattern 用于校验手动指定的 schedule ID 格式。
var ScheduleIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

const maxDailyTimes = 96

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
