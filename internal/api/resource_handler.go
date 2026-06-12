package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/unkmonster/tmd/internal/database"
)

// ============ ID 路径参数解析 ============

// resolvePathID 从路径参数中解析 uint64 ID，解析失败时自动写入 400 响应。
// 返回 true 表示成功，false 表示已写入错误响应。
// 注意：返回 uint64，调用 GetUserEntity(id int)/GetUserLinkById(id int32) 等函数时需显式转换。
func (s *Server) resolvePathID(w http.ResponseWriter, r *http.Request, paramName, resourceName string) (uint64, bool) {
	id, err := strconv.ParseUint(r.PathValue(paramName), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid "+resourceName+" ID")
		return 0, false
	}
	return id, true
}

// ============ JSON 请求体解析 ============

// decodeBody 解析 JSON 请求体，解析失败时自动写入 400 响应。
// 与 download_handlers.go 中已有的 decodeOptionalJSON 不同，此函数要求非空请求体。
func (s *Server) decodeBody(w http.ResponseWriter, r *http.Request, dest interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body")
		return false
	}
	return true
}

// ============ 泛型资源存在性检查 ============

// requireResource 检查资源是否找到，数据库错误或不存在时写入错误响应。
// 由于 Go 不支持方法级别的泛型参数，此函数为独立包级函数，
// 通过 writeError 回调将错误写入响应。
// writeError 需要绑定 http.ResponseWriter，推荐调用方式：
//
//	if !requireResource(w, user, err, "User", func(c int, m string) { s.writeError(w, c, m) }) {
//	    return
//	}
func requireResource[T any](resource *T, err error, resourceName string, writeError func(int, string)) bool {
	if err != nil {
		writeError(http.StatusInternalServerError, err.Error())
		return false
	}
	if resource == nil {
		writeError(http.StatusNotFound, resourceName+" not found")
		return false
	}
	return true
}

// ============ 统一响应写入 ============

// writeResourceJSON 写入资源操作成功响应（200 + 资源数据）
func (s *Server) writeResourceJSON(w http.ResponseWriter, item interface{}) {
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(item))
}

// writeResourceDeleted 写入删除成功响应
func (s *Server) writeResourceDeleted(w http.ResponseWriter, resourceName string) {
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]string{
		"message": resourceName + " deleted successfully",
	}))
}

// ============ 安全 COUNT 查询 ============

// countWithError 执行 COUNT 查询，失败时自动写入 500 并返回是否成功。
// table 参数支持表表达式，如 "users" 或 "user_previous_names pn LEFT JOIN users u"。
func (s *Server) countWithError(w http.ResponseWriter, table string, where string, args []interface{}) (int, bool) {
	total, err := database.Count(s.db, table, &database.QueryOptions{Where: where, Args: args})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return 0, false
	}
	return total, true
}

// ============ Entity → Item 转换函数 ============

func dbUserToItem(u *database.User) DBUserItem {
	return DBUserItem{
		ID:           strconv.FormatUint(u.Id, 10),
		ScreenName:   u.ScreenName,
		Name:         u.Name,
		IsProtected:  u.IsProtected,
		FriendsCount: u.FriendsCount,
		IsAccessible: u.IsAccessible,
	}
}

func dbListToItem(l *database.Lst) DBListItem {
	return DBListItem{
		ID:      strconv.FormatUint(l.Id, 10),
		Name:    l.Name,
		OwnerID: strconv.FormatUint(l.OwnerUserId, 10),
	}
}

func dbEntityToItem(e *database.UserEntity) DBEntityItem {
	item := DBEntityItem{
		ID:         strconv.FormatInt(int64(nullInt32(e.Id)), 10),
		UserID:     strconv.FormatUint(e.UserId, 10),
		Name:       e.Name,
		ParentDir:  e.ParentDir,
		MediaCount: nullInt32(e.MediaCount),
	}
	if e.LatestReleaseTime.Valid {
		item.LatestReleaseTime = e.LatestReleaseTime.Time.Format("2006-01-02 15:04:05")
	}
	return item
}

func dbLstEntityToItem(e *database.LstEntity, lstName string) DBListEntityItem {
	return DBListEntityItem{
		ID:        strconv.FormatInt(int64(nullInt32(e.Id)), 10),
		LstID:     strconv.FormatInt(e.LstId, 10),
		Name:      e.Name,
		ParentDir: e.ParentDir,
		ListName:  lstName,
	}
}

func dbUserLinkToItem(l *database.UserLink, lstEntName string) DBUserLinkItem {
	return DBUserLinkItem{
		ID:                  strconv.Itoa(int(l.Id)),
		UserID:              strconv.FormatUint(l.UserId, 10),
		Name:                l.Name,
		ParentLstEntityID:   strconv.Itoa(int(l.ParentLstEntityId)),
		ParentLstEntityName: lstEntName,
	}
}

func dbPrevNameToItem(n *database.UserPreviousNameWithCurrent) DBUserPreviousNameItem {
	return DBUserPreviousNameItem{
		ID:                strconv.Itoa(int(n.Id)),
		UserID:            strconv.FormatUint(n.UserId, 10),
		ScreenName:        n.ScreenName,
		Name:              n.Name,
		RecordDate:        n.RecordDate.Format("2006-01-02"),
		CurrentScreenName: n.CurrentScreenName,
		CurrentName:       n.CurrentName,
	}
}

// ============ 级联统计辅助 ============

// userCascadeCount 用户删除级联统计
type userCascadeCount struct {
	linkCount, entityCount, nameCount int
}

// countUserCascade 统计删除用户时需要级联删除的记录数
func (s *Server) countUserCascade(id uint64) userCascadeCount {
	var c userCascadeCount
	if err := s.db.Get(&c.linkCount, "SELECT COUNT(*) FROM user_links WHERE user_id = ?", id); err != nil {
		log.Warnf("failed to count user_links for user %d: %v", id, err)
	}
	if err := s.db.Get(&c.entityCount, "SELECT COUNT(*) FROM user_entities WHERE user_id = ?", id); err != nil {
		log.Warnf("failed to count user_entities for user %d: %v", id, err)
	}
	if err := s.db.Get(&c.nameCount, "SELECT COUNT(*) FROM user_previous_names WHERE user_id = ?", id); err != nil {
		log.Warnf("failed to count user_previous_names for user %d: %v", id, err)
	}
	return c
}


