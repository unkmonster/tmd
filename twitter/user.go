package twitter

import (
	"context"
	"fmt"
	"time"

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
	Muting       bool
	Blocking     bool
}

func GetUserById(ctx context.Context, client *resty.Client, id uint64) (*User, error) {
	api := userByRestId{id}
	getUrl := makeUrl(&api)
	r, err := getUser(ctx, client, getUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get user [%d]: %v", id, err)
	}
	return r, err
}

func GetUserByScreenName(ctx context.Context, client *resty.Client, screenName string) (*User, error) {
	u := makeUrl(&userByScreenName{screenName: screenName})
	r, err := getUser(ctx, client, u)
	if err != nil {
		return nil, fmt.Errorf("failed to get user [%s]: %v", screenName, err)
	}
	return r, err
}

func getUser(ctx context.Context, client *resty.Client, url string) (*User, error) {
	resp, err := client.R().SetContext(ctx).Get(url)
	if err != nil {
		return nil, err
	}
	if err := utils.CheckRespStatus(resp); err != nil {
		return nil, err
	}
	return parseRespJson(resp.Body())
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
	muting := legacy.Get("muting")
	blocking := legacy.Get("blocking")

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
	usr.Muting = muting.Exists() && muting.Bool()
	usr.Blocking = blocking.Exists() && blocking.Bool()
	return &usr, nil
}

func parseRespJson(resp []byte) (*User, error) {
	user := gjson.GetBytes(resp, "data.user")
	if !user.Exists() {
		return nil, fmt.Errorf("user does not exist")
	}
	return parseUserResults(&user)
}

func (u *User) IsVisiable() bool {
	return u.Followstate == FS_FOLLOWING || !u.IsProtected
}

func itemContentsToTweets(itemContents []gjson.Result) []*Tweet {
	res := make([]*Tweet, 0, len(itemContents))
	for _, itemContent := range itemContents {
		tweetResults := getResults(itemContent, timelineTweet)
		if tw := parseTweetResults(&tweetResults); tw != nil {
			res = append(res, tw)
		}
	}
	return res
}

func (u *User) getMediasOnePage(ctx context.Context, api *userMedia, client *resty.Client) ([]*Tweet, string, error) {
	if !u.IsVisiable() {
		return nil, "", nil
	}

	itemContents, next, err := getTimelineItemContents(ctx, api, client, "data.user.result.timeline_v2.timeline.instructions")
	return itemContentsToTweets(itemContents), next, err
}

// 在逆序切片中，筛选出在 timerange 范围内的推文
func filterTweetsByTimeRange(tweets []*Tweet, min *time.Time, max *time.Time) (cutMin bool, cutMax bool, res []*Tweet) {
	n := len(tweets)
	begin, end := 0, n

	// 从左到右查找第一个小于 min 的推文
	if min != nil && !min.IsZero() {
		for i := 0; i < n; i++ {
			if !tweets[i].CreatedAt.After(*min) {
				end = i // 找到第一个不大于 min 的推文位置
				cutMin = true
				break
			}
		}
	}

	// 从右到左查找最后一个大于 max 的推文
	if max != nil && !max.IsZero() {
		for i := n - 1; i >= 0; i-- {
			if !tweets[i].CreatedAt.Before(*max) {
				begin = i + 1 // 找到第一个不小于 max 的推文位置
				cutMax = true
				break
			}
		}
	}

	if begin >= end {
		// 如果最终的范围无效，返回空结果
		return cutMin, cutMax, nil
	}

	res = tweets[begin:end]
	return
}

func (u *User) GetMeidas(ctx context.Context, client *resty.Client, timeRange *utils.TimeRange) ([]*Tweet, error) {
	if !u.IsVisiable() {
		return nil, nil
	}

	api := userMedia{}
	api.count = 100
	api.cursor = ""
	api.userId = u.Id

	results := make([]*Tweet, 0)

	var minTime *time.Time
	var maxTime *time.Time

	if timeRange != nil {
		minTime = &timeRange.Min
		maxTime = &timeRange.Max
	}

	for {
		currentTweets, next, err := u.getMediasOnePage(ctx, &api, client)
		if err != nil {
			return nil, err
		}

		if len(currentTweets) == 0 {
			break // empty page
		}

		api.SetCursor(next)

		if timeRange == nil {
			results = append(results, currentTweets...)
			continue
		}

		// 筛选推文，并判断是否获取下页
		cutMin, cutMax, currentTweets := filterTweetsByTimeRange(currentTweets, minTime, maxTime)
		results = append(results, currentTweets...)

		if cutMin {
			break
		}
		if cutMax && len(currentTweets) != 0 {
			maxTime = nil
		}
	}
	return results, nil
}

func (u *User) Title() string {
	return fmt.Sprintf("%s(%s)", u.Name, u.ScreenName)
}

func (u *User) Following() UserFollowing {
	return UserFollowing{u}
}
