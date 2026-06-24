package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"


	log "github.com/sirupsen/logrus"
	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/utils"
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

	whereClause := strings.Join(whereConditions, " AND ")

	total, ok := s.countWithError(w, "users", whereClause, args)
	if !ok {
		return
	}

	orderBy := pagination.BuildOrderBy(userSortFields)
	users, err := database.QueryUsers(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		log.Errorf("[db] QueryUsers failed: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Database query failed")
		return
	}

	items := make([]DBUserItem, len(users))
	for i, u := range users {
		items[i] = dbUserToItem(&u)
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(pagination.ToResponse(items, total)))
}

func (s *Server) handleDBUserDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := s.resolvePathID(w, r, "id", "user")
	if !ok {
		return
	}

	user, err := database.GetUserById(s.db, id)
	if !requireResource(user, err, "User", func(c int, m string) { s.writeError(w, c, m) }) {
		return
	}

	s.writeResourceJSON(w, dbUserToItem(user))
}

func (s *Server) handleDBUserUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := s.resolvePathID(w, r, "id", "user")
	if !ok {
		return
	}

	var req struct {
		ScreenName   *string `json:"screen_name,omitempty"`
		Name         *string `json:"name,omitempty"`
		FriendsCount *int    `json:"friends_count,omitempty"`
		IsProtected  *bool   `json:"protected,omitempty"`
		IsAccessible *bool   `json:"is_accessible,omitempty"`
	}

	if !s.decodeBody(w, r, &req) {
		return
	}

	user, err := database.GetUserById(s.db, id)
	if !requireResource(user, err, "User", func(c int, m string) { s.writeError(w, c, m) }) {
		return
	}
	if req.ScreenName != nil {
		if err := validateScreenName(*req.ScreenName); err != nil {
			log.Errorf("[db] Invalid screen_name: %v", err)
			s.writeErrorDetail(w, http.StatusBadRequest, "Invalid screen name", err.Error())
			return
		}
		user.ScreenName = *req.ScreenName
	}
	if req.Name != nil {
		if err := validateFieldName(*req.Name); err != nil {
			log.Errorf("[db] Invalid name: %v", err)
			s.writeErrorDetail(w, http.StatusBadRequest, "Invalid name", err.Error())
			return
		}
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
		log.Errorf("[db] UpdateUser failed: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Database query failed")
		return
	}

	s.writeResourceJSON(w, dbUserToItem(user))
}

func (s *Server) handleDBUserDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := s.resolvePathID(w, r, "id", "user")
	if !ok {
		return
	}

	user, err := database.GetUserById(s.db, id)
	if !requireResource(user, err, "User", func(c int, m string) { s.writeError(w, c, m) }) {
		return
	}

	cascade := s.countUserCascade(id)

	if err := database.DelUser(s.db, id); err != nil {
		log.Errorf("[db] DelUser failed: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Database query failed")
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message":                "User deleted successfully",
		"cascade_links":          cascade.linkCount,
		"cascade_entities":       cascade.entityCount,
		"cascade_previous_names": cascade.nameCount,
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

	whereClause := strings.Join(whereConditions, " AND ")

	total, ok := s.countWithError(w, "lsts", whereClause, args)
	if !ok {
		return
	}

	orderBy := pagination.BuildOrderBy(listSortFields)
	lists, err := database.QueryLists(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		log.Errorf("[db] QueryLists failed: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Database query failed")
		return
	}

	items := make([]DBListItem, len(lists))
	for i, l := range lists {
		items[i] = dbListToItem(&l)
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(pagination.ToResponse(items, total)))
}

func (s *Server) handleDBListDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := s.resolvePathID(w, r, "id", "list")
	if !ok {
		return
	}

	lst, err := database.GetLst(s.db, id)
	if !requireResource(lst, err, "List", func(c int, m string) { s.writeError(w, c, m) }) {
		return
	}

	s.writeResourceJSON(w, dbListToItem(lst))
}

