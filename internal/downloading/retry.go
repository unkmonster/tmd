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

	// 注意：BatchDownloadTweet 内部的 downloadTweetMedia 会原地修改 tweet.Urls
	// （只保留仍需重试的 URL），此处 te.Tweet 指向 dumper 内部的同一 *twitter.Tweet 指针。
	// 因此下面对 te.Tweet.Urls 的读取反映的是已被 downloadTweetMedia 缩减后的结果。
	failedSet := make(map[uint64]struct{}, len(newFails))
	for _, pt := range newFails {
		if te, ok := pt.(*TweetInEntity); ok && te != nil && te.Tweet != nil {
			failedSet[te.Tweet.Id] = struct{}{}
		}
	}

	for _, pt := range toretry {
		te, ok := pt.(*TweetInEntity)
		if !ok || te == nil || te.Tweet == nil || te.Entity == nil {
			continue
		}
		eid, err := te.Entity.Id()
		if err != nil {
			log.Warnf("skip tweet %d: entity id error (%v), will retry next time", te.Tweet.Id, err)
			continue
		}

		if _, isFailed := failedSet[te.Tweet.Id]; !isFailed {
			dumper.Remove(eid, te.Tweet.Id)
			log.Infof("tweet %d all media downloaded successfully on retry", te.Tweet.Id)
		} else if len(te.Tweet.Urls) > 0 {
			log.Warnf("tweet %d still has %d media(s) to download", te.Tweet.Id, len(te.Tweet.Urls))
		} else {
			dumper.Remove(eid, te.Tweet.Id)
			log.Infof("tweet %d all media handled (non-retriable skipped) on retry", te.Tweet.Id)
		}
	}

	summary := RetrySummary{
		TotalEntities:     totalEntities,
		RemainingEntities: dumper.EntityCount(),
	}
	if progress != nil {
		progress(RetryProgress{
			Total:     totalTweets,
			Completed: int(completedTweets.Load()),
			Failed:    dumper.Count(),
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

func RetryFailedJsonTweets(ctx context.Context, dumper *JsonTweetDumper, client *resty.Client, dwn downloader.Downloader, fileWriter downloader.FileWriter, progress RetryProgressFunc) (RetrySummary, error) {
	if dumper.Count() == 0 {
		return RetrySummary{}, nil
	}
	totalEntries := dumper.EntryCount()

	log.Infoln("starting to retry failed JSON tweets")
	legacy := dumper.GetTotal()

	toretry := make([]PackagedTweet, 0, len(legacy))
	noUrlTweetIDs := make([]uint64, 0)
	for _, pt := range legacy {
		if pt.GetTweet() == nil {
			continue
		}
		if len(pt.GetTweet().Urls) > 0 {
			toretry = append(toretry, pt)
		} else {
			noUrlTweetIDs = append(noUrlTweetIDs, pt.GetTweet().Id)
		}
	}

	if len(toretry) == 0 {
		log.Infoln("no JSON tweets need to be retried")
		return RetrySummary{}, nil
	}

	tweetIDToSource := make(map[uint64]string, len(toretry))
	for sourcePath, ids := range dumper.set {
		for id := range ids {
			tweetIDToSource[id] = sourcePath
		}
	}

	log.Infof("retrying %d JSON tweets with %d total media(s)", len(toretry), countTotalUrls(toretry))
	totalTweets := len(toretry)
	if progress != nil {
		progress(RetryProgress{
			Total:     totalTweets,
			Completed: 0,
			Failed:    totalTweets,
		})
	}

	var completedTweets atomic.Int64

	newFails := BatchDownloadTweet(ctx, client, true, dwn, fileWriter, func(pt PackagedTweet, failed bool) {
		if progress == nil {
			return
		}
		completed := int(completedTweets.Add(1))
		progress(RetryProgress{
			Total:     totalTweets,
			Completed: completed,
			Failed:    totalTweets - completed,
		})
	}, toretry...)

	failedSet := make(map[uint64]struct{}, len(newFails))
	for _, pt := range newFails {
		if pt.GetTweet() != nil {
			failedSet[pt.GetTweet().Id] = struct{}{}
		}
	}

	for _, pt := range toretry {
		jpt, ok := pt.(JsonPackagedTweet)
		if !ok || jpt.Tweet == nil {
			continue
		}
		sourcePath, exists := tweetIDToSource[jpt.Tweet.Id]
		if !exists {
			continue
		}
		if _, isFailed := failedSet[jpt.Tweet.Id]; !isFailed {
			dumper.Remove(sourcePath, jpt.Tweet.Id)
			log.Infof("JSON tweet %d all media downloaded successfully on retry", jpt.Tweet.Id)
		} else if len(jpt.Tweet.Urls) > 0 {
			log.Warnf("JSON tweet %d still has %d media(s) to download", jpt.Tweet.Id, len(jpt.Tweet.Urls))
		} else {
			dumper.Remove(sourcePath, jpt.Tweet.Id)
			log.Infof("JSON tweet %d all media handled (non-retriable skipped) on retry", jpt.Tweet.Id)
		}
	}

	for _, id := range noUrlTweetIDs {
		if sp, ok := tweetIDToSource[id]; ok {
			dumper.Remove(sp, id)
		}
	}

	summary := RetrySummary{
		TotalEntities:     totalEntries,
		RemainingEntities: dumper.EntryCount(),
	}
	if progress != nil {
		progress(RetryProgress{
			Total:     totalTweets,
			Completed: int(completedTweets.Load()),
			Failed:    dumper.Count(),
		})
	}
	return summary, nil
}
