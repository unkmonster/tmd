package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/unkmonster/tmd/internal/database"

	log "github.com/sirupsen/logrus"
)

// ============ 排序字段白名单 ============

var userSortFields = map[string]string{
	"id":            "id",
	"screen_name":   "screen_name",
	"name":          "name",
	"friends_count": "friends_count",
	"is_accessible": "is_accessible",
}

var listSortFields = map[string]string{
	"id":       "id",
	"name":     "name",
	"owner_id": "owner_user_id",
}

var entitySortFields = map[string]string{
	"id":                  "id",
	"user_id":             "user_id",
	"name":                "name",
	"media_count":         "media_count",
	"latest_release_time": "latest_release_time",
}

var lstEntitySortFields = map[string]string{
	"id":     "id",
	"lst_id": "lst_id",
	"name":   "name",
}

var linkSortFields = map[string]string{
	"id":                   "id",
	"user_id":              "user_id",
	"name":                 "name",
	"parent_lst_entity_id": "parent_lst_entity_id",
}

var prevNameSortFields = map[string]string{
	"id":                  "pn.id",
	"user_id":             "pn.user_id",
	"screen_name":         "pn.screen_name",
	"name":                "pn.name",
	"record_date":         "pn.record_date",
	"current_screen_name": "u.screen_name",
}

func optionalUint64Query(r *http.Request, name string) (uint64, bool, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, false, nil
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("invalid %s", name)
	}
	return value, true, nil
}

// ============ Users 管理 ============

func (s *Server) handleDBUsers(w http.ResponseWriter, r *http.Request) {
	pagination := NewPagination(r)

	var whereConditions []string
	var args []interface{}

	// 搜索关键词
	if keyword := r.URL.Query().Get("q"); keyword != "" {
		cond, searchArgs := database.BuildSearchCondition(
			[]string{"screen_name", "name"},
			keyword,
		)
		whereConditions = append(whereConditions, cond)
		args = append(args, searchArgs...)
	}

	// 可访问状态筛选
	if accessible := r.URL.Query().Get("accessible"); accessible != "" {
		whereConditions = append(whereConditions, "is_accessible = ?")
		args = append(args, accessible == "true")
	}

	// 保护状态筛选
	if protected := r.URL.Query().Get("protected"); protected != "" {
		whereConditions = append(whereConditions, "protected = ?")
		args = append(args, protected == "true")
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = strings.Join(whereConditions, " AND ")
	}

	// 获取总数
	total, err := database.Count(s.db, "users", &database.QueryOptions{
		Where: whereClause,
		Args:  args,
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 查询数据
	orderBy := pagination.BuildOrderBy(userSortFields)
	users, err := database.QueryUsers(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]DBUserItem, len(users))
	for i, u := range users {
		items[i] = DBUserItem{
			ID:           strconv.FormatUint(u.Id, 10),
			ScreenName:   u.ScreenName,
			Name:         u.Name,
			IsProtected:  u.IsProtected,
			FriendsCount: u.FriendsCount,
			IsAccessible: u.IsAccessible,
		}
	}

	response := pagination.ToResponse(items, total)
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(response))
}

func (s *Server) handleDBUserDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	user, err := database.GetUserById(s.db, id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if user == nil {
		s.writeError(w, http.StatusNotFound, "User not found")
		return
	}

	item := DBUserItem{
		ID:           strconv.FormatUint(user.Id, 10),
		ScreenName:   user.ScreenName,
		Name:         user.Name,
		IsProtected:  user.IsProtected,
		FriendsCount: user.FriendsCount,
		IsAccessible: user.IsAccessible,
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(item))
}