func (s *Server) handleDBListUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := s.resolvePathID(w, r, "id", "list")
	if !ok {
		return
	}

	var req struct {
		Name    *string `json:"name,omitempty"`
		OwnerID *string `json:"owner_user_id,omitempty"`
	}

	if !s.decodeBody(w, r, &req) {
		return
	}

	lst, err := database.GetLst(s.db, id)
	if !requireResource(lst, err, "List", func(c int, m string) { s.writeError(w, c, m) }) {
		return
	}

	if req.Name != nil {
		if err := validateFieldName(*req.Name); err != nil {
			log.Errorf("[db] Invalid name: %v", err)
			s.writeErrorDetail(w, http.StatusBadRequest, "Invalid name", err.Error())
			return
		}
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
		log.Errorf("[db] UpdateLst failed: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Database query failed")
		return
	}

	s.writeResourceJSON(w, dbListToItem(lst))
}

func (s *Server) handleDBListDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := s.resolvePathID(w, r, "id", "list")
	if !ok {
		return
	}

	lst, err := database.GetLst(s.db, id)
	if !requireResource(lst, err, "List", func(c int, m string) { s.writeError(w, c, m) }) {
		return
	}

	if err := database.DelLst(s.db, id); err != nil {
		log.Errorf("[db] DelLst failed: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Database query failed")
		return
	}

	s.writeResourceDeleted(w, "List")
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

	whereClause := strings.Join(whereConditions, " AND ")

	total, ok := s.countWithError(w, "user_entities", whereClause, args)
	if !ok {
		return
	}

	orderBy := pagination.BuildOrderBy(entitySortFields)
	entities, err := database.QueryUserEntities(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		log.Errorf("[db] QueryUserEntities failed: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Database query failed")
		return
	}

	items := make([]DBEntityItem, len(entities))
	for i, e := range entities {
		items[i] = dbEntityToItem(&e)
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(pagination.ToResponse(items, total)))
}

func (s *Server) handleDBUserEntityDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := s.resolvePathID(w, r, "id", "entity")
	if !ok {
		return
	}

	entity, err := database.GetUserEntity(s.db, int(id))
	if !requireResource(entity, err, "Entity", func(c int, m string) { s.writeError(w, c, m) }) {
		return
	}

	s.writeResourceJSON(w, dbEntityToItem(entity))
}

func (s *Server) handleDBUserEntityUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := s.resolvePathID(w, r, "id", "entity")
	if !ok {
		return
	}

	var req struct {
		Name              *string `json:"name"`
		ParentDir         *string `json:"parent_dir"`
		MediaCount        *int32  `json:"media_count,omitempty"`
		LatestReleaseTime *string `json:"latest_release_time,omitempty"`
	}

	if !s.decodeBody(w, r, &req) {
		return
	}

	if req.ParentDir != nil {
		s.writeError(w, http.StatusBadRequest, "Modifying parent_dir is not allowed")
		return
	}

	entity, err := database.GetUserEntity(s.db, int(id))
	if !requireResource(entity, err, "Entity", func(c int, m string) { s.writeError(w, c, m) }) {
		return
	}

	if req.Name != nil {
		if err := validateFieldName(*req.Name); err != nil {
			log.Errorf("[db] Invalid name: %v", err)
			s.writeErrorDetail(w, http.StatusBadRequest, "Invalid name", err.Error())
			return
		}
		entity.Name = *req.Name
	}
	if req.MediaCount != nil {
		entity.MediaCount = sql.NullInt32{Int32: *req.MediaCount, Valid: true}
	}
	if req.LatestReleaseTime != nil {
		if *req.LatestReleaseTime == "" {
			if err := database.ClearUserEntityLatestReleaseTime(s.db, int(id)); err != nil {
				log.Errorf("[db] ClearUserEntityLatestReleaseTime failed: %v", err)
				s.writeError(w, http.StatusInternalServerError, "Database query failed")
				return
			}
		} else {
			t, err := time.Parse(time.RFC3339, *req.LatestReleaseTime)
			if err != nil {
				s.writeError(w, http.StatusBadRequest, "Invalid latest_release_time format, use RFC 3339 (e.g. 2024-12-18T15:30:00Z)")
				return
			}
			if err := database.SetUserEntityLatestReleaseTime(s.db, int(id), t); err != nil {
				log.Errorf("[db] SetUserEntityLatestReleaseTime failed: %v", err)
				s.writeError(w, http.StatusInternalServerError, "Database query failed")
				return
			}
		}
	}

	if err := database.UpdateUserEntityFields(s.db, int(id), entity.Name, entity.MediaCount); err != nil {
		log.Errorf("[db] UpdateUserEntityFields failed: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Database query failed")
		return
	}

	s.writeResourceJSON(w, dbEntityToItem(entity))
}

