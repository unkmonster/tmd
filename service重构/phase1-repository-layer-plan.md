# Phase 1: Repository 层实现计划

## 1. 目标

创建 Repository 层，将数据访问逻辑从 database 包中抽象出来，为 Service 层提供统一的数据访问接口。

## 2. 设计原则

遵循 CLAUDE.md 的准则：
- **简单优先**：只做必要的抽象，不过度设计
- **外科手术式修改**：保持现有代码风格，只改必须改的内容
- **目标驱动**：每个步骤都有明确的验证标准

## 3. 模块划分

```
internal/
├── repository/           # 新增目录
│   ├── interfaces.go     # Repository 接口定义
│   ├── user_repo.go      # 用户相关数据访问
│   ├── list_repo.go      # 列表相关数据访问
│   ├── entity_repo.go    # 实体相关数据访问
│   └── link_repo.go      # 用户链接相关数据访问
├── database/             # 现有目录（保持不变，作为底层实现）
│   └── ...
```

## 4. 详细步骤

### 步骤 1: 创建 Repository 接口定义

**文件**: `internal/repository/interfaces.go`

**新增代码**:
```go
package repository

import (
    "context"
    "github.com/jmoiron/sqlx"
)

// User 用户模型
type User struct {
    ID           uint64
    ScreenName   string
    Name         string
    IsProtected  bool
    FriendsCount int
    IsAccessible bool
}

// List 列表模型
type List struct {
    ID      uint64
    Name    string
    OwnerID uint64
}

// UserEntity 用户实体模型
type UserEntity struct {
    ID                int
    UserID            uint64
    Name              string
    LatestReleaseTime *string
    ParentDir         string
    MediaCount        int32
}

// ListEntity 列表实体模型
type ListEntity struct {
    ID        int
    LstID     int64
    Name      string
    ParentDir string
}

// UserLink 用户链接模型
type UserLink struct {
    ID                int
    UserID            uint64
    ParentLstEntityID int
}

// UserRepository 用户仓库接口
type UserRepository interface {
    GetByID(ctx context.Context, id uint64) (*User, error)
    GetByScreenName(ctx context.Context, screenName string) (*User, error)
    Create(ctx context.Context, user *User) error
    Update(ctx context.Context, user *User) error
    MarkInaccessible(ctx context.Context, id uint64, screenName string) error
    GetPreviousNames(ctx context.Context, userID uint64) ([]string, error)
}

// ListRepository 列表仓库接口
type ListRepository interface {
    GetByID(ctx context.Context, id uint64) (*List, error)
    Create(ctx context.Context, list *List) error
    Update(ctx context.Context, list *List) error
}

// EntityRepository 实体仓库接口
type EntityRepository interface {
    // 用户实体
    GetUserEntityByUserID(ctx context.Context, userID uint64) (*UserEntity, error)
    CreateUserEntity(ctx context.Context, entity *UserEntity) error
    UpdateUserEntity(ctx context.Context, entity *UserEntity) error
    
    // 列表实体
    GetListEntityByListID(ctx context.Context, listID int64) (*ListEntity, error)
    CreateListEntity(ctx context.Context, entity *ListEntity) error
    UpdateListEntity(ctx context.Context, entity *ListEntity) error
}

// LinkRepository 链接仓库接口
type LinkRepository interface {
    GetByListEntityID(ctx context.Context, listEntityID int) ([]*UserLink, error)
    Create(ctx context.Context, link *UserLink) error
    Delete(ctx context.Context, id int) error
}
```

**风险评估**: 
- 风险: 低（只是接口定义）
- 注意: 模型字段需要与 database 包保持一致

**测试要点**:
- 编译通过即可

---

### 步骤 2: 实现 UserRepository

**文件**: `internal/repository/user_repo.go`