func (s *Server) handleDBUserUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req struct {
		ScreenName   *string `json:"screen_name,omitempty"`
		Name         *string `json:"name,omitempty"`
		FriendsCount *int    `json:"friends_count,omitempty"`
		IsProtected  *bool   `json:"protected,omitempty"`
		IsAccessible *bool   `json:"is_accessible,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := database.GetUserById(s.db, id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if user == nil {
		s.writeError(w, http.StatusNotFound, "User not found")
		return
	}

	if req.ScreenName != nil {
		user.ScreenName = *req.ScreenName
	}
	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.FriendsCount != nil {
		user.FriendsCount = *req.FriendsCount
	}
	if req.IsProtected != nil {
		user.IsProtected = *req.IsProtected
	}
	if req.IsAccessible != nil {
		user.IsAccessible = *req.IsAccessible
	}

	if err := database.UpdateUser(s.db, user); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	item := DBUserItem{
		ID:           strconv.FormatUint(user.Id, 10),
		ScreenName:   user.ScreenName,
		Name:         user.Name,
		IsProtected:  user.IsProtected,
		FriendsCount: user.FriendsCount,
		IsAccessible: user.IsAccessible,
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(item))
}

func (s *Server) handleDBUserDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	user, err := database.GetUserById(s.db, id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if user == nil {
		s.writeError(w, http.StatusNotFound, "User not found")
		return
	}

	// 统计级联删除的记录数
	var linkCount, entityCount, nameCount int
	if err := s.db.Get(&linkCount, "SELECT COUNT(*) FROM user_links WHERE user_id = ?", id); err != nil {
		log.Warnf("failed to count user_links for user %d: %v", id, err)
	}
	if err := s.db.Get(&entityCount, "SELECT COUNT(*) FROM user_entities WHERE user_id = ?", id); err != nil {
		log.Warnf("failed to count user_entities for user %d: %v", id, err)
	}
	if err := s.db.Get(&nameCount, "SELECT COUNT(*) FROM user_previous_names WHERE user_id = ?", id); err != nil {
		log.Warnf("failed to count user_previous_names for user %d: %v", id, err)
	}

	if err := database.DelUser(s.db, id); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message":                "User deleted successfully",
		"cascade_links":          linkCount,
		"cascade_entities":       entityCount,
		"cascade_previous_names": nameCount,
	}))
}

// ============ Lists 管理 ============

func (s *Server) handleDBLists(w http.ResponseWriter, r *http.Request) {
	pagination := NewPagination(r)

	var whereConditions []string
	var args []interface{}

	if keyword := r.URL.Query().Get("q"); keyword != "" {
		cond, searchArgs := database.BuildSearchCondition([]string{"name"}, keyword)
		whereConditions = append(whereConditions, cond)
		args = append(args, searchArgs...)
	}

	if ownerID := r.URL.Query().Get("ownerId"); ownerID != "" {
		ownerUID, err := strconv.ParseUint(ownerID, 10, 64)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "Invalid owner ID")
			return
		}
		whereConditions = append(whereConditions, "owner_user_id = ?")
		args = append(args, ownerUID)
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = strings.Join(whereConditions, " AND ")
	}

	total, err := database.Count(s.db, "lsts", &database.QueryOptions{
		Where: whereClause,
		Args:  args,
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	orderBy := pagination.BuildOrderBy(listSortFields)
	lists, err := database.QueryLists(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]DBListItem, len(lists))
	for i, l := range lists {
		items[i] = DBListItem{
			ID:      strconv.FormatUint(l.Id, 10),
			Name:    l.Name,
			OwnerID: strconv.FormatUint(l.OwnerUserId, 10),
		}
	}

	response := pagination.ToResponse(items, total)
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(response))
}

func (s *Server) handleDBListDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid list ID")
		return
	}

	lst, err := database.GetLst(s.db, id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if lst == nil {
		s.writeError(w, http.StatusNotFound, "List not found")
		return
	}

	item := DBListItem{
		ID:      strconv.FormatUint(lst.Id, 10),
		Name:    lst.Name,
		OwnerID: strconv.FormatUint(lst.OwnerUserId, 10),
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(item))
}

func (s *Server) handleDBListUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid list ID")
		return
	}

	var req struct {
		Name    *string `json:"name,omitempty"`
		OwnerID *string `json:"owner_user_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	lst, err := database.GetLst(s.db, id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if lst == nil {
		s.writeError(w, http.StatusNotFound, "List not found")
		return
	}

	if req.Name != nil {
		lst.Name = *req.Name
	}
	if req.OwnerID != nil {
		ownerID, err := strconv.ParseUint(*req.OwnerID, 10, 64)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "Invalid owner ID")
			return
		}
		lst.OwnerUserId = ownerID
	}

	if err := database.UpdateLst(s.db, lst); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	item := DBListItem{
		ID:      strconv.FormatUint(lst.Id, 10),
		Name:    lst.Name,
		OwnerID: strconv.FormatUint(lst.OwnerUserId, 10),
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(item))
}

