package downloading

import (
	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	"github.com/unkmonster/tmd2/database"
	"github.com/unkmonster/tmd2/internal/utils"
	"github.com/unkmonster/tmd2/twitter"
)

func syncUser(db *sqlx.DB, user *twitter.User) error {
	renamed := false
	isNew := false
	usrdb, err := database.GetUserById(db, user.Id)
	if err != nil {
		return err
	}

	if usrdb == nil {
		isNew = true
		usrdb = &database.User{}
		usrdb.Id = user.Id
	} else {
		renamed = usrdb.Name != user.Name || usrdb.ScreenName != user.ScreenName
	}

	usrdb.FriendsCount = user.FriendsCount
	usrdb.IsProtected = user.IsProtected
	usrdb.Name = user.Name
	usrdb.ScreenName = user.ScreenName

	if isNew {
		err = database.CreateUser(db, usrdb)
	} else {
		err = database.UpdateUser(db, usrdb)
	}
	if err != nil {
		return err
	}
	if renamed || isNew {
		err = database.RecordUserPreviousName(db, user.Id, user.Name, user.ScreenName)
	}
	return err
}

func getTweetAndUpdateLatestReleaseTime(client *resty.Client, user *twitter.User, entity *UserEntity) ([]*twitter.Tweet, error) {
	tweets, err := user.GetMeidas(client, &utils.TimeRange{Min: entity.LatestReleaseTime()})
	if err != nil || len(tweets) == 0 {
		return nil, err
	}
	if err := entity.SetLatestReleaseTime(tweets[0].CreatedAt); err != nil {
		return nil, err
	}
	return tweets, nil
}

func DownloadUser(db *sqlx.DB, client *resty.Client, user *twitter.User, dir string) ([]PackgedTweet, error) {
	entity, err := syncUserAndEntityInDir(db, user, dir)
	if err != nil {
		return nil, err
	}
	tweets, err := getTweetAndUpdateLatestReleaseTime(client, user, entity)
	if err != nil || len(tweets) == 0 {
		return nil, err
	}

	// 打包推文
	pts := make([]PackgedTweet, 0, len(tweets))
	for _, tw := range tweets {
		pts = append(pts, TweetInEntity{Tweet: tw, Entity: entity})
	}

	failures := batchDownloadTweet(client, pts...)
	return failures, nil
}

func syncUserAndEntityInDir(db *sqlx.DB, user *twitter.User, dir string) (*UserEntity, error) {
	if err := syncUser(db, user); err != nil {
		return nil, err
	}
	expectedTitle := string(utils.WinFileName([]byte(user.Title())))

	entity := NewUserEntityByParentDir(db, user.Id, dir)
	err := syncPath(entity, expectedTitle)
	if err != nil {
		return nil, err
	}
	return entity, nil
}
