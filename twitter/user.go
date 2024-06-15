package twitter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

type FollowState int

const (
	FS_UNFOLLOW FollowState = iota
	FS_FOLLOWING
	FS_REQUESTED
)

type User struct {
	Id           uint64
	Name         string
	ScreenName   string
	IsProtected  bool
	FriendsCount int
	Followstate  FollowState
}

func GetUserById(client *http.Client, id uint64) (*User, error) {
	api := userByRestId{id}
	getUrl := makeUrl(&api)
	r, err := getUser(client, getUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get user [%d]: %v", id, err)
	}
	return r, err
}

func GetUserByScreenName(client *http.Client, screenName string) (*User, error) {
	u := makeUrl(&userByScreenName{screenName: screenName})
	r, err := getUser(client, u)
	if err != nil {
		return nil, fmt.Errorf("failed to get user [%s]: %v", screenName, err)
	}
	return r, err
}

func getUser(client *http.Client, url string) (*User, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%d %s", resp.StatusCode, data)
	}

	var j interface{}
	if err = json.Unmarshal(data, &j); err != nil {
		return nil, err
	}
	return parseRespJson(j.(map[string]interface{}))
}

func parseRespJson(resp map[string]interface{}) (*User, error) {
	data := resp["data"].(map[string]interface{})
	if data["user"] == nil {
		return nil, fmt.Errorf("user does not exist")
	}
	user := data["user"].(map[string]interface{})
	result := user["result"].(map[string]interface{})
	if result["__typename"] == nil {
		return nil, fmt.Errorf("user unavaiable")
	}
	legacy := result["legacy"].(map[string]interface{})

	id, err := strconv.ParseUint(result["rest_id"].(string), 10, 64)
	if err != nil {
		return nil, err
	}

	name := legacy["name"].(string)
	screen_name := legacy["screen_name"].(string)
	friends_count := int(legacy["friends_count"].(float64))
	protected := legacy["protected"] != nil
	var followState FollowState
	if legacy["following"] != nil && legacy["following"].(bool) {
		followState = FS_FOLLOWING
	} else if legacy["follow_request_sent"] != nil && legacy["follow_request_sent"].(bool) {
		followState = FS_REQUESTED
	} else {
		followState = FS_UNFOLLOW
	}

	usr := User{}
	usr.FriendsCount = friends_count
	usr.Id = id
	usr.IsProtected = protected
	usr.Name = name
	usr.ScreenName = screen_name
	usr.Followstate = followState
	return &usr, nil
}

func (u *User) IsVisiable() bool {
	return u.Followstate == FS_FOLLOWING || !u.IsProtected
}
