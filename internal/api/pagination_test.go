package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPagination_DefaultValues(t *testing.T) {
	req := &http.Request{
		URL: &url.URL{RawQuery: ""},
	}

	p := NewPagination(req)

	assert.Equal(t, 1, p.Page)
	assert.Equal(t, 20, p.PageSize)
	assert.Equal(t, "id", p.SortBy)
	assert.Equal(t, "desc", p.SortOrder)
	assert.Equal(t, 0, p.Offset)
}

func TestNewPagination_CustomValues(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		wantPage     int
		wantPageSize int
		wantSortBy   string
		wantOrder    string
		wantOffset   int
	}{
		{
			name:         "完整参数",
			query:        "page=3&pageSize=50&sortBy=name&sortOrder=asc",
			wantPage:     3,
			wantPageSize: 50,
			wantSortBy:   "name",
			wantOrder:    "asc",
			wantOffset:   100,
		},
		{
			name:         "仅页码",
			query:        "page=5",
			wantPage:     5,
			wantPageSize: 20,
			wantSortBy:   "id",
			wantOrder:    "desc",
			wantOffset:   80,
		},
		{
			name:         "仅页面大小",
			query:        "pageSize=100",
			wantPage:     1,
			wantPageSize: 100,
			wantSortBy:   "id",
			wantOrder:    "desc",
			wantOffset:   0,
		},
		{
			name:         "边界值-最大页面大小",
			query:        "pageSize=1000",
			wantPage:     1,
			wantPageSize: 20,
			wantSortBy:   "id",
			wantOrder:    "desc",
			wantOffset:   0,
		},
		{
			name:         "边界值-零页码",
			query:        "page=0",
			wantPage:     1,
			wantPageSize: 20,
			wantSortBy:   "id",
			wantOrder:    "desc",
			wantOffset:   0,
		},
		{
			name:         "边界值-负页码",
			query:        "page=-1",
			wantPage:     1,
			wantPageSize: 20,
			wantSortBy:   "id",
			wantOrder:    "desc",
			wantOffset:   0,
		},
		{
			name:         "无效排序方向",
			query:        "sortOrder=invalid",
			wantPage:     1,
			wantPageSize: 20,
			wantSortBy:   "id",
			wantOrder:    "desc",
			wantOffset:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				URL: &url.URL{RawQuery: tt.query},
			}

			p := NewPagination(req)

			assert.Equal(t, tt.wantPage, p.Page)
			assert.Equal(t, tt.wantPageSize, p.PageSize)
			assert.Equal(t, tt.wantSortBy, p.SortBy)
			assert.Equal(t, tt.wantOrder, p.SortOrder)
			assert.Equal(t, tt.wantOffset, p.Offset)
		})
	}
}

func TestNewPagination_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		query string
		page  int
		size  int
	}{
		{
			name:  "空查询",
			query: "",
			page:  1,
			size:  20,
		},
		{
			name:  "无效页码字符串",
			query: "page=abc",
			page:  1,
			size:  20,
		},
		{
			name:  "无效页面大小字符串",
			query: "pageSize=xyz",
			page:  1,
			size:  20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				URL: &url.URL{RawQuery: tt.query},
			}

			p := NewPagination(req)
			assert.Equal(t, tt.page, p.Page)
			assert.Equal(t, tt.size, p.PageSize)
		})
	}
}

