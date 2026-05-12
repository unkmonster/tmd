package downloading

import (
	"context"
	"sync/atomic"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/unkmonster/tmd/internal/downloader"
)

type RetryProgress struct {
	Total     int
	Completed int
	Failed    int
}

type RetryProgressFunc func(progress RetryProgress)

type RetrySummary struct {
	TotalEntities     int
	RemainingEntities int
}

func RetryFailedTweets(ctx context.Context, dumper *TweetDumper, db *sqlx.DB, client *resty.Client, dwn downloader.Downloader, fileWriter downloader.FileWriter, progress RetryProgressFunc) (RetrySummary, error) {
	if dumper.Count() == 0 {
		return RetrySummary{}, nil
	}
	totalEntities := dumper.EntityCount()

	log.Infoln("starting to retry failed tweets")
	legacy, err := dumper.GetTotal(db)
	if err != nil {
		return RetrySummary{}, err
	}

	toretry := make([]PackagedTweet, 0, len(legacy))
	for _, leg := range legacy {
		if leg.Tweet == nil {
			continue
		}
		if len(leg.Tweet.Urls) > 0 {
			toretry = append(toretry, leg)
		}
	}

	if len(toretry) == 0 {
		log.Infoln("no tweets need to be retried")
		dumper.Clear()
		return RetrySummary{}, nil
	}

	log.Infof("retrying %d tweets with %d total media(s)", len(toretry), countTotalUrls(toretry))
	totalTweets := len(toretry)
	if progress != nil {
		progress(RetryProgress{
			Total:     totalTweets,
			Completed: 0,
			Failed:    totalTweets,
		})
	}

	var completedTweets atomic.Int64
	var remainingTweets atomic.Int64
	remainingTweets.Store(int64(totalTweets))

	newFails := BatchDownloadTweet(ctx, client, true, dwn, fileWriter, func(pt PackagedTweet, failed bool) {
		if progress == nil {
			return
		}

		completed := int(completedTweets.Add(1))
		remaining := int(remainingTweets.Load())
		if !failed {
			remaining = int(remainingTweets.Add(-1))
		}

		progress(RetryProgress{
			Total:     totalTweets,
			Completed: completed,
			Failed:    remaining,
		})
	}, toretry...)
	dumper.Clear()
	for _, pt := range newFails {
		te := pt.(*TweetInEntity)
		if te.Tweet == nil {
			continue
		}
		eid, err := te.Entity.Id()
		if err != nil {
			log.Warnln("failed to get entity id:", err)
			continue
		}

		// 只保留还有URL需要下载的推文
		if len(te.Tweet.Urls) > 0 {
			dumper.Push(eid, te.Tweet)
			log.Warnf("tweet %d still has %d media(s) to download", te.Tweet.Id, len(te.Tweet.Urls))
		} else {
			log.Infof("tweet %d all media downloaded successfully on retry", te.Tweet.Id)
		}
	}

	summary := RetrySummary{
		TotalEntities:     totalEntities,
		RemainingEntities: dumper.EntityCount(),
	}
	if progress != nil {
		progress(RetryProgress{
			Total:     totalTweets,
			Completed: totalTweets,
			Failed:    len(newFails),
		})
	}
	return summary, nil
}

// countTotalUrls 统计所有推文中需要下载的URL总数
func countTotalUrls(tweets []PackagedTweet) int {
	count := 0
	for _, pt := range tweets {
		if pt.GetTweet() != nil {
			count += len(pt.GetTweet().Urls)
		}
	}
	return count
}
