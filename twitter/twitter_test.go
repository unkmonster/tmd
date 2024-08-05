package twitter

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/unkmonster/tmd2/internal/utils"
)

var client *resty.Client

func init() {
	var err error
	client, _, err = Login(os.Getenv("AUTH_TOKEN"), os.Getenv("CT0"))
	if err != nil {
		panic(err)
	}
	EnableRateLimit(client)
}

func TestGetUser(t *testing.T) {
	tests := []struct {
		Id    uint64
		Sname string
	}{
		{Id: 1528902077325332480},
		{Id: 3316272504},
		{Id: 1478962175947390976},
		{Sname: "Chibubao01"},
		{Sname: "Tsumugi69458619"},
		{Sname: "_sosen_"},
	}

	// real test
	for _, test := range tests {
		var u *User = nil
		var err error
		if test.Id != 0 {
			u, err = GetUserById(client, test.Id)
		} else {
			u, err = GetUserByScreenName(client, test.Sname)
		}
		if err != nil {
			t.Error("failed to get user:", err)
			continue
		}

		if test.Id != 0 && u.Id != test.Id {
			t.Errorf("user.id = %d, want %d", u.Id, test.Id)
		} else if test.Id == 0 && u.ScreenName != test.Sname {
			t.Errorf("screen_name = %s, want %s", u.ScreenName, test.Sname)
		}

		// report
		j, err := json.MarshalIndent(u, "", "  ")
		if err != nil {
			t.Error(err)
		}
		fmt.Printf("%s\n", j)
	}
}

func TestGetMedia(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	makeMinMax := func(items []*Tweet) (min int, max int) {
		r1 := r.Intn(len(items))
		r2 := r.Intn(len(items))
		if r1 > r2 {
			max = r2
			min = r1

		} else {
			max = r1
			min = r2
		}
		return
	}

	tests := []string{
		"baobaoxqaq",
		"Greenfish_insky",
		"_sosen_",
	}

	for _, test := range tests {
		usr, err := GetUserByScreenName(client, test)
		if err != nil {
			t.Error(err)
			continue
		}

		tweets, err := usr.GetMeidas(client, nil)
		if err != nil {
			t.Error(err, usr)
			continue
		}
		t.Logf("[user:%s] tweets: %d\n", usr.ScreenName, len(tweets))
		if len(tweets) != usr.MediaCount {
			t.Errorf("len(tweets) == %d, want %d", len(tweets), usr.MediaCount)
		}

		if !usr.IsVisiable() || len(tweets) == 0 {
			continue
		}

		// 区间测试
		minIndex, maxIndex := makeMinMax(tweets)
		tr := &utils.TimeRange{Min: tweets[minIndex].CreatedAt.Add(-time.Second), Max: tweets[maxIndex].CreatedAt.Add(time.Second)}
		rangedTweets, err := usr.GetMeidas(client, tr)
		if err != nil {
			t.Error(err, usr, "range")
			continue
		}

		if !rangedTweets[len(rangedTweets)-1].CreatedAt.Equal(tweets[minIndex].CreatedAt) {
			t.Errorf("!rangedTweets[len(rangedTweets)-1].CreatedAt.Equal(tweets[minIndex].CreatedAt) %v != %v", rangedTweets[len(rangedTweets)-1].CreatedAt, tweets[minIndex].CreatedAt)
		}
		if !rangedTweets[0].CreatedAt.Equal(tweets[maxIndex].CreatedAt) {
			t.Errorf("!rangedTweets[0].CreatedAt.Equal(tweets[maxIndex].CreatedAt) %v != %v", rangedTweets[0].CreatedAt, tweets[maxIndex].CreatedAt)
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
		fmt.Printf("%s\n", j)
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
		if math.Abs(float64(len(users)-lst.MemberCount)) > float64(len(users)*2/100) {
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
		if math.Abs(float64(len(users)-fo.creator.FriendsCount)) > float64(len(users)*2/100) {
			t.Errorf("len(users) == %d, want %d", len(users), fo.creator.FriendsCount)
		}
	}
}
