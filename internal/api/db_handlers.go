package api

import (
	"database/sql"
	"net/http"
	"path/filepath"

	"github.com/unkmonster/tmd/internal/database"
)

// handleDBUsers 查询数据库用户
func (s *Server) handleDBUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var rows []database.User
	err := s.db.Select(&rows, "SELECT * FROM users ORDER BY id DESC LIMIT 100")
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]DBUserItem, len(rows))
	for i, u := range rows {
		items[i] = DBUserItem{
			ID:           u.Id,
			ScreenName:   u.ScreenName,
			Name:         u.Name,
			IsProtected:  u.IsProtected,
			FriendsCount: u.FriendsCount,
			IsAccessible: u.IsAccessible,
		}
	}
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(DBUserResponse{Users: items, Total: len(items)}))
}

// handleDBLists 查询数据库列表
func (s *Server) handleDBLists(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var rows []database.Lst
	err := s.db.Select(&rows, "SELECT * FROM lsts ORDER BY id DESC LIMIT 100")
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]DBListItem, len(rows))
	for i, l := range rows {
		items[i] = DBListItem{
			ID:      l.Id,
			Name:    l.Name,
			OwnerID: l.OwnerId,
		}
	}
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(DBListResponse{Lists: items, Total: len(items)}))
}

// handleDBUserEntities 查询用户实体
func (s *Server) handleDBUserEntities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var rows []database.UserEntity
	err := s.db.Select(&rows, "SELECT * FROM user_entities ORDER BY id DESC LIMIT 100")
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]DBEntityItem, len(rows))
	for i, e := range rows {
		items[i] = DBEntityItem{
			ID:         int64(nullInt32(e.Id)),
			UserID:     e.Uid,
			Name:       e.Name,
			ParentDir:  e.ParentDir,
			MediaCount: nullInt32(e.MediaCount),
		}
		if e.LatestReleaseTime.Valid {
			items[i].LatestReleaseTime = e.LatestReleaseTime.Time.Format("2006-01-02 15:04:05")
		}
	}
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(DBEntityResponse{Entities: items, Total: len(items)}))
}

// nullInt32 辅助函数：将 sql.NullInt32 转为 int32，无效时返回 0
func nullInt32(n sql.NullInt32) int32 {
	if n.Valid {
		return n.Int32
	}
	return 0
}

// handleConfig 获取脱敏配置
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	// 仅返回目录名，避免暴露完整绝对路径
	rootName := filepath.Base(s.config.RootPath)
	resp := ConfigResponse{
		RootPath:           rootName,
		MaxDownloadRoutine: s.config.MaxDownloadRoutine,
		MaxFileNameLen:     s.config.MaxFileNameLen,
	}
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(resp))
}