func (s *Server) handleDBListDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid list ID")
		return
	}

	lst, err := database.GetLst(s.db, id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if lst == nil {
		s.writeError(w, http.StatusNotFound, "List not found")
		return
	}

	if err := database.DelLst(s.db, id); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]string{
		"message": "List deleted successfully",
	}))
}

// ============ User Entities 管理 ============

func (s *Server) handleDBUserEntities(w http.ResponseWriter, r *http.Request) {
	pagination := NewPagination(r)

	var whereConditions []string
	var args []interface{}

	if keyword := r.URL.Query().Get("q"); keyword != "" {
		cond, searchArgs := database.BuildSearchCondition([]string{"name"}, keyword)
		whereConditions = append(whereConditions, cond)
		args = append(args, searchArgs...)
	}

	if userID, ok, err := optionalUint64Query(r, "userId"); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	} else if ok {
		whereConditions = append(whereConditions, "user_id = ?")
		args = append(args, userID)
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = strings.Join(whereConditions, " AND ")
	}

	total, err := database.Count(s.db, "user_entities", &database.QueryOptions{
		Where: whereClause,
		Args:  args,
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	orderBy := pagination.BuildOrderBy(entitySortFields)
	entities, err := database.QueryUserEntities(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]DBEntityItem, len(entities))
	for i, e := range entities {
		items[i] = DBEntityItem{
			ID:         strconv.FormatInt(int64(nullInt32(e.Id)), 10),
			UserID:     strconv.FormatUint(e.UserId, 10),
			Name:       e.Name,
			ParentDir:  e.ParentDir,
			MediaCount: nullInt32(e.MediaCount),
		}
		if e.LatestReleaseTime.Valid {
			items[i].LatestReleaseTime = e.LatestReleaseTime.Time.Format("2006-01-02 15:04:05")
		}
	}

	response := pagination.ToResponse(items, total)
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(response))
}

func (s *Server) handleDBUserEntityDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid entity ID")
		return
	}

	entity, err := database.GetUserEntity(s.db, int(id))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entity == nil {
		s.writeError(w, http.StatusNotFound, "Entity not found")
		return
	}

	item := DBEntityItem{
		ID:         strconv.FormatInt(int64(nullInt32(entity.Id)), 10),
		UserID:     strconv.FormatUint(entity.UserId, 10),
		Name:       entity.Name,
		ParentDir:  entity.ParentDir,
		MediaCount: nullInt32(entity.MediaCount),
	}
	if entity.LatestReleaseTime.Valid {
		item.LatestReleaseTime = entity.LatestReleaseTime.Time.Format("2006-01-02 15:04:05")
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(item))
}