**新增代码**:
```go
package repository

import (
    "context"
    "database/sql"
    "github.com/jmoiron/sqlx"
)

type userRepo struct {
    db *sqlx.DB
}

// NewUserRepository 创建用户仓库
func NewUserRepository(db *sqlx.DB) UserRepository {
    return &userRepo{db: db}
}

func (r *userRepo) GetByID(ctx context.Context, id uint64) (*User, error) {
    // 调用现有的 database.GetUserById
    // 注意：这里需要检查 database 包的函数签名
    row := r.db.QueryRowxContext(ctx, 
        "SELECT id, screen_name, name, protected, friends_count, is_accessible FROM users WHERE id = ?", id)
    
    var user User
    err := row.Scan(&user.ID, &user.ScreenName, &user.Name, &user.IsProtected, &user.FriendsCount, &user.IsAccessible)
    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    return &user, nil
}

func (r *userRepo) GetByScreenName(ctx context.Context, screenName string) (*User, error) {
    row := r.db.QueryRowxContext(ctx,
        "SELECT id, screen_name, name, protected, friends_count, is_accessible FROM users WHERE screen_name = ?", screenName)
    
    var user User
    err := row.Scan(&user.ID, &user.ScreenName, &user.Name, &user.IsProtected, &user.FriendsCount, &user.IsAccessible)
    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    return &user, nil
}

func (r *userRepo) Create(ctx context.Context, user *User) error {
    _, err := r.db.ExecContext(ctx,
        "INSERT INTO users (id, screen_name, name, protected, friends_count) VALUES (?, ?, ?, ?, ?)",
        user.ID, user.ScreenName, user.Name, user.IsProtected, user.FriendsCount)
    return err
}

func (r *userRepo) Update(ctx context.Context, user *User) error {
    _, err := r.db.ExecContext(ctx,
        "UPDATE users SET screen_name = ?, name = ?, protected = ?, friends_count = ? WHERE id = ?",
        user.ScreenName, user.Name, user.IsProtected, user.FriendsCount, user.ID)
    return err
}

func (r *userRepo) MarkInaccessible(ctx context.Context, id uint64, screenName string) error {
    // 调用现有的 database.MarkUserInaccessible 逻辑
    if id != 0 {
        _, err := r.db.ExecContext(ctx, "UPDATE users SET is_accessible = 0 WHERE id = ?", id)
        return err
    }
    if screenName != "" {
        _, err := r.db.ExecContext(ctx, "UPDATE users SET is_accessible = 0 WHERE screen_name = ?", screenName)
        return err
    }
    return nil
}

func (r *userRepo) GetPreviousNames(ctx context.Context, userID uint64) ([]string, error) {
    rows, err := r.db.QueryContext(ctx,
        "SELECT screen_name FROM user_previous_names WHERE uid = ? ORDER BY record_date DESC", userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var names []string
    for rows.Next() {
        var name string
        if err := rows.Scan(&name); err != nil {
            return nil, err
        }
        names = append(names, name)
    }
    return names, rows.Err()
}
```

**风险评估**:
- 风险: 中（需要确保 SQL 与现有 database 包一致）
- 注意: 需要检查现有 database 包的表结构和字段名

**测试要点**:
- 编译通过
- 各方法能正确执行 CRUD 操作

---

### 步骤 3: 实现 ListRepository

**文件**: `internal/repository/list_repo.go`

**新增代码**:
```go
package repository

import (
    "context"
    "database/sql"
    "github.com/jmoiron/sqlx"
)

type listRepo struct {
    db *sqlx.DB
}

// NewListRepository 创建列表仓库
func NewListRepository(db *sqlx.DB) ListRepository {
    return &listRepo{db: db}
}

func (r *listRepo) GetByID(ctx context.Context, id uint64) (*List, error) {
    row := r.db.QueryRowxContext(ctx,
        "SELECT id, name, owner_id FROM lsts WHERE id = ?", id)
    
    var list List
    err := row.Scan(&list.ID, &list.Name, &list.OwnerID)
    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    return &list, nil
}

func (r *listRepo) Create(ctx context.Context, list *List) error {
    _, err := r.db.ExecContext(ctx,
        "INSERT INTO lsts (id, name, owner_id) VALUES (?, ?, ?)",
        list.ID, list.Name, list.OwnerID)
    return err
}

func (r *listRepo) Update(ctx context.Context, list *List) error {
    _, err := r.db.ExecContext(ctx,
        "UPDATE lsts SET name = ?, owner_id = ? WHERE id = ?",
        list.Name, list.OwnerID, list.ID)
    return err
}
```