func (s *Server) handleDBUserEntityDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := s.resolvePathID(w, r, "id", "entity")
	if !ok {
		return
	}

	entity, err := database.GetUserEntity(s.db, int(id))
	if !requireResource(entity, err, "Entity", func(c int, m string) { s.writeError(w, c, m) }) {
		return
	}

	if err := database.DelUserEntity(s.db, uint32(id)); err != nil {
		log.Errorf("[db] DelUserEntity failed: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Database query failed")
		return
	}

	s.writeResourceDeleted(w, "Entity")
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

	whereClause := strings.Join(whereConditions, " AND ")

	total, ok := s.countWithError(w, "lst_entities", whereClause, args)
	if !ok {
		return
	}

	orderBy := pagination.BuildOrderBy(lstEntitySortFields)
	entities, err := database.QueryLstEntities(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		log.Errorf("[db] QueryLstEntities failed: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Database query failed")
		return
	}

	items := make([]DBListEntityItem, len(entities))
	ids := make([]interface{}, 0, len(entities))
	for _, e := range entities {
		ids = append(ids, e.LstId)
	}
	lstNames := s.batchLoadNames("lsts", "id", "name", ids)
	for i, e := range entities {
		items[i] = dbLstEntityToItem(&e, lstNames[e.LstId])
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(pagination.ToResponse(items, total)))
}

func (s *Server) handleDBListEntityDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := s.resolvePathID(w, r, "id", "list entity")
	if !ok {
		return
	}

	entity, err := database.GetLstEntity(s.db, int(id))
	if !requireResource(entity, err, "List entity", func(c int, m string) { s.writeError(w, c, m) }) {
		return
	}

	lstName := ""
	if lst, err := database.GetLst(s.db, uint64(entity.LstId)); err == nil && lst != nil {
		lstName = lst.Name
	}

	s.writeResourceJSON(w, dbLstEntityToItem(entity, lstName))
}

func (s *Server) handleDBListEntityUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := s.resolvePathID(w, r, "id", "list entity")
	if !ok {
		return
	}

	var req struct {
		Name      *string `json:"name"`
		ParentDir *string `json:"parent_dir"`
	}

	if !s.decodeBody(w, r, &req) {
		return
	}

	if req.ParentDir != nil {
		s.writeError(w, http.StatusBadRequest, "Modifying parent_dir is not allowed")
		return
	}

	entity, err := database.GetLstEntity(s.db, int(id))
	if !requireResource(entity, err, "List entity", func(c int, m string) { s.writeError(w, c, m) }) {
		return
	}

	if req.Name != nil {
		if err := validateFieldName(*req.Name); err != nil {
			log.Errorf("[db] Invalid name: %v", err)
			s.writeErrorDetail(w, http.StatusBadRequest, "Invalid name", err.Error())
			return
		}
		entity.Name = *req.Name
	}

	if err := database.UpdateLstEntity(s.db, entity); err != nil {
		log.Errorf("[db] UpdateLstEntity failed: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Database query failed")
		return
	}

	lstName := ""
	if lst, err := database.GetLst(s.db, uint64(entity.LstId)); err == nil && lst != nil {
		lstName = lst.Name
	}

	s.writeResourceJSON(w, dbLstEntityToItem(entity, lstName))
}

func (s *Server) handleDBListEntityDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := s.resolvePathID(w, r, "id", "list entity")
	if !ok {
		return
	}

	entity, err := database.GetLstEntity(s.db, int(id))
	if !requireResource(entity, err, "List entity", func(c int, m string) { s.writeError(w, c, m) }) {
		return
	}

	if err := database.DelLstEntity(s.db, int(id)); err != nil {
		log.Errorf("[db] DelLstEntity failed: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Database query failed")
		return
	}

	s.writeResourceDeleted(w, "List entity")
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

	whereClause := strings.Join(whereConditions, " AND ")

	total, ok := s.countWithError(w, "user_links", whereClause, args)
	if !ok {
		return
	}

	orderBy := pagination.BuildOrderBy(linkSortFields)
	links, err := database.QueryUserLinks(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		log.Errorf("[db] QueryUserLinks failed: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Database query failed")
		return
	}

	items := make([]DBUserLinkItem, len(links))
	ids := make([]interface{}, 0, len(links))
	for _, l := range links {
		ids = append(ids, int64(l.ParentLstEntityId))
	}
	entNames := s.batchLoadNames("lst_entities", "id", "name", ids)
	for i, l := range links {
		items[i] = dbUserLinkToItem(&l, entNames[int64(l.ParentLstEntityId)])
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(pagination.ToResponse(items, total)))
}

