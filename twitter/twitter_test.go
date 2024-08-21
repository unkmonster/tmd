package twitter

import (
	"context"
	"encoding/json"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/unkmonster/tmd2/internal/utils"
)

var client *resty.Client
var someUsers = []struct {
	id         uint64
	screenName string
}{
	{id: 1528902077325332480},
	{id: 3316272504},
	{id: 1478962175947390976},
	{screenName: "Chibubao01"},
	{screenName: "Tsumugi69458619"},
	{screenName: "_sosen_"},
	{screenName: "baobaoxqaq"},
	{screenName: "Greenfish_insky"},
}
var someLists = []uint64{
	1293998605938950144,
	1073356376045436928,
	1360265344439443460,
}

func init() {
	var err error
	ctx := context.Background()
	client, _, err = Login(ctx, os.Getenv("AUTH_TOKEN"), os.Getenv("CT0"))
	if err != nil {
		panic(err)
	}
	//EnableRateLimit(client)
}

func TestGetUser(t *testing.T) {
	ctx := context.Background()

	for _, test := range someUsers {
		var u *User = nil
		var err error
		if test.id != 0 {
			u, err = GetUserById(ctx, client, test.id)
		} else {
			u, err = GetUserByScreenName(ctx, client, test.screenName)
		}
		if err != nil {
			t.Error("failed to get user:", err)
			continue
		}

		if test.id != 0 && u.Id != test.id {
			t.Errorf("user.id = %d, want %d", u.Id, test.id)
		} else if test.id == 0 && u.ScreenName != test.screenName {
			t.Errorf("screen_name = %s, want %s", u.ScreenName, test.screenName)
		}

		// report
		j, err := json.MarshalIndent(u, "", "  ")
		if err != nil {
			t.Error(err)
		}
		t.Logf("%s\n", j)
	}
}

func TestGetMedia(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	makeMinMax := func(items []*Tweet) (min int, max int) {
		// 时间线是逆序的，所以更大的索引对应发布时间更早的推文
		for {
			r1 := r.Intn(len(items))
			r2 := r.Intn(len(items))
			if r1 > r2 {
				max = r2
				min = r1

			} else {
				max = r1
				min = r2
			}

			// 不要生成在边界
			if max != 0 && min != len(items)-1 {
				return
			}
		}
	}

	wg := sync.WaitGroup{}
	ctx := context.Background()

	routine := func(test string) {
		defer wg.Done()
		usr, err := GetUserByScreenName(ctx, client, test)
		if err != nil {
			t.Error(err)
			return
		}

		// 获取全部推文
		if !usr.IsVisiable() {
			//t.Errorf("%s is invisiable", test)
			return
		}
		tweets, err := usr.GetMeidas(ctx, client, nil)
		if err != nil {
			t.Error(err, usr)
			return
		}
		//t.Logf("[user:%s] tweets: %d\n", usr.ScreenName, len(tweets))
		if len(tweets) > usr.MediaCount || usr.MediaCount-len(tweets) > usr.MediaCount*2/100 {
			t.Errorf("%s: len(tweets) == %d, want %d", test, len(tweets), usr.MediaCount)
		}

		if len(tweets) == 0 {
			return
		}

		// 区间测试
		minIndex, maxIndex := makeMinMax(tweets)
		tr := &utils.TimeRange{Min: tweets[minIndex+1].CreatedAt, Max: tweets[maxIndex-1].CreatedAt}
		rangedTweets, err := usr.GetMeidas(ctx, client, tr)
		if err != nil {
			t.Error(err, usr, "range")
			return
		}

		if rangedTweets[0].Id != tweets[maxIndex].Id {
			t.Errorf("rangedTweets[0].Id = %d, want %d", rangedTweets[0].Id, tweets[maxIndex].Id)
		}
		if rangedTweets[len(rangedTweets)-1].Id != tweets[minIndex].Id {
			t.Errorf("rangedTweets[-1].Id = %d, want %d", rangedTweets[len(rangedTweets)-1].Id, tweets[minIndex].Id)
		}
	}

	for _, user := range someUsers {
		if user.id != 0 {
			continue
		}

		wg.Add(1)
		go routine(user.screenName)
	}

	wg.Wait()
}

func TestGetList(t *testing.T) {
	ctx := context.Background()

	for _, test := range someLists {
		lst, err := GetLst(ctx, client, test)
		if err != nil {
			t.Error(err)
			continue
		}

		if lst.Id != test {
			t.Errorf("lst.id == %d, want %d", lst.Id, test)
		}

		j, err := json.MarshalIndent(&lst, "", "    ")
		if err != nil {
			t.Error(err)
		}
		t.Logf("%s\n", j)
	}
}

func TestGetMember(t *testing.T) {
	wg := sync.WaitGroup{}
	ctx := context.Background()

	routine := func(test uint64) {
		defer wg.Done()

		lst, err := GetLst(ctx, client, test)
		if err != nil {
			t.Error(err)
			return
		}

		users, err := lst.GetMembers(ctx, client)
		if err != nil {
			t.Error(err)
			return
		}
		if len(users) > lst.MemberCount || lst.MemberCount-len(users) > lst.MemberCount*2/100 {
			t.Errorf("len(users) == %d, want %d", len(users), lst.MemberCount)
		}

		// following
		fo := UserFollowing{lst.Creator}
		users, err = fo.GetMembers(ctx, client)
		if err != nil {
			t.Error(err)
			return
		}
		//t.Logf("usr %s following count: %d\n", fo.creator.Title(), len(users))
		if len(users) > fo.creator.FriendsCount || fo.creator.FriendsCount-len(users) > fo.creator.FriendsCount*2/100 {
			t.Errorf("len(users) == %d, want %d", len(users), fo.creator.FriendsCount)
		}
	}

	for _, test := range someLists {
		wg.Add(1)
		go routine(test)
	}

	wg.Wait()
}

// func TestRateLimit(t *testing.T) {
// 	oCount := client.RetryCount
// 	client.SetRetryCount(20)
// 	defer client.SetRetryCount(oCount)

// 	sn := GetClientScreenName(client)
// 	if sn == "" {
// 		panic("screen_name is empty")
// 	}

// 	api := userByScreenName{}
// 	api.screenName = sn
// 	url := makeUrl(&api)
// 	resp, err := client.R().Get(url)
// 	if err != nil {
// 		t.Error(err)
// 		return
// 	}
// 	ratelimit := makeRateLimit(resp)
// 	if ratelimit == nil {
// 		panic("invalid xRateLimit")
// 	}

// 	success := &atomic.Int32{}
// 	success.Store(0)

// 	wg := sync.WaitGroup{}
// 	ctx, cancel := context.WithCancel(context.Background())
// 	done := make(chan struct{})

// 	routine := func() {
// 		defer wg.Done()
// 		_, err := GetUserByScreenName(client, sn)
// 		if err != nil {
// 			t.Error("[TestRateLimit]", err)
// 			cancel()
// 		}
// 	}

// 	// 确保休眠一次
// 	for i := 0; i < ratelimit.Remaining+1; i++ {
// 		wg.Add(1)
// 		go routine()
// 	}

// 	go func() {
// 		wg.Wait()
// 		done <- struct{}{}
// 	}()

// 	select {
// 	case <-done:
// 	case <-ctx.Done():
// 		return
// 	}

// 	// 如果正常完成休眠，这次请求不会出错
// 	if _, err := GetUserByScreenName(client, sn); err != nil {
// 		t.Error("get after blocking", err)
// 		return
// 	}
// }
