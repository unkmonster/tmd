package twitter

import (
	"fmt"
	"time"

	"github.com/tidwall/gjson"
	"github.com/unkmonster/tmd2/internal/utils"
)

type Tweet struct {
	Id        uint64
	Text      string
	CreatedAt time.Time
	Creator   *User
	Urls      []string
}

func parseTweetResults(tweet_results *gjson.Result) *Tweet {
	var tweet Tweet
	var err error = nil

	result := tweet_results.Get("result")
	if !result.Exists() || result.Get("__typename").String() == "TweetTombstone" {
		return nil
	}
	if result.Get("__typename").String() == "TweetWithVisibilityResults" {
		result = result.Get("tweet")
	}
	legacy := result.Get("legacy")
	// TODO: 利用 rest_id 重新获取推文信息
	if !legacy.Exists() {
		return nil
	}
	user_results := result.Get("core.user_results")

	tweet.Id = result.Get("rest_id").Uint()
	tweet.Text = legacy.Get("full_text").String()
	tweet.Creator, _ = parseUserResults(&user_results)
	tweet.CreatedAt, err = time.Parse(time.RubyDate, legacy.Get("created_at").String())
	if err != nil {
		panic(fmt.Errorf("invalid time format %v", err))
	}
	media := legacy.Get("extended_entities.media")
	if media.Exists() {
		tweet.Urls = getUrlsFromMedia(&media)
	}
	return &tweet
}

func getUrlsFromMedia(media *gjson.Result) []string {
	results := []string{}
	for _, m := range media.Array() {
		typ := m.Get("type").String()
		if typ == "video" || typ == "animated_gif" {
			results = append(results, m.Get("video_info.variants.@reverse.0.url").String())
		} else if typ == "photo" {
			results = append(results, m.Get("media_url_https").String())
		}
	}
	return results
}

type Tweets []Tweet

func (tweets Tweets) Len() int {
	return len(tweets)
}

func (tweets Tweets) Less(i, j int) bool {
	return tweets[i].CreatedAt.Before(tweets[j].CreatedAt)
}

func (tweets Tweets) Swap(i, j int) {
	tweets[i], tweets[j] = tweets[j], tweets[i]
}

func (tweet *Tweet) LessThan(rhs utils.Comparable) bool {
	val := rhs.(*Tweet).CreatedAt
	return tweet.CreatedAt.Before(val)
}

func (tweet *Tweet) GreaterThan(rhs utils.Comparable) bool {
	val := rhs.(*Tweet).CreatedAt
	return tweet.CreatedAt.After(val)
}