func (s *Server) handleDBUserLinkDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := s.resolvePathID(w, r, "id", "user link")
	if !ok {
		return
	}

	link, err := database.GetUserLinkById(s.db, int32(id))
	if !requireResource(link, err, "User link", func(c int, m string) { s.writeError(w, c, m) }) {
		return
	}

	lstEntName := ""
	if lstEnt, err := database.GetLstEntity(s.db, int(link.ParentLstEntityId)); err == nil && lstEnt != nil {
		lstEntName = lstEnt.Name
	}

	s.writeResourceJSON(w, dbUserLinkToItem(link, lstEntName))
}

func (s *Server) handleDBUserLinkUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := s.resolvePathID(w, r, "id", "user link")
	if !ok {
		return
	}

	var req struct {
		Name *string `json:"name"`
	}

	if !s.decodeBody(w, r, &req) {
		return
	}

	link, err := database.GetUserLinkById(s.db, int32(id))
	if !requireResource(link, err, "User link", func(c int, m string) { s.writeError(w, c, m) }) {
		return
	}

	if req.Name != nil {
		if err := validateFieldName(*req.Name); err != nil {
			log.Errorf("[db] Invalid name: %v", err)
			s.writeErrorDetail(w, http.StatusBadRequest, "Invalid name", err.Error())
			return
		}
		link.Name = *req.Name
	}

	if err := database.UpdateUserLink(s.db, link.Id, link.Name); err != nil {
		log.Errorf("[db] UpdateUserLink failed: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Database query failed")
		return
	}

	lstEntName := ""
	if lstEnt, err := database.GetLstEntity(s.db, int(link.ParentLstEntityId)); err == nil && lstEnt != nil {
		lstEntName = lstEnt.Name
	}

	s.writeResourceJSON(w, dbUserLinkToItem(link, lstEntName))
}

func (s *Server) handleDBUserLinkDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := s.resolvePathID(w, r, "id", "user link")
	if !ok {
		return
	}

	link, err := database.GetUserLinkById(s.db, int32(id))
	if !requireResource(link, err, "User link", func(c int, m string) { s.writeError(w, c, m) }) {
		return
	}

	if err := database.DelUserLink(s.db, int32(id)); err != nil {
		log.Errorf("[db] DelUserLink failed: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Database query failed")
		return
	}

	s.writeResourceDeleted(w, "User link")
}

// ============ User Previous Names 查询（新增） ============

func (s *Server) handleDBUserPreviousNames(w http.ResponseWriter, r *http.Request) {
	id, ok := s.resolvePathID(w, r, "id", "user")
	if !ok {
		return
	}

	pagination := NewPagination(r)

	whereClause := "pn.user_id = ?"
	args := []interface{}{id}

	tableExpr := "user_previous_names pn LEFT JOIN users u ON pn.user_id = u.id"
	total, ok2 := s.countWithError(w, tableExpr, whereClause, args)
	if !ok2 {
		return
	}

	orderBy := pagination.BuildOrderBy(prevNameSortFields)
	names, err := database.QueryAllUserPreviousNames(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		log.Errorf("[db] QueryAllUserPreviousNames failed: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Database query failed")
		return
	}

	items := make([]DBUserPreviousNameItem, len(names))
	for i, n := range names {
		items[i] = dbPrevNameToItem(&n)
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(pagination.ToResponse(items, total)))
}

// ============ 子资源快捷端点 ============

