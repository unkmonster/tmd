package twitter

import (
	"fmt"
	"io"
	"net/http"

	"github.com/tidwall/gjson"
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

	return parseRespJson(string(data))
}

func parseUserResults(user_results *gjson.Result) (*User, error) {
	result := user_results.Get("result")
	if result.Get("__typename").String() == "UserUnavailable" {
		return nil, fmt.Errorf("user unavaiable")
	}
	legacy := result.Get("legacy")

	restId := result.Get("rest_id")
	friends_count := legacy.Get("friends_count")
	name := legacy.Get("name")
	screen_name := legacy.Get("screen_name")
	protected := legacy.Get("protected").Exists()

	usr := User{}
	if legacy.Get("following").Exists() {
		usr.Followstate = FS_FOLLOWING
	} else if legacy.Get("follow_request_sent").Exists() {
		usr.Followstate = FS_REQUESTED
	} else {
		usr.Followstate = FS_UNFOLLOW
	}
	usr.FriendsCount = int(friends_count.Int())
	usr.Id = restId.Uint()
	usr.IsProtected = protected
	usr.Name = name.String()
	usr.ScreenName = screen_name.String()
	return &usr, nil
}

func parseRespJson(resp string) (*User, error) {
	user := gjson.Get(resp, "data.user")
	if !user.Exists() {
		return nil, fmt.Errorf("user does not exist")
	}
	return parseUserResults(&user)
}

func (u *User) IsVisiable() bool {
	return u.Followstate == FS_FOLLOWING || !u.IsProtected
}
