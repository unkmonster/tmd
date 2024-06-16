package twitter

import (
	"io"
	"net/http"

	"github.com/tidwall/gjson"
	"github.com/unkmonster/tmd2/internal/utils"
)

type List struct {
	Id          uint64
	MemberCount int
	Name        string
	Creator     *User
}

func GetLst(client *http.Client, id uint64) (*List, error) {
	api := ListByRestId{}
	api.id = id
	url := makeUrl(&api)

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	err = utils.CheckRespStatus(resp)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	list := gjson.Get(string(data), "data.list")
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
