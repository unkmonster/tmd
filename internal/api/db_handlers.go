package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/unkmonster/tmd/internal/database"
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
	"owner_id": "owner_uid",
}

var entitySortFields = map[string]string{
	"id":                  "id",
	"user_id":             "user_id",
	"name":                "name",
	"media_count":         "media_count",
	"latest_release_time": "latest_release_time",
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
		ScreenName   string `json:"screen_name"`
		Name         string `json:"name"`
		FriendsCount *int   `json:"friends_count,omitempty"`
		IsProtected  *bool  `json:"protected,omitempty"`
		IsAccessible *bool  `json:"is_accessible,omitempty"`
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

	if req.ScreenName != "" {
		user.ScreenName = req.ScreenName
	}
	if req.Name != "" {
		user.Name = req.Name
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

	if err := database.DelUser(s.db, id); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]string{
		"message": "User deleted successfully",
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
		whereConditions = append(whereConditions, "owner_uid = ?")
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
			OwnerID: strconv.FormatUint(l.OwnerId, 10),
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
		OwnerID: strconv.FormatUint(lst.OwnerId, 10),
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
		Name    string `json:"name"`
		OwnerID string `json:"owner_uid"`
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

	if req.Name != "" {
		lst.Name = req.Name
	}
	if req.OwnerID != "" {
		ownerID, err := strconv.ParseUint(req.OwnerID, 10, 64)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "Invalid owner ID")
			return
		}
		lst.OwnerId = ownerID
	}

	if err := database.UpdateLst(s.db, lst); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	item := DBListItem{
		ID:      strconv.FormatUint(lst.Id, 10),
		Name:    lst.Name,
		OwnerID: strconv.FormatUint(lst.OwnerId, 10),
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

	if userID := r.URL.Query().Get("userId"); userID != "" {
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
			UserID:     strconv.FormatUint(e.Uid, 10),
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
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid entity ID")
		return
	}

	entity, err := database.GetUserEntity(s.db, id)
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
		UserID:     strconv.FormatUint(entity.Uid, 10),
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
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid entity ID")
		return
	}

	var req struct {
		Name       *string `json:"name"`
		ParentDir  *string `json:"parent_dir"`
		MediaCount *int32  `json:"media_count,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.ParentDir != nil {
		s.writeError(w, http.StatusBadRequest, "Modifying parent_dir is not allowed")
		return
	}

	entity, err := database.GetUserEntity(s.db, id)
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

	if err := database.UpdateUserEntity(s.db, entity); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	item := DBEntityItem{
		ID:         strconv.FormatInt(int64(nullInt32(entity.Id)), 10),
		UserID:     strconv.FormatUint(entity.Uid, 10),
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
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid entity ID")
		return
	}

	entity, err := database.GetUserEntity(s.db, id)
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

	if listID := r.URL.Query().Get("listId"); listID != "" {
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

	orderBy := pagination.BuildOrderBy(entitySortFields)
	entities, err := database.QueryLstEntities(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]DBListEntityItem, len(entities))
	for i, e := range entities {
		items[i] = DBListEntityItem{
			ID:        strconv.FormatInt(int64(nullInt32(e.Id)), 10),
			LstID:     strconv.FormatInt(e.LstId, 10),
			Name:      e.Name,
			ParentDir: e.ParentDir,
		}
	}

	response := pagination.ToResponse(items, total)
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(response))
}

func (s *Server) handleDBListEntityDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid list entity ID")
		return
	}

	entity, err := database.GetLstEntity(s.db, id)
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

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(item))
}

func (s *Server) handleDBListEntityUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
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

	entity, err := database.GetLstEntity(s.db, id)
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

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(item))
}

func (s *Server) handleDBListEntityDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid list entity ID")
		return
	}

	entity, err := database.GetLstEntity(s.db, id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entity == nil {
		s.writeError(w, http.StatusNotFound, "List entity not found")
		return
	}

	if err := database.DelLstEntity(s.db, id); err != nil {
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

	if userID := r.URL.Query().Get("userId"); userID != "" {
		whereConditions = append(whereConditions, "user_id = ?")
		args = append(args, userID)
	}

	if listEntityID := r.URL.Query().Get("listEntityId"); listEntityID != "" {
		whereConditions = append(whereConditions, "parent_lst_entity_id = ?")
		args = append(args, listEntityID)
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

	links, err := database.QueryUserLinks(s.db, whereClause, args, pagination.PageSize, pagination.Offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]DBUserLinkItem, len(links))
	for i, l := range links {
		items[i] = DBUserLinkItem{
			ID:                strconv.Itoa(int(l.Id)),
			UserID:            strconv.FormatUint(l.UserId, 10),
			Name:              l.Name,
			ParentLstEntityID: strconv.Itoa(int(l.ParentLstEntityId)),
		}
	}

	response := pagination.ToResponse(items, total)
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(response))
}

func (s *Server) handleDBUserLinkDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
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

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(item))
}

func (s *Server) handleDBUserLinkUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
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

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(item))
}

func (s *Server) handleDBUserLinkDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
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

	names, err := database.QueryUserPreviousNames(s.db, uid, pagination.PageSize, pagination.Offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 获取总数
	total, err := database.Count(s.db, "user_previous_names", &database.QueryOptions{
		Where: "uid = ?",
		Args:  []interface{}{uid},
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]DBUserPreviousNameItem, len(names))
	for i, n := range names {
		items[i] = DBUserPreviousNameItem{
			ID:         strconv.Itoa(int(n.Id)),
			Uid:        strconv.FormatUint(n.Uid, 10),
			ScreenName: n.ScreenName,
			Name:       n.Name,
			RecordDate: n.RecordDate.Format("2006-01-02"),
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