func (s *Server) handleDBUserEntityUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid entity ID")
		return
	}

	var req struct {
		Name              *string `json:"name"`
		ParentDir         *string `json:"parent_dir"`
		MediaCount        *int32  `json:"media_count,omitempty"`
		LatestReleaseTime *string `json:"latest_release_time,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.ParentDir != nil {
		s.writeError(w, http.StatusBadRequest, "Modifying parent_dir is not allowed")
		return
	}

	entity, err := database.GetUserEntity(s.db, int(id))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entity == nil {
		s.writeError(w, http.StatusNotFound, "Entity not found")
		return
	}

	if req.Name != nil {
		entity.Name = *req.Name
	}
	if req.MediaCount != nil {
		entity.MediaCount = sql.NullInt32{Int32: *req.MediaCount, Valid: true}
	}
	if req.LatestReleaseTime != nil {
		if *req.LatestReleaseTime == "" {
			if err := database.ClearUserEntityLatestReleaseTime(s.db, int(id)); err != nil {
				s.writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		} else {
			t, err := time.Parse(time.RFC3339, *req.LatestReleaseTime)
			if err != nil {
				s.writeError(w, http.StatusBadRequest, "Invalid latest_release_time format, use RFC 3339 (e.g. 2024-12-18T15:30:00Z)")
				return
			}
			if err := database.SetUserEntityLatestReleaseTime(s.db, int(id), t); err != nil {
				s.writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		// 不再重读 entity：UpdateUserEntityFields 只写 name+media_count，
		// 不碰 latest_release_time，重读会覆盖 Name/MediaCount 的在内存修改。
	}

	if err := database.UpdateUserEntityFields(s.db, int(id), entity.Name, entity.MediaCount); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	item := DBEntityItem{
		ID:         strconv.FormatInt(int64(nullInt32(entity.Id)), 10),
		UserID:     strconv.FormatUint(entity.UserId, 10),
		Name:       entity.Name,
		ParentDir:  entity.ParentDir,
		MediaCount: nullInt32(entity.MediaCount),
	}
	if entity.LatestReleaseTime.Valid {
		item.LatestReleaseTime = entity.LatestReleaseTime.Time.Format("2006-01-02 15:04:05")
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(item))
}

func (s *Server) handleDBUserEntityDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid entity ID")
		return
	}

	entity, err := database.GetUserEntity(s.db, int(id))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entity == nil {
		s.writeError(w, http.StatusNotFound, "Entity not found")
		return
	}

	if err := database.DelUserEntity(s.db, uint32(id)); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]string{
		"message": "Entity deleted successfully",
	}))
}

// batchLoadNames 批量加载指定表的 id→name 映射，返回 map[int64]string。
// 用于替代循环内逐条 GetLst/GetLstEntity 查询，消除 N+1 和 nil panic。
func (s *Server) batchLoadNames(table, idCol, nameCol string, ids []interface{}) map[int64]string {
	names := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return names
	}
	q := fmt.Sprintf("SELECT %s, %s FROM %s WHERE %s IN (%s)",
		idCol, nameCol, table, idCol,
		strings.TrimRight(strings.Repeat("?,", len(ids)), ","))
	rows, err := s.db.Query(q, ids...)
	if err != nil {
		return names
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err == nil {
			names[id] = name
		}
	}
	return names
}

// ============ List Entities 管理（新增） ============

func (s *Server) handleDBListEntities(w http.ResponseWriter, r *http.Request) {
	pagination := NewPagination(r)

	var whereConditions []string
	var args []interface{}

	if keyword := r.URL.Query().Get("q"); keyword != "" {
		cond, searchArgs := database.BuildSearchCondition([]string{"name"}, keyword)
		whereConditions = append(whereConditions, cond)
		args = append(args, searchArgs...)
	}

	if listID, ok, err := optionalUint64Query(r, "listId"); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid list ID")
		return
	} else if ok {
		whereConditions = append(whereConditions, "lst_id = ?")
		args = append(args, listID)
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = strings.Join(whereConditions, " AND ")
	}

	total, err := database.Count(s.db, "lst_entities", &database.QueryOptions{
		Where: whereClause,
		Args:  args,
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	orderBy := pagination.BuildOrderBy(lstEntitySortFields)
	entities, err := database.QueryLstEntities(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]DBListEntityItem, len(entities))
	ids := make([]interface{}, 0, len(entities))
	for _, e := range entities {
		ids = append(ids, e.LstId)
	}
	lstNames := s.batchLoadNames("lsts", "id", "name", ids)
	for i, e := range entities {
		items[i] = DBListEntityItem{
			ID:        strconv.FormatInt(int64(nullInt32(e.Id)), 10),
			LstID:     strconv.FormatInt(e.LstId, 10),
			Name:      e.Name,
			ParentDir: e.ParentDir,
			ListName:  lstNames[e.LstId],
		}
	}

	response := pagination.ToResponse(items, total)
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(response))
}

func (s *Server) handleDBListEntityDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid list entity ID")
		return
	}

	entity, err := database.GetLstEntity(s.db, int(id))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entity == nil {
		s.writeError(w, http.StatusNotFound, "List entity not found")
		return
	}

	item := DBListEntityItem{
		ID:        strconv.FormatInt(int64(nullInt32(entity.Id)), 10),
		LstID:     strconv.FormatInt(entity.LstId, 10),
		Name:      entity.Name,
		ParentDir: entity.ParentDir,
	}
	if lst, err := database.GetLst(s.db, uint64(entity.LstId)); err == nil && lst != nil {
		item.ListName = lst.Name
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(item))
}

func (s *Server) handleDBListEntityUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid list entity ID")
		return
	}

	var req struct {
		Name      *string `json:"name"`
		ParentDir *string `json:"parent_dir"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.ParentDir != nil {
		s.writeError(w, http.StatusBadRequest, "Modifying parent_dir is not allowed")
		return
	}

	entity, err := database.GetLstEntity(s.db, int(id))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entity == nil {
		s.writeError(w, http.StatusNotFound, "List entity not found")
		return
	}

	if req.Name != nil {
		entity.Name = *req.Name
	}

	if err := database.UpdateLstEntity(s.db, entity); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	item := DBListEntityItem{
		ID:        strconv.FormatInt(int64(nullInt32(entity.Id)), 10),
		LstID:     strconv.FormatInt(entity.LstId, 10),
		Name:      entity.Name,
		ParentDir: entity.ParentDir,
	}
	if lst, err := database.GetLst(s.db, uint64(entity.LstId)); err == nil && lst != nil {
		item.ListName = lst.Name
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(item))
}

