package twitter

import (
	"fmt"

	"github.com/go-resty/resty/v2"
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

func GetUserById(client *resty.Client, id uint64) (*User, error) {
	api := userByRestId{id}
	getUrl := makeUrl(&api)
	r, err := getUser(client, getUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get user [%d]: %v", id, err)
	}
	return r, err
}

func GetUserByScreenName(client *resty.Client, screenName string) (*User, error) {
	u := makeUrl(&userByScreenName{screenName: screenName})
	r, err := getUser(client, u)
	if err != nil {
		return nil, fmt.Errorf("failed to get user [%s]: %v", screenName, err)
	}
	return r, err
}

func getUser(client *resty.Client, url string) (*User, error) {
	resp, err := client.R().Get(url)
	if err != nil {
		return nil, err
	}
	if err := utils.CheckRespStatus(resp); err != nil {
		return nil, err
	}
	return parseRespJson(resp.String())
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
	protected := legacy.Get("protected").Exists() && legacy.Get("protected").Bool()
	media_count := legacy.Get("media_count")

	usr := User{}
	if foll := legacy.Get("following"); foll.Exists() {
		if foll.Bool() {
			usr.Followstate = FS_FOLLOWING
		} else {
			usr.Followstate = FS_UNFOLLOW
		}
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

func (u *User) getMediasOnPage(client *resty.Client, cursor string) ([]*Tweet, string, error) {
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
	if len(itemContents) == 0 {
		return nil, "", nil
	}

	tweets := make([]*Tweet, 0, len(itemContents))
	for i := 0; i < len(itemContents); i++ {
		tweet_results := itemContents[i].GetTweetResults()
		ptweet := parseTweetResults(&tweet_results)
		if ptweet != nil {
			tweets = append(tweets, ptweet)
		}
	}
	return tweets, entries.getBottomCursor(), nil
}

func (u *User) GetMeidas(client *resty.Client, trange *utils.TimeRange) ([]*Tweet, error) {
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

func (u *User) Title() string {
	return fmt.Sprintf("%s(%s)", u.Name, u.ScreenName)
}

func (u *User) Following() UserFollowing {
	return UserFollowing{u}
}