**风险评估**:
- 风险: 低
- 注意: 表名是 `lsts` 不是 `lists`

---

### 步骤 4: 实现 EntityRepository

**文件**: `internal/repository/entity_repo.go`

**新增代码**:
```go
package repository

import (
    "context"
    "database/sql"
    "github.com/jmoiron/sqlx"
)

type entityRepo struct {
    db *sqlx.DB
}

// NewEntityRepository 创建实体仓库
func NewEntityRepository(db *sqlx.DB) EntityRepository {
    return &entityRepo{db: db}
}

// 用户实体方法
func (r *entityRepo) GetUserEntityByUserID(ctx context.Context, userID uint64) (*UserEntity, error) {
    row := r.db.QueryRowxContext(ctx,
        "SELECT id, uid, name, latest_release_time, parent_dir, media_count FROM user_entities WHERE uid = ?", userID)
    
    var entity UserEntity
    err := row.Scan(&entity.ID, &entity.UserID, &entity.Name, &entity.LatestReleaseTime, &entity.ParentDir, &entity.MediaCount)
    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    return &entity, nil
}

func (r *entityRepo) CreateUserEntity(ctx context.Context, entity *UserEntity) error {
    result, err := r.db.ExecContext(ctx,
        "INSERT INTO user_entities (uid, name, parent_dir) VALUES (?, ?, ?)",
        entity.UserID, entity.Name, entity.ParentDir)
    if err != nil {
        return err
    }
    id, _ := result.LastInsertId()
    entity.ID = int(id)
    return nil
}

func (r *entityRepo) UpdateUserEntity(ctx context.Context, entity *UserEntity) error {
    _, err := r.db.ExecContext(ctx,
        "UPDATE user_entities SET name = ?, latest_release_time = ?, parent_dir = ?, media_count = ? WHERE id = ?",
        entity.Name, entity.LatestReleaseTime, entity.ParentDir, entity.MediaCount, entity.ID)
    return err
}

// 列表实体方法
func (r *entityRepo) GetListEntityByListID(ctx context.Context, listID int64) (*ListEntity, error) {
    row := r.db.QueryRowxContext(ctx,
        "SELECT id, lst_id, name, parent_dir FROM lst_entities WHERE lst_id = ?", listID)
    
    var entity ListEntity
    err := row.Scan(&entity.ID, &entity.LstID, &entity.Name, &entity.ParentDir)
    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    return &entity, nil
}

func (r *entityRepo) CreateListEntity(ctx context.Context, entity *ListEntity) error {
    result, err := r.db.ExecContext(ctx,
        "INSERT INTO lst_entities (lst_id, name, parent_dir) VALUES (?, ?, ?)",
        entity.LstID, entity.Name, entity.ParentDir)
    if err != nil {
        return err
    }
    id, _ := result.LastInsertId()
    entity.ID = int(id)
    return nil
}

func (r *entityRepo) UpdateListEntity(ctx context.Context, entity *ListEntity) error {
    _, err := r.db.ExecContext(ctx,
        "UPDATE lst_entities SET name = ?, parent_dir = ? WHERE id = ?",
        entity.Name, entity.ParentDir, entity.ID)
    return err
}
```

**风险评估**:
- 风险: 中（需要确认表结构和字段类型）
- 注意: `latest_release_time` 可能是 time.Time 或 string 类型

---

### 步骤 5: 实现 LinkRepository

**文件**: `internal/repository/link_repo.go`