func TestPagination_BuildOrderBy(t *testing.T) {
	allowedFields := map[string]string{
		"id":   "id",
		"name": "name",
		"age":  "age",
	}

	tests := []struct {
		name      string
		sortBy    string
		sortOrder string
		want      string
	}{
		{
			name:      "有效字段升序",
			sortBy:    "name",
			sortOrder: "asc",
			want:      "ORDER BY name asc",
		},
		{
			name:      "有效字段降序",
			sortBy:    "name",
			sortOrder: "desc",
			want:      "ORDER BY name desc",
		},
		{
			name:      "无效字段使用默认",
			sortBy:    "invalid",
			sortOrder: "asc",
			want:      "ORDER BY id asc",
		},
		{
			name:      "空字段使用默认",
			sortBy:    "",
			sortOrder: "desc",
			want:      "ORDER BY id desc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pagination{
				SortBy:    tt.sortBy,
				SortOrder: tt.sortOrder,
			}

			got := p.BuildOrderBy(allowedFields)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPagination_BuildOrderBy_MissingDefault(t *testing.T) {
	// 测试当 allowedFields 中不包含 "id" 时的回退行为
	allowedFields := map[string]string{
		"name": "name",
	}

	p := &Pagination{
		SortBy:    "invalid",
		SortOrder: "asc",
	}

	// 当 "id" 不在 allowedFields 中时，应该返回空字符串，避免生成无效 SQL
	got := p.BuildOrderBy(allowedFields)
	assert.Equal(t, "", got)
}

func TestPagination_ToResponse(t *testing.T) {
	tests := []struct {
		name       string
		page       int
		pageSize   int
		total      int
		data       interface{}
		wantPages  int
		wantTotal  int
		wantPage   int
		wantSize   int
	}{
		{
			name:      "整除",
			page:      1,
			pageSize:  20,
			total:     100,
			data:      []string{"item1", "item2"},
			wantPages: 5,
			wantTotal: 100,
			wantPage:  1,
			wantSize:  20,
		},
		{
			name:      "有余数",
			page:      1,
			pageSize:  20,
			total:     105,
			data:      []string{"item1"},
			wantPages: 6,
			wantTotal: 105,
			wantPage:  1,
			wantSize:  20,
		},
		{
			name:      "零总数",
			page:      1,
			pageSize:  20,
			total:     0,
			data:      []string{},
			wantPages: 0,
			wantTotal: 0,
			wantPage:  1,
			wantSize:  20,
		},
		{
			name:      "单页",
			page:      1,
			pageSize:  50,
			total:     30,
			data:      []string{"item1", "item2"},
			wantPages: 1,
			wantTotal: 30,
			wantPage:  1,
			wantSize:  50,
		},
		{
			name:      "大数据集",
			page:      5,
			pageSize:  100,
			total:     1000,
			data:      []int{1, 2, 3},
			wantPages: 10,
			wantTotal: 1000,
			wantPage:  5,
			wantSize:  100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pagination{
				Page:     tt.page,
				PageSize: tt.pageSize,
			}

			resp := p.ToResponse(tt.data, tt.total)

			assert.Equal(t, tt.wantPages, resp.TotalPages)
			assert.Equal(t, tt.wantTotal, resp.Total)
			assert.Equal(t, tt.wantPage, resp.Page)
			assert.Equal(t, tt.wantSize, resp.PageSize)
			assert.Equal(t, tt.data, resp.Data)
		})
	}
}

func TestPagination_ToResponse_TotalPagesCalculation(t *testing.T) {
	tests := []struct {
		total    int
		pageSize int
		expected int
	}{
		{0, 20, 0},
		{1, 20, 1},
		{19, 20, 1},
		{20, 20, 1},
		{21, 20, 2},
		{100, 20, 5},
		{101, 20, 6},
		{100, 10, 10},
		{99, 10, 10},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("total_%d_size_%d", tt.total, tt.pageSize), func(t *testing.T) {
			p := &Pagination{
				Page:     1,
				PageSize: tt.pageSize,
			}

			resp := p.ToResponse([]int{}, tt.total)
			assert.Equal(t, tt.expected, resp.TotalPages)
		})
	}
}

func TestPaginatedResponse_JSONMarshal(t *testing.T) {
	data := []map[string]interface{}{
		{"id": 1, "name": "item1"},
		{"id": 2, "name": "item2"},
	}

	resp := &PaginatedResponse{
		Data:       data,
		Total:      100,
		Page:       2,
		PageSize:   20,
		TotalPages: 5,
	}

	bytes, err := json.Marshal(resp)
	assert.NoError(t, err)

	var decoded map[string]interface{}
	err = json.Unmarshal(bytes, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, float64(100), decoded["total"])
	assert.Equal(t, float64(2), decoded["page"])
	assert.Equal(t, float64(20), decoded["pageSize"])
	assert.Equal(t, float64(5), decoded["totalPages"])
	assert.NotNil(t, decoded["data"])
}

func TestPagination_OffsetCalculation(t *testing.T) {
	tests := []struct {
		page     int
		pageSize int
		offset   int
	}{
		{1, 20, 0},
		{2, 20, 20},
		{3, 20, 40},
		{1, 50, 0},
		{5, 10, 40},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("page_%d_size_%d", tt.page, tt.pageSize), func(t *testing.T) {
			// 通过 HTTP 请求创建分页，这样 Offset 会被正确计算
			u, _ := url.Parse(fmt.Sprintf("http://example.com/api?page=%d&pageSize=%d", tt.page, tt.pageSize))
			req := &http.Request{URL: u}
			p := NewPagination(req)
			assert.Equal(t, tt.offset, p.Offset)
		})
	}
}

func TestNewPagination_RealHTTPRequest(t *testing.T) {
	// 使用真实 HTTP 请求测试
	u, _ := url.Parse("http://example.com/api?page=2&pageSize=30&sortBy=created_at&sortOrder=asc")
	req := &http.Request{
		URL: u,
	}

	p := NewPagination(req)

	assert.Equal(t, 2, p.Page)
	assert.Equal(t, 30, p.PageSize)
	assert.Equal(t, "created_at", p.SortBy)
	assert.Equal(t, "asc", p.SortOrder)
	assert.Equal(t, 30, p.Offset)
}

func TestPagination_CompleteFlow(t *testing.T) {
	// 测试完整的分页流程
	u, _ := url.Parse("http://example.com/api?page=3&pageSize=25&sortBy=name&sortOrder=desc")
	req := &http.Request{
		URL: u,
	}

	p := NewPagination(req)

	// 验证分页参数
	assert.Equal(t, 3, p.Page)
	assert.Equal(t, 25, p.PageSize)
	assert.Equal(t, 50, p.Offset) // (3-1) * 25

	// 验证排序构建
	allowedFields := map[string]string{
		"id":   "id",
		"name": "user_name",
		"age":  "user_age",
	}
	orderBy := p.BuildOrderBy(allowedFields)
	assert.Equal(t, "ORDER BY user_name desc", orderBy)

	// 验证响应构建
	data := []string{"user1", "user2", "user3"}
	resp := p.ToResponse(data, 100)

	assert.Equal(t, data, resp.Data)
	assert.Equal(t, 100, resp.Total)
	assert.Equal(t, 3, resp.Page)
	assert.Equal(t, 25, resp.PageSize)
	assert.Equal(t, 4, resp.TotalPages) // 100 / 25 = 4
}