func (s *Server) handleDBListEntityDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid list entity ID")
		return
	}

	entity, err := database.GetLstEntity(s.db, int(id))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entity == nil {
		s.writeError(w, http.StatusNotFound, "List entity not found")
		return
	}

	if err := database.DelLstEntity(s.db, int(id)); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]string{
		"message": "List entity deleted successfully",
	}))
}

// ============ User Links 管理（新增） ============

func (s *Server) handleDBUserLinks(w http.ResponseWriter, r *http.Request) {
	pagination := NewPagination(r)

	var whereConditions []string
	var args []interface{}

	if userID, ok, err := optionalUint64Query(r, "userId"); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	} else if ok {
		whereConditions = append(whereConditions, "user_id = ?")
		args = append(args, userID)
	}

	if listEntityID, ok, err := optionalUint64Query(r, "listEntityId"); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid list entity ID")
		return
	} else if ok {
		whereConditions = append(whereConditions, "parent_lst_entity_id = ?")
		args = append(args, listEntityID)
	}

	// 搜索关键词
	if keyword := r.URL.Query().Get("q"); keyword != "" {
		cond, searchArgs := database.BuildSearchCondition(
			[]string{"name"},
			keyword,
		)
		whereConditions = append(whereConditions, cond)
		args = append(args, searchArgs...)
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = strings.Join(whereConditions, " AND ")
	}

	total, err := database.Count(s.db, "user_links", &database.QueryOptions{
		Where: whereClause,
		Args:  args,
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	orderBy := pagination.BuildOrderBy(linkSortFields)
	links, err := database.QueryUserLinks(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]DBUserLinkItem, len(links))
	ids := make([]interface{}, 0, len(links))
	for _, l := range links {
		ids = append(ids, int64(l.ParentLstEntityId))
	}
	entNames := s.batchLoadNames("lst_entities", "id", "name", ids)
	for i, l := range links {
		items[i] = DBUserLinkItem{
			ID:                strconv.Itoa(int(l.Id)),
			UserID:            strconv.FormatUint(l.UserId, 10),
			Name:              l.Name,
			ParentLstEntityID: strconv.Itoa(int(l.ParentLstEntityId)),
			ParentLstEntityName: entNames[int64(l.ParentLstEntityId)],
		}
	}

	response := pagination.ToResponse(items, total)
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(response))
}

func (s *Server) handleDBUserLinkDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid user link ID")
		return
	}

	link, err := database.GetUserLinkById(s.db, int32(id))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if link == nil {
		s.writeError(w, http.StatusNotFound, "User link not found")
		return
	}

	item := DBUserLinkItem{
		ID:                strconv.Itoa(int(link.Id)),
		UserID:            strconv.FormatUint(link.UserId, 10),
		Name:              link.Name,
		ParentLstEntityID: strconv.Itoa(int(link.ParentLstEntityId)),
	}
	if lstEnt, err := database.GetLstEntity(s.db, int(link.ParentLstEntityId)); err == nil && lstEnt != nil {
		item.ParentLstEntityName = lstEnt.Name
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(item))
}

