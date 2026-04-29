package twitter

import (
	"context"
	"fmt"
	"html"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
	"github.com/unkmonster/tmd/internal/utils"
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
	AvatarURL    string // 头像URL
	BannerURL    string // 横幅URL
	Description  string // 用户简介
	Location     string // 位置
	URL          string // 个人链接
	Verified     bool   // 是否认证
	CreatedAt    string // 账号创建时间
}

func GetUserById(ctx context.Context, client *resty.Client, id uint64) (*User, uint64, error) {
	api := userByRestId{id}
	getUrl := makeUrl(&api)
	r, uid, err := getUser(ctx, client, getUrl)
	if err != nil {
		// 返回 uid（可能是从 UserUnavailable 中提取的ID）用于标记不可访问状态
		return nil, uid, fmt.Errorf("failed to get user [%d]: %v", id, err)
	}
	return r, uid, err
}

func GetUserByScreenName(ctx context.Context, client *resty.Client, screenName string) (*User, uint64, error) {
	u := makeUrl(&userByScreenName{screenName: screenName})
	r, uid, err := getUser(ctx, client, u)
	if err != nil {
		// 注意：通过 screen_name 查询时，UserUnavailable 响应不包含 rest_id
		// 所以 uid 可能为 0，调用方需要检查
		return nil, uid, fmt.Errorf("failed to get user [%s]: %v", screenName, err)
	}
	return r, uid, err
}

func getUser(ctx context.Context, client *resty.Client, url string) (*User, uint64, error) {
	resp, err := client.R().SetContext(ctx).Get(url)
	if err != nil {
		return nil, 0, err
	}
	if err := CheckApiResp(resp.Body()); err != nil {
		return nil, 0, err
	}
	user := gjson.GetBytes(resp.Body(), "data.user")
	if !user.Exists() {
		return nil, 0, fmt.Errorf("user does not exist or is not accessible")
	}
	return parseUserResults(&user)
}

func parseUserResults(user_results *gjson.Result) (*User, uint64, error) {
	result := user_results.Get("result")
	if !result.Exists() {
		return nil, 0, fmt.Errorf("user result does not exist")
	}
	if result.Get("__typename").String() == "UserUnavailable" {
		// 返回不可访问用户的 ID，用于标记状态
		if restId := result.Get("rest_id"); restId.Exists() {
			log.Debugf("UserUnavailable detected, rest_id: %s", restId.String())
			return nil, restId.Uint(), fmt.Errorf("user unavaiable")
		}
		// 尝试从其他字段获取ID
		log.Debugf("UserUnavailable result: %s", result.String())
		return nil, 0, fmt.Errorf("user unavaiable")
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
	usr.Name = html.UnescapeString(name.String())
	usr.ScreenName = screen_name.String()
	usr.MediaCount = int(media_count.Int())
	usr.Muting = muting.Exists() && muting.Bool()
	usr.Blocking = blocking.Exists() && blocking.Bool()

	// 从API响应中提取头像和横幅URL
	if avatar := result.Get("avatar.image_url"); avatar.Exists() && avatar.String() != "" {
		usr.AvatarURL = avatar.String()
	} else if avatar := legacy.Get("profile_image_url_https"); avatar.Exists() && avatar.String() != "" {
		usr.AvatarURL = avatar.String()
	} else if avatar := legacy.Get("profile_image_url"); avatar.Exists() && avatar.String() != "" {
		usr.AvatarURL = avatar.String()
	}

	if banner := legacy.Get("profile_banner_url"); banner.Exists() {
		usr.BannerURL = banner.String()
	}

	// 提取用户简介和其他信息
	if desc := legacy.Get("description"); desc.Exists() {
		usr.Description = html.UnescapeString(desc.String())
	}
	if loc := legacy.Get("location"); loc.Exists() {
		usr.Location = html.UnescapeString(loc.String())
	}
	if url := legacy.Get("url"); url.Exists() {
		usr.URL = url.String()
	}
	if verified := legacy.Get("verified"); verified.Exists() {
		usr.Verified = verified.Bool()
	}
	if created := legacy.Get("created_at"); created.Exists() {
		usr.CreatedAt = created.String()
	}

	return &usr, usr.Id, nil
}

func (u *User) IsVisiable() bool {
	return u.Followstate == FS_FOLLOWING || !u.IsProtected
}

func itemContentsToTweets(itemContents []gjson.Result) []*Tweet {
	res := make([]*Tweet, 0, len(itemContents))
	for _, itemContent := range itemContents {
		tweetResults, err := getResults(itemContent, timelineTweet)
		if err != nil {
			log.Debugln("getResults failed:", err)
			continue
		}
		tw, err := parseTweetResults(&tweetResults)
		if err != nil {
			log.Debugln("parseTweetResults failed:", err)
			continue
		}
		if tw != nil {
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

func (u *User) GetMedias(ctx context.Context, client *resty.Client, timeRange *utils.TimeRange) ([]*Tweet, error) {
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

func FollowUser(ctx context.Context, client *resty.Client, user *User) error {
	url := "https://x.com/i/api/1.1/friendships/create.json"
	_, err := client.R().SetFormData(map[string]string{
		"user_id": fmt.Sprintf("%d", user.Id),
	}).SetContext(ctx).Post(url)
	return err
}