**新增代码**:
```go
package repository

import (
    "context"
    "github.com/jmoiron/sqlx"
)

type linkRepo struct {
    db *sqlx.DB
}

// NewLinkRepository 创建链接仓库
func NewLinkRepository(db *sqlx.DB) LinkRepository {
    return &linkRepo{db: db}
}

func (r *linkRepo) GetByListEntityID(ctx context.Context, listEntityID int) ([]*UserLink, error) {
    rows, err := r.db.QueryContext(ctx,
        "SELECT id, uid, parent_lst_entity_id FROM user_links WHERE parent_lst_entity_id = ?", listEntityID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var links []*UserLink
    for rows.Next() {
        var link UserLink
        if err := rows.Scan(&link.ID, &link.UserID, &link.ParentLstEntityID); err != nil {
            return nil, err
        }
        links = append(links, &link)
    }
    return links, rows.Err()
}

func (r *linkRepo) Create(ctx context.Context, link *UserLink) error {
    result, err := r.db.ExecContext(ctx,
        "INSERT INTO user_links (uid, parent_lst_entity_id) VALUES (?, ?)",
        link.UserID, link.ParentLstEntityID)
    if err != nil {
        return err
    }
    id, _ := result.LastInsertId()
    link.ID = int(id)
    return nil
}

func (r *linkRepo) Delete(ctx context.Context, id int) error {
    _, err := r.db.ExecContext(ctx, "DELETE FROM user_links WHERE id = ?", id)
    return err
}
```

**风险评估**:
- 风险: 低

---

## 5. 验证步骤

### 5.1 编译验证
```bash
cd C:\Users\leeexxx\Documents\trae_projects\tmd
go build ./...
```

### 5.2 单元测试（可选，但推荐）
为每个 Repository 创建简单的单元测试：

**文件**: `internal/repository/user_repo_test.go`
```go
package repository

import (
    "context"
    "testing"
    "github.com/jmoiron/sqlx"
    _ "github.com/mattn/go-sqlite3"
)

func TestUserRepository(t *testing.T) {
    // 使用内存数据库测试
    db, err := sqlx.Open("sqlite3", ":memory:")
    if err != nil {
        t.Fatal(err)
    }
    defer db.Close()
    
    // 创建表
    db.MustExec(`
        CREATE TABLE users (
            id INTEGER PRIMARY KEY,
            screen_name TEXT,
            name TEXT,
            protected BOOLEAN,
            friends_count INTEGER,
            is_accessible BOOLEAN DEFAULT 1
        )
    `)
    
    repo := NewUserRepository(db)
    ctx := context.Background()
    
    // 测试 Create
    user := &User{
        ID:           123,
        ScreenName:   "testuser",
        Name:         "Test User",
        IsProtected:  false,
        FriendsCount: 100,
    }
    if err := repo.Create(ctx, user); err != nil {
        t.Errorf("Create failed: %v", err)
    }
    
    // 测试 GetByID
    found, err := repo.GetByID(ctx, 123)
    if err != nil {
        t.Errorf("GetByID failed: %v", err)
    }
    if found == nil || found.ScreenName != "testuser" {
        t.Error("GetByID returned wrong user")
    }
}
```

## 6. 与现有代码的关系

### 6.1 不修改的代码
- `internal/database/*` - 保持原样，作为底层实现参考
- `internal/cli/*` - 暂不修改，Phase 3 再适配
- `internal/api/*` - 暂不修改，Phase 4 再适配

### 6.2 后续会使用的代码
Phase 2 的 Service 层将使用：
```go
import "github.com/unkmonster/tmd/internal/repository"

type downloadService struct {
    userRepo   repository.UserRepository
    listRepo   repository.ListRepository
    entityRepo repository.EntityRepository
    linkRepo   repository.LinkRepository
}
```

## 7. 风险评估

| 风险 | 可能性 | 影响 | 缓解措施 |
|------|--------|------|----------|
| SQL 与现有表结构不一致 | 中 | 高 | 1. 仔细对照 database 包的 SQL<br>2. 运行测试验证 |
| 遗漏必要的查询条件 | 中 | 中 | 1. 检查所有使用 database 包的地方<br>2. 补充遗漏的方法 |
| 事务处理不当 | 低 | 高 | Phase 2 时再考虑复杂事务场景 |

## 8. 成功标准

- [ ] 所有 Repository 接口定义完成
- [ ] 所有 Repository 实现完成
- [ ] 代码编译通过
- [ ] （可选）单元测试通过
- [ ] 代码符合 CLAUDE.md 的准则（简单、精确、目标驱动）

## 9. 预计时间

- 步骤 1 (接口定义): 1-2 小时
- 步骤 2-5 (实现): 4-6 小时
- 测试与验证: 1-2 小时
- **总计**: 6-10 小时
