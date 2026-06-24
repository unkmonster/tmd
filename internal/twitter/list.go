package twitter

import (
	"context"
	"fmt"
	"html"

	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

type ListBase interface {
	GetMembers(context.Context, *resty.Client) (*MembersResult, error)
	GetId() int64
	Title() string
}

type MembersResult struct {
	Users []*User
}

type List struct {
	Id          uint64
	MemberCount int
	Name        string
	Creator     *User
}

func GetLst(ctx context.Context, client *resty.Client, id uint64) (*List, error) {
	api := listByRestId{}
	api.id = id
	url := makeUrl(&api)

	resp, err := client.R().SetContext(ctx).Get(url)
	if err != nil {
		return nil, err
	}

	if err := CheckApiResp(resp.Body()); err != nil {
		return nil, err
	}

	list := gjson.GetBytes(resp.Body(), "data.list")
	return parseList(&list)
}

func parseList(list *gjson.Result) (*List, error) {
	if !list.Exists() {
		return nil, fmt.Errorf("the list doesn't exist")
	}
	id_str := list.Get("id_str")
	member_count := list.Get("member_count")
	name := list.Get("name")

	result := List{}
	result.Id = id_str.Uint()
	result.MemberCount = int(member_count.Int())
	result.Name = html.UnescapeString(name.String())

	user_results := list.Get("user_results")
	if user_results.Exists() {
		if creator, _, err := parseUserResults(&user_results); err == nil {
			result.Creator = creator
		}
	}

	return &result, nil
}

func itemContentsToUsers(itemContents []gjson.Result) MembersResult {
	result := MembersResult{
		Users: make([]*User, 0, len(itemContents)),
	}
	for _, ic := range itemContents {
		user_results, err := getResults(ic, timelineUser)
		if err != nil {
			log.Debugln("[twitter] GetResults(timelineUser) failed:", err)
			continue
		}
		if user_results.String() == "{}" {
			continue
		}
		u, _, err := parseUserResults(&user_results)
		if err != nil {
			log.Debugln("[twitter] ParseUserResults failed:", err)
			continue
		}
		if u != nil {
			result.Users = append(result.Users, u)
		}
	}
	return result
}

func getMembers(ctx context.Context, client *resty.Client, api timelineApi, instsPath string) (*MembersResult, error) {
	api.SetCursor("")
	itemContents, err := getTimelineItemContentsTillEnd(ctx, api, client, instsPath)
	if err != nil {
		return nil, err
	}
	result := itemContentsToUsers(itemContents)
	return &result, nil
}

func (list *List) GetMembers(ctx context.Context, client *resty.Client) (*MembersResult, error) {
	api := listMembers{}
	api.count = 200
	api.id = list.Id
	return getMembers(ctx, client, &api, "data.list.members_timeline.timeline.instructions")
}

func (list *List) GetId() int64 {
	return int64(list.Id)
}

func (list *List) Title() string {
	return fmt.Sprintf("%s(%d)", list.Name, list.Id)
}

type UserFollowing struct {
	creator *User
}

func (fo UserFollowing) GetMembers(ctx context.Context, client *resty.Client) (*MembersResult, error) {
	api := following{}
	api.count = 200
	api.uid = fo.creator.Id
	return getMembers(ctx, client, &api, "data.user.result.timeline.timeline.instructions")
}

func (fo UserFollowing) GetId() int64 {
	return -int64(fo.creator.Id)
}

func (fo UserFollowing) Title() string {
	name := fmt.Sprintf("%s's Following", fo.creator.ScreenName)
	return name
}
