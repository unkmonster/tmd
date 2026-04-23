package api

import (
	"fmt"
	"net/http"
	"strconv"
)

// Pagination 分页参数
type Pagination struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	SortBy    string `json:"sortBy"`
	SortOrder string `json:"sortOrder"`
	Offset    int    `json:"-"`
}

// PaginatedResponse 分页响应
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"pageSize"`
	TotalPages int         `json:"totalPages"`
}

// NewPagination 从请求创建分页参数
func NewPagination(r *http.Request) *Pagination {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	sortBy := r.URL.Query().Get("sortBy")
	if sortBy == "" {
		sortBy = "id"
	}

	sortOrder := r.URL.Query().Get("sortOrder")
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	return &Pagination{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    sortBy,
		SortOrder: sortOrder,
		Offset:    (page - 1) * pageSize,
	}
}

// BuildOrderBy 构建 ORDER BY 子句
func (p *Pagination) BuildOrderBy(allowedFields map[string]string) string {
	field, ok := allowedFields[p.SortBy]
	if !ok {
		field = allowedFields["id"]
	}
	return fmt.Sprintf("ORDER BY %s %s", field, p.SortOrder)
}

// ToResponse 转换为分页响应
func (p *Pagination) ToResponse(data interface{}, total int) *PaginatedResponse {
	totalPages := total / p.PageSize
	if total%p.PageSize > 0 {
		totalPages++
	}
	return &PaginatedResponse{
		Data:       data,
		Total:      total,
		Page:       p.Page,
		PageSize:   p.PageSize,
		TotalPages: totalPages,
	}
}
