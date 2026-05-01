package utils

import "strings"

// UserIDExtractor 用于从用户结构体提取 ID 的泛型函数
type UserIDExtractor[T any] func(T) uint64

// ExtractIDs 通用 ID 提取函数
// 传入用户列表和提取函数，返回 ID 列表
func ExtractIDs[T any](users []T, extract UserIDExtractor[T]) []uint64 {
	if len(users) == 0 {
		return nil
	}
	uids := make([]uint64, len(users))
	for i, u := range users {
		uids[i] = extract(u)
	}
	return uids
}

// NormalizeScreenName 统一剥离 screen name 的 @ 前缀。
func NormalizeScreenName(screenName string) string {
	return strings.TrimPrefix(screenName, "@")
}

// IsValidScreenName 校验 Twitter screen name 格式。
// 规则：1-15 个字符，只允许字母、数字、下划线。
func IsValidScreenName(screenName string) bool {
	if len(screenName) < 1 || len(screenName) > 15 {
		return false
	}
	for _, ch := range screenName {
		if !((ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '_') {
			return false
		}
	}
	return true
}
