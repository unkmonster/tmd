package cli

import (
	"context"
	"fmt"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/twitter"
)

// Task 任务
type Task struct {
	Users []*twitter.User
	Lists []twitter.ListBase
}

// MakeTask 创建任务
func MakeTask(ctx context.Context, client *resty.Client, db *sqlx.DB, usrArgs UserArgs, listArgs ListArgs, follArgs UserArgs) (*Task, error) {
	task := Task{
		Users: make([]*twitter.User, 0),
		Lists: make([]twitter.ListBase, 0),
	}

	// 处理用户参数
	for _, id := range usrArgs.ID {
		usr, uid, err := twitter.GetUserById(ctx, client, id)
		if err != nil {
			database.MarkUserInaccessible(db, uid, "")
			return nil, err
		}
		task.Users = append(task.Users, usr)
	}
	for _, screenName := range usrArgs.ScreenName {
		usr, uid, err := twitter.GetUserByScreenName(ctx, client, screenName)
		if err != nil {
			database.MarkUserInaccessible(db, uid, screenName)
			return nil, err
		}
		task.Users = append(task.Users, usr)
	}

	lists, err := listArgs.GetList(ctx, client)
	if err != nil {
		return nil, err
	}
	for _, list := range lists {
		task.Lists = append(task.Lists, list)
	}

	// 处理关注参数
	for _, id := range follArgs.ID {
		usr, uid, err := twitter.GetUserById(ctx, client, id)
		if err != nil {
			database.MarkUserInaccessible(db, uid, "")
			return nil, err
		}
		task.Lists = append(task.Lists, usr.Following())
	}
	for _, screenName := range follArgs.ScreenName {
		usr, uid, err := twitter.GetUserByScreenName(ctx, client, screenName)
		if err != nil {
			database.MarkUserInaccessible(db, uid, screenName)
			return nil, err
		}
		task.Lists = append(task.Lists, usr.Following())
	}

	return &task, nil
}

// PrintTask 打印任务
func PrintTask(task *Task) {
	if len(task.Users) != 0 {
		fmt.Printf("users: %d\n", len(task.Users))
	}
	for _, u := range task.Users {
		fmt.Printf("    - %s\n", u.Title())
	}
	if len(task.Lists) != 0 {
		fmt.Printf("lists: %d\n", len(task.Lists))
	}
	for _, l := range task.Lists {
		fmt.Printf("    - %s\n", l.Title())
	}
}
