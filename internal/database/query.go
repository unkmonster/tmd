package database

import (
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

// QueryOptions 查询选项
type QueryOptions struct {
	Where   string
	Args    []interface{}
	OrderBy string
	Limit   int
	Offset  int
}

// Count 获取总数
func Count(db *sqlx.DB, table string, opts *QueryOptions) (int, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
	if opts != nil && opts.Where != "" {
		query += " WHERE " + opts.Where
	}

	var count int
	var err error
	if opts != nil && len(opts.Args) > 0 {
		err = db.Get(&count, query, opts.Args...)
	} else {
		err = db.Get(&count, query)
	}
	return count, err
}

// BuildSearchCondition 构建搜索条件
func BuildSearchCondition(fields []string, keyword string) (string, []interface{}) {
	if keyword == "" || len(fields) == 0 {
		return "", nil
	}

	conditions := make([]string, len(fields))
	args := make([]interface{}, len(fields))
	for i, field := range fields {
		conditions[i] = fmt.Sprintf("%s LIKE ?", field)
		args[i] = "%" + keyword + "%"
	}

	return "(" + strings.Join(conditions, " OR ") + ")", args
}

// QueryUsers 分页查询用户
func QueryUsers(db *sqlx.DB, where string, args []interface{}, orderBy string, limit, offset int) ([]User, error) {
	query := "SELECT * FROM users"
	if where != "" {
		query += " WHERE " + where
	}
	if orderBy != "" {
		query += " " + orderBy
	}
	query += " LIMIT ? OFFSET ?"

	var users []User
	err := db.Select(&users, query, append(args, limit, offset)...)
	return users, err
}

// QueryLists 分页查询列表
func QueryLists(db *sqlx.DB, where string, args []interface{}, orderBy string, limit, offset int) ([]Lst, error) {
	query := "SELECT * FROM lsts"
	if where != "" {
		query += " WHERE " + where
	}
	if orderBy != "" {
		query += " " + orderBy
	}
	query += " LIMIT ? OFFSET ?"

	var lists []Lst
	err := db.Select(&lists, query, append(args, limit, offset)...)
	return lists, err
}

// QueryUserEntities 分页查询用户实体
func QueryUserEntities(db *sqlx.DB, where string, args []interface{}, orderBy string, limit, offset int) ([]UserEntity, error) {
	query := "SELECT * FROM user_entities"
	if where != "" {
		query += " WHERE " + where
	}
	if orderBy != "" {
		query += " " + orderBy
	}
	query += " LIMIT ? OFFSET ?"

	var entities []UserEntity
	err := db.Select(&entities, query, append(args, limit, offset)...)
	return entities, err
}

// QueryLstEntities 分页查询列表实体
func QueryLstEntities(db *sqlx.DB, where string, args []interface{}, orderBy string, limit, offset int) ([]LstEntity, error) {
	query := "SELECT * FROM lst_entities"
	if where != "" {
		query += " WHERE " + where
	}
	if orderBy != "" {
		query += " " + orderBy
	}
	query += " LIMIT ? OFFSET ?"

	var entities []LstEntity
	err := db.Select(&entities, query, append(args, limit, offset)...)
	return entities, err
}

// QueryUserLinks 查询用户链接
func QueryUserLinks(db *sqlx.DB, where string, args []interface{}, orderBy string, limit, offset int) ([]UserLink, error) {
	query := "SELECT * FROM user_links"
	if where != "" {
		query += " WHERE " + where
	}
	if orderBy != "" {
		query += " " + orderBy
	}
	query += " LIMIT ? OFFSET ?"

	var links []UserLink
	err := db.Select(&links, query, append(args, limit, offset)...)
	return links, err
}

// QueryUserPreviousNames 查询用户历史名称
func QueryUserPreviousNames(db *sqlx.DB, uid uint64, limit, offset int) ([]UserPreviousName, error) {
	var names []UserPreviousName
	err := db.Select(&names,
		"SELECT * FROM user_previous_names WHERE user_id = ? ORDER BY record_date DESC LIMIT ? OFFSET ?",
		uid, limit, offset)
	return names, err
}

// QueryAllUserPreviousNames 全局查询用户历史名称（支持筛选、搜索、排序，JOIN users 获取当前名称）
func QueryAllUserPreviousNames(db *sqlx.DB, where string, args []interface{}, orderBy string, limit, offset int) ([]UserPreviousNameWithCurrent, error) {
	query := `SELECT pn.id, pn.user_id, pn.screen_name, pn.name, pn.record_date,
		u.screen_name AS current_screen_name, u.name AS current_name
		FROM user_previous_names pn
		LEFT JOIN users u ON pn.user_id = u.id`
	if where != "" {
		query += " WHERE " + where
	}
	if orderBy != "" {
		query += " " + orderBy
	} else {
		query += " ORDER BY pn.record_date DESC"
	}
	query += " LIMIT ? OFFSET ?"

	var names []UserPreviousNameWithCurrent
	err := db.Select(&names, query, append(args, limit, offset)...)
	return names, err
}