func (s *Server) handleDBUserLinkUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid user link ID")
		return
	}

	var req struct {
		Name *string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	link, err := database.GetUserLinkById(s.db, int32(id))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if link == nil {
		s.writeError(w, http.StatusNotFound, "User link not found")
		return
	}

	if req.Name != nil {
		link.Name = *req.Name
	}

	if err := database.UpdateUserLink(s.db, link.Id, link.Name); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	item := DBUserLinkItem{
		ID:                strconv.Itoa(int(link.Id)),
		UserID:            strconv.FormatUint(link.UserId, 10),
		Name:              link.Name,
		ParentLstEntityID: strconv.Itoa(int(link.ParentLstEntityId)),
	}
	if lstEnt, err := database.GetLstEntity(s.db, int(link.ParentLstEntityId)); err == nil && lstEnt != nil {
		item.ParentLstEntityName = lstEnt.Name
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(item))
}

func (s *Server) handleDBUserLinkDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid user link ID")
		return
	}

	link, err := database.GetUserLinkById(s.db, int32(id))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if link == nil {
		s.writeError(w, http.StatusNotFound, "User link not found")
		return
	}

	if err := database.DelUserLink(s.db, int32(id)); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]string{
		"message": "User link deleted successfully",
	}))
}

// ============ User Previous Names 查询（新增） ============

func (s *Server) handleDBUserPreviousNames(w http.ResponseWriter, r *http.Request) {
	uid, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	pagination := NewPagination(r)

	whereClause := "pn.user_id = ?"
	args := []interface{}{uid}

	total, err := database.Count(s.db, "user_previous_names pn LEFT JOIN users u ON pn.user_id = u.id", &database.QueryOptions{
		Where: whereClause,
		Args:  args,
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	orderBy := pagination.BuildOrderBy(prevNameSortFields)
	names, err := database.QueryAllUserPreviousNames(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]DBUserPreviousNameItem, len(names))
	for i, n := range names {
		items[i] = DBUserPreviousNameItem{
			ID:                strconv.Itoa(int(n.Id)),
			UserID:            strconv.FormatUint(n.UserId, 10),
			ScreenName:        n.ScreenName,
			Name:              n.Name,
			RecordDate:        n.RecordDate.Format("2006-01-02"),
			CurrentScreenName: n.CurrentScreenName,
			CurrentName:       n.CurrentName,
		}
	}

	response := pagination.ToResponse(items, total)
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(response))
}

// ============ 子资源快捷端点 ============

// handleDBUserEntitiesByUserID 获取指定用户的所有实体
func (s *Server) handleDBUserEntitiesByUserID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	pagination := NewPagination(r)
	whereClause := "user_id = ?"
	args := []interface{}{id}

	total, err := database.Count(s.db, "user_entities", &database.QueryOptions{
		Where: whereClause,
		Args:  args,
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	orderBy := pagination.BuildOrderBy(entitySortFields)
	entities, err := database.QueryUserEntities(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]DBEntityItem, len(entities))
	for i, e := range entities {
		items[i] = DBEntityItem{
			ID:        strconv.FormatInt(int64(nullInt32(e.Id)), 10),
			UserID:    strconv.FormatUint(e.UserId, 10),
			Name:      e.Name,
			ParentDir: e.ParentDir,
		}
	}
	resp := pagination.ToResponse(items, total)
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(resp))
}

// handleDBUserLinksByUserID 获取指定用户的所有链接
func (s *Server) handleDBUserLinksByUserID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	pagination := NewPagination(r)
	whereClause := "user_id = ?"
	args := []interface{}{id}

	total, err := database.Count(s.db, "user_links", &database.QueryOptions{
		Where: whereClause,
		Args:  args,
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	orderBy := pagination.BuildOrderBy(linkSortFields)
	links, err := database.QueryUserLinks(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]DBUserLinkItem, len(links))
	ids := make([]interface{}, 0, len(links))
	for _, l := range links {
		ids = append(ids, int64(l.ParentLstEntityId))
	}
	entNames := s.batchLoadNames("lst_entities", "id", "name", ids)
	for i, l := range links {
		items[i] = DBUserLinkItem{
			ID:                strconv.Itoa(int(l.Id)),
			UserID:            strconv.FormatUint(l.UserId, 10),
			Name:              l.Name,
			ParentLstEntityID: strconv.Itoa(int(l.ParentLstEntityId)),
			ParentLstEntityName: entNames[int64(l.ParentLstEntityId)],
		}
	}
	resp := pagination.ToResponse(items, total)
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(resp))
}

