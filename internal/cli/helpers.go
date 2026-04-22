package cli

import (
	"context"
	"fmt"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
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

	users, err := usrArgs.GetUser(ctx, client, db)
	if err != nil {
		return nil, err
	}
	task.Users = append(task.Users, users...)

	lists, err := listArgs.GetList(ctx, client)
	if err != nil {
		return nil, err
	}
	for _, list := range lists {
		task.Lists = append(task.Lists, list)
	}

	users, err = follArgs.GetUser(ctx, client, db)
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		task.Lists = append(task.Lists, user.Following())
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
