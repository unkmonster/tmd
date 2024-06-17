package twitter

import (
	"log"

	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
	"github.com/unkmonster/tmd2/internal/utils"
)

type ListBase interface {
	GetMembers() ([]*User, error)
}

type List struct {
	Id          uint64
	MemberCount int
	Name        string
	Creator     *User
}

func GetLst(client *resty.Client, id uint64) (*List, error) {
	api := ListByRestId{}
	api.id = id
	url := makeUrl(&api)

	resp, err := client.R().Get(url)
	if err != nil {
		return nil, err
	}

	err = utils.CheckRespStatus(resp)
	if err != nil {
		return nil, err
	}

	list := gjson.Get(resp.String(), "data.list")
	return parseList(&list)
}

func parseList(list *gjson.Result) (*List, error) {
	user_results := list.Get("user_results")
	creator, err := parseUserResults(&user_results)
	if err != nil {
		return nil, err
	}
	id_str := list.Get("id_str")
	member_count := list.Get("member_count")
	name := list.Get("name")

	result := List{}
	result.Creator = creator
	result.Id = id_str.Uint()
	result.MemberCount = int(member_count.Int())
	result.Name = name.String()
	return &result, nil
}

func getMembersOnePage(client *resty.Client, api timelineApi, instsPath string) ([]*User, string, error) {
	data, err := getTimeline(client, api)
	if err != nil {
		return nil, "", err
	}

	temp := gjson.Get(data, instsPath)
	insts := itemInstructions{&temp}
	entries := insts.GetEntries()
	itemContents := entries.GetItemContents()

	users := make([]*User, 0, len(itemContents))
	for _, ic := range itemContents {
		user_results := ic.GetUserResults()
		u, err := parseUserResults(&user_results)
		if err != nil {
			log.Printf("%v\n%v", err, user_results)
			continue
		}
		users = append(users, u)
	}
	return users, entries.getBottomCursor(), nil
}

func getMembers(client *resty.Client, api timelineApi, instsPath string) ([]*User, error) {
	cursor := ""
	users := []*User{}
	for {
		api.SetCursor(cursor)
		currentUsers, next, err := getMembersOnePage(client, api, instsPath)
		if err != nil {
			return nil, err
		}
		users = append(users, currentUsers...)
		if next == "" {
			return users, nil
		}
		cursor = next
	}
}

func (list *List) GetMembers(client *resty.Client) ([]*User, error) {
	api := ListMembers{}
	api.count = 200
	api.id = list.Id
	return getMembers(client, &api, "data.list.members_timeline.timeline.instructions")
}

type UserFollowing struct {
	creator *User
}

func (fo UserFollowing) GetMembers(client *resty.Client) ([]*User, error) {
	api := Following{}
	api.count = 200
	api.uid = fo.creator.Id
	return getMembers(client, &api, "data.user.result.timeline.timeline.instructions")
}
