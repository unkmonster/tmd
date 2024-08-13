package twitter

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
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

func init() {
	var err error
	client, _, err = Login(os.Getenv("AUTH_TOKEN"), os.Getenv("CT0"))
	if err != nil {
		panic(err)
	}
	EnableRateLimit(client)
}

func TestGetUser(t *testing.T) {
	for _, test := range someUsers {
		var u *User = nil
		var err error
		if test.id != 0 {
			u, err = GetUserById(client, test.id)
		} else {
			u, err = GetUserByScreenName(client, test.screenName)
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

	for _, user := range someUsers {
		if user.id != 0 {
			continue
		}
		test := user.screenName

		usr, err := GetUserByScreenName(client, test)
		if err != nil {
			t.Error(err)
			continue
		}

		// 获取全部推文
		if !usr.IsVisiable() {
			t.Errorf("%s is invisiable", test)
			continue
		}
		tweets, err := usr.GetMeidas(client, nil)
		if err != nil {
			t.Error(err, usr)
			continue
		}
		//t.Logf("[user:%s] tweets: %d\n", usr.ScreenName, len(tweets))
		if math.Abs(float64(len(tweets)-usr.MediaCount)) > float64(usr.MediaCount*2/100) {
			t.Errorf("%s: len(tweets) == %d, want %d", test, len(tweets), usr.MediaCount)
		}

		if len(tweets) == 0 {
			continue
		}

		// 区间测试
		minIndex, maxIndex := makeMinMax(tweets)
		tr := &utils.TimeRange{Min: tweets[minIndex+1].CreatedAt, Max: tweets[maxIndex-1].CreatedAt}
		rangedTweets, err := usr.GetMeidas(client, tr)
		if err != nil {
			t.Error(err, usr, "range")
			continue
		}

		if rangedTweets[0].Id != tweets[maxIndex].Id {
			t.Errorf("rangedTweets[0].Id = %d, want %d", rangedTweets[0].Id, tweets[maxIndex].Id)
		}
		if rangedTweets[len(rangedTweets)-1].Id != tweets[minIndex].Id {
			t.Errorf("rangedTweets[-1].Id = %d, want %d", rangedTweets[len(rangedTweets)-1].Id, tweets[minIndex].Id)
		}
	}
}

var someLists = []uint64{
	1293998605938950144,
	1073356376045436928,
	1360265344439443460,
}

func TestGetList(t *testing.T) {
	for _, test := range someLists {
		lst, err := GetLst(client, test)
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
	for _, test := range someLists {
		lst, err := GetLst(client, test)
		if err != nil {
			t.Error(err)
			continue
		}

		users, err := lst.GetMembers(client)
		if err != nil {
			t.Error(err)
			continue
		}
		if math.Abs(float64(len(users)-lst.MemberCount)) > float64(lst.MemberCount*2/100) {
			t.Errorf("len(users) == %d, want %d", len(users), lst.MemberCount)
		}

		// following
		fo := UserFollowing{lst.Creator}
		users, err = fo.GetMembers(client)
		if err != nil {
			t.Error(err)
			continue
		}
		//t.Logf("usr %s following count: %d\n", fo.creator.Title(), len(users))
		if math.Abs(float64(len(users)-fo.creator.FriendsCount)) > float64(fo.creator.FriendsCount*2/100) {
			t.Errorf("len(users) == %d, want %d", len(users), fo.creator.FriendsCount)
		}
	}
}
