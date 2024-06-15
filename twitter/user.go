package twitter

import (
	"fmt"
	"io"
	"net/http"

	"github.com/tidwall/gjson"
	"github.com/unkmonster/tmd2/internal/utils"
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
	MediaCount   int
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
	media_count := legacy.Get("media_count")

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
	usr.MediaCount = int(media_count.Int())
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

func (u *User) getMediasOnPage(client *http.Client, cursor string) ([]*Tweet, string, error) {
	if !u.IsVisiable() {
		return nil, "", nil
	}
	api := userMedia{}
	api.count = 100
	api.cursor = cursor
	api.userId = u.Id

	resp, err := getTimeline(client, &api)
	if err != nil {
		return nil, "", err
	}

	j := gjson.Get(resp, "data.user.result.timeline_v2.timeline.instructions")
	minsts := moduleInstructions{itemInstructions{&j}}
	entries := minsts.GetEntries()
	itemContents := entries.GetItemContents()
	tweets := make([]*Tweet, len(itemContents))
	for i := 0; i < len(itemContents); i++ {
		tweet_results := itemContents[i].GetTweetResults()
		ptweet := parseTweetResults(&tweet_results)
		if ptweet != nil {
			tweets[i] = ptweet
		}
	}
	if len(itemContents) == 0 {
		return nil, "", nil
	}
	return tweets, entries.getBottomCursor(), nil
}

func (u *User) GetMeidas(client *http.Client, trange *utils.TimeRange) ([]*Tweet, error) {
	if !u.IsVisiable() {
		return nil, nil
	}
	var cursor string
	var done bool
	var firstPage bool = true
	results := make([]*Tweet, 0, u.MediaCount)

	// temp
	var tempMin Tweet
	var tempMax Tweet
	if trange != nil {
		tempMin = Tweet{CreatedAt: trange.Min}
		tempMax = Tweet{CreatedAt: trange.Max}
	}

	for !done {
		currentTweets, next, err := u.getMediasOnPage(client, cursor)
		if err != nil {
			return nil, err
		}
		if next == "" {
			done = true
		} else {
			cursor = next
		}

		if trange == nil {
			results = append(results, currentTweets...)
			continue
		}

		// ? 这样就行？
		comparables := make([]utils.Comparable, len(currentTweets))
		for i := 0; i < len(currentTweets); i++ {
			comparables[i] = currentTweets[i]
		}

		// filter
		// 过滤掉发布日期超出指定范围的推文，仅需在第一页执行
		if firstPage && !trange.Max.IsZero() && len(currentTweets) != 0 {
			firstPage = false
			index := utils.RFirstGreaterEqual(comparables, &tempMax)
			if index >= 0 {
				currentTweets = currentTweets[index+1:]
				comparables = comparables[index+1:]
			}
		}
		// 过滤掉发布日期小于等于指定范围的推文，
		// 当发现满足条件的推文，返回不再获取下页
		if !trange.Min.IsZero() && len(currentTweets) != 0 {
			index := utils.RFirstLessEqual(comparables, &tempMin)
			if index < len(currentTweets) {
				done = true
				currentTweets = currentTweets[:index]
			}
		}
		results = append(results, currentTweets...)
	}
	return results, nil
}