// handleDBUserEntitiesByUserID 获取指定用户的所有实体
func (s *Server) handleDBUserEntitiesByUserID(w http.ResponseWriter, r *http.Request) {
	id, ok := s.resolvePathID(w, r, "id", "user")
	if !ok {
		return
	}

	pagination := NewPagination(r)
	whereClause := "user_id = ?"
	args := []interface{}{id}

	total, ok2 := s.countWithError(w, "user_entities", whereClause, args)
	if !ok2 {
		return
	}

	orderBy := pagination.BuildOrderBy(entitySortFields)
	entities, err := database.QueryUserEntities(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		log.Errorf("[db] QueryUserEntities failed: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Database query failed")
		return
	}

	items := make([]DBEntityItem, len(entities))
	for i, e := range entities {
		items[i] = dbEntityToItem(&e)
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(pagination.ToResponse(items, total)))
}

// handleDBUserLinksByUserID 获取指定用户的所有链接
func (s *Server) handleDBUserLinksByUserID(w http.ResponseWriter, r *http.Request) {
	id, ok := s.resolvePathID(w, r, "id", "user")
	if !ok {
		return
	}

	pagination := NewPagination(r)
	whereClause := "user_id = ?"
	args := []interface{}{id}

	total, ok2 := s.countWithError(w, "user_links", whereClause, args)
	if !ok2 {
		return
	}

	orderBy := pagination.BuildOrderBy(linkSortFields)
	links, err := database.QueryUserLinks(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		log.Errorf("[db] QueryUserLinks failed: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Database query failed")
		return
	}

	items := make([]DBUserLinkItem, len(links))
	ids := make([]interface{}, 0, len(links))
	for _, l := range links {
		ids = append(ids, int64(l.ParentLstEntityId))
	}
	entNames := s.batchLoadNames("lst_entities", "id", "name", ids)
	for i, l := range links {
		items[i] = dbUserLinkToItem(&l, entNames[int64(l.ParentLstEntityId)])
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(pagination.ToResponse(items, total)))
}

// handleDBLstEntitiesByListID 获取指定列表的所有实体
func (s *Server) handleDBLstEntitiesByListID(w http.ResponseWriter, r *http.Request) {
	id, ok := s.resolvePathID(w, r, "id", "list")
	if !ok {
		return
	}

	pagination := NewPagination(r)
	whereClause := "lst_id = ?"
	args := []interface{}{id}

	total, ok2 := s.countWithError(w, "lst_entities", whereClause, args)
	if !ok2 {
		return
	}

	orderBy := pagination.BuildOrderBy(lstEntitySortFields)
	entities, err := database.QueryLstEntities(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		log.Errorf("[db] QueryLstEntities failed: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Database query failed")
		return
	}

	items := make([]DBListEntityItem, len(entities))
	ids := make([]interface{}, 0, len(entities))
	for _, e := range entities {
		ids = append(ids, e.LstId)
	}
	lstNames := s.batchLoadNames("lsts", "id", "name", ids)
	for i, e := range entities {
		items[i] = dbLstEntityToItem(&e, lstNames[e.LstId])
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(pagination.ToResponse(items, total)))
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
			log.Errorf("[db] Failed to count %s: %v", t.Name, err)
			s.writeError(w, http.StatusInternalServerError, "Database query failed")
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

	tableExpr := "user_previous_names pn LEFT JOIN users u ON pn.user_id = u.id"
	total, ok := s.countWithError(w, tableExpr, whereClause, args)
	if !ok {
		return
	}

	orderBy := pagination.BuildOrderBy(prevNameSortFields)
	names, err := database.QueryAllUserPreviousNames(s.db, whereClause, args, orderBy, pagination.PageSize, pagination.Offset)
	if err != nil {
		log.Errorf("[db] QueryAllUserPreviousNames failed: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Database query failed")
		return
	}

	items := make([]DBUserPreviousNameItem, len(names))
	for i, n := range names {
		items[i] = dbPrevNameToItem(&n)
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(pagination.ToResponse(items, total)))
}

// ============ 辅助函数 ============

func nullInt32(n sql.NullInt32) int32 {
	if n.Valid {
		return n.Int32
	}
	return 0
}

// validateFieldName 校验数据库字段 Name 的值，拒绝空串和超长值。
func validateFieldName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("must not be empty")
	}
	if len(trimmed) > 250 {
		return fmt.Errorf("must not exceed 250 characters")
	}
	return nil
}

// validateScreenName 校验 Twitter screen name 格式（复用 utils.IsValidScreenName）。
func validateScreenName(name string) error {
	if !utils.IsValidScreenName(name) {
		return fmt.Errorf("must be 1-15 characters (letters, digits, underscores)")
	}
	return nil
}
