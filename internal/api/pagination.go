package api

import (
	"fmt"
	"net/http"
	"strconv"
)

const (
	defaultPageNumber     = 1
	defaultPageSize       = 20
	defaultMaxPageSize    = 200
	defaultPaginationSort = "id"
	defaultSortOrder      = "desc"
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
	return NewPaginationWithDefaults(r, defaultPageSize, defaultMaxPageSize, defaultPaginationSort, defaultSortOrder)
}

// NewPaginationWithDefaults 从请求创建分页参数，允许调用方覆盖默认 pageSize/maxPageSize/sort 配置。
func NewPaginationWithDefaults(r *http.Request, fallbackPageSize, maxPageSize int, fallbackSortBy, fallbackSortOrder string) *Pagination {
	pageStr := r.URL.Query().Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = defaultPageNumber
	}

	pageSizeStr := r.URL.Query().Get("pageSize")
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > maxPageSize {
		pageSize = fallbackPageSize
	}

	sortBy := r.URL.Query().Get("sortBy")
	if sortBy == "" {
		sortBy = fallbackSortBy
	}

	sortOrder := r.URL.Query().Get("sortOrder")
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = fallbackSortOrder
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
		field, ok = allowedFields["id"]
		if !ok || field == "" {
			return ""
		}
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