// handleDBLstEntitiesByListID 获取指定列表的所有实体
func (s *Server) handleDBLstEntitiesByListID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid list ID")
		return
	}

	pagination := NewPagination(r)
	whereClause := "lst_id = ?"
	args := []interface{}{id}

	total, err := database.Count(s.db, "lst_entities", &database.QueryOptions{
		Where: whereClause,
		Args:  args,
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	orderBy := pagination.BuildOrderBy(lstEntitySortFields)
	entities, err := database.QueryLstEntities(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]DBListEntityItem, len(entities))
	ids := make([]interface{}, 0, len(entities))
	for _, e := range entities {
		ids = append(ids, e.LstId)
	}
	lstNames := s.batchLoadNames("lsts", "id", "name", ids)
	for i, e := range entities {
		items[i] = DBListEntityItem{
			ID:        strconv.FormatInt(int64(nullInt32(e.Id)), 10),
			LstID:     strconv.FormatUint(uint64(e.LstId), 10),
			Name:      e.Name,
			ParentDir: e.ParentDir,
			ListName:  lstNames[e.LstId],
		}
	}
	resp := pagination.ToResponse(items, total)
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(resp))
}

// handleDBStats 返回各表记录数
func (s *Server) handleDBStats(w http.ResponseWriter, _ *http.Request) {
	tables := []struct {
		Name  string
		Field string `json:"field"`
	}{
		{"users", "users"},
		{"lsts", "lists"},
		{"user_entities", "user_entities"},
		{"lst_entities", "lst_entities"},
		{"user_links", "user_links"},
		{"user_previous_names", "user_previous_names"},
	}

	stats := make(map[string]int, len(tables))
	for _, t := range tables {
		count, err := database.Count(s.db, t.Name, nil)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "Failed to count "+t.Name+": "+err.Error())
			return
		}
		stats[t.Name] = count
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(stats))
}

// ============ Previous Names 全局列表 ============

func (s *Server) handleDBPreviousNames(w http.ResponseWriter, r *http.Request) {
	pagination := NewPagination(r)

	var whereConditions []string
	var args []interface{}

	// 可选：按 user_id 筛选
	if userID, ok, err := optionalUint64Query(r, "userId"); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	} else if ok {
		whereConditions = append(whereConditions, "pn.user_id = ?")
		args = append(args, userID)
	}

	// 搜索关键词（pn.screen_name、pn.name、u.screen_name）
	if keyword := r.URL.Query().Get("q"); keyword != "" {
		cond, searchArgs := database.BuildSearchCondition(
			[]string{"pn.screen_name", "pn.name", "u.screen_name"},
			keyword,
		)
		whereConditions = append(whereConditions, cond)
		args = append(args, searchArgs...)
	}

	// 基础条件：只统计有多条记录的用户（实际改过名的）
	baseWhere := "pn.user_id IN (SELECT user_id FROM user_previous_names GROUP BY user_id HAVING COUNT(*) > 1)"
	var whereClause string
	if len(whereConditions) > 0 {
		whereClause = baseWhere + " AND " + strings.Join(whereConditions, " AND ")
	} else {
		whereClause = baseWhere
	}

	total, err := database.Count(s.db, "user_previous_names pn LEFT JOIN users u ON pn.user_id = u.id", &database.QueryOptions{
		Where: whereClause,
		Args:  args,
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	orderBy := pagination.BuildOrderBy(prevNameSortFields)
	names, err := database.QueryAllUserPreviousNames(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]DBUserPreviousNameItem, len(names))
	for i, n := range names {
		items[i] = DBUserPreviousNameItem{
			ID:                strconv.Itoa(int(n.Id)),
			UserID:            strconv.FormatUint(n.UserId, 10),
			ScreenName:        n.ScreenName,
			Name:              n.Name,
			RecordDate:        n.RecordDate.Format("2006-01-02"),
			CurrentScreenName: n.CurrentScreenName,
			CurrentName:       n.CurrentName,
		}
	}

	response := pagination.ToResponse(items, total)
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(response))
}

// ============ 辅助函数 ============

func nullInt32(n sql.NullInt32) int32 {
	if n.Valid {
		return n.Int32
	}
	return 0
}
