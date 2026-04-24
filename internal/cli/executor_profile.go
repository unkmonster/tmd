package cli

import (
	"context"
	"fmt"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"

	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/downloader"
	"github.com/unkmonster/tmd/internal/downloading/profile"
	"github.com/unkmonster/tmd/internal/twitter"
	"github.com/unkmonster/tmd/internal/utils"
)

func handleProfileOnly(ctx context.Context, cfg *CLIConfig, deps *Dependencies, pathHelper *StorePath, versionManager downloader.VersionManager, fileWriter downloader.FileWriter, dwn downloader.Downloader) error {
	shouldDownloadProfile := len(cfg.ProfileUsers.ScreenName) > 0 || len(cfg.ProfileList.ID) > 0
	if !shouldDownloadProfile {
		return nil
	}

	profileCtx, profileCancel := context.WithCancel(ctx)
	defer profileCancel()

	task := &Task{
		Users: make([]*twitter.User, 0),
		Lists: make([]twitter.ListBase, 0),
	}

	handleProfileDownload(profileCtx, cfg, deps, task, pathHelper, dwn, fileWriter, versionManager)
	return nil
}

func handleProfileDownload(ctx context.Context, cfg *CLIConfig, deps *Dependencies, task *Task, pathHelper *StorePath, dwn downloader.Downloader, fileWriter downloader.FileWriter, versionManager downloader.VersionManager) {
	clients := make([]*resty.Client, 0)
	clients = append(clients, deps.Client)
	clients = append(clients, deps.AdditionalClients...)

	storage, err := profile.NewFileStorageManager(pathHelper.Users)
	if err != nil {
		log.Fatalln("failed to create profile storage:", err)
	}
	storage.SetVersionManager(versionManager)

	profileDownloader := profile.NewProfileDownloaderWithDB(nil, storage, clients, deps.DB, dwn, fileWriter)

	requests := make([]profile.DownloadRequest, 0)

	if len(task.Users) > 0 {
		for _, user := range task.Users {
			req := profile.DownloadRequest{
				ScreenName: user.ScreenName,
				UserTitle:  user.Title(),
				Name:       user.Name,
				UserID:     user.Id,
			}
			if !cfg.NoProfile {
				req.AvatarURL = user.AvatarURL
				req.BannerURL = user.BannerURL
				req.Description = user.Description
				req.Location = user.Location
				req.URL = user.URL
				req.Verified = user.Verified
				req.Protected = user.IsProtected
				req.CreatedAt = user.CreatedAt
			}
			requests = append(requests, req)
		}
	}

	for _, screenName := range cfg.ProfileUsers.ScreenName {
		requests = append(requests, profile.DownloadRequest{
			ScreenName: screenName,
			UserTitle:  "",
			Name:       "",
			UserID:     0,
		})
	}

	if len(cfg.ProfileList.ID) > 0 {
		lists, err := cfg.ProfileList.GetList(ctx, deps.Client)
		if err != nil {
			log.WithError(err).Errorln("failed to get profile lists")
		} else {
			for _, lst := range lists {
				appendListMemberRequests(ctx, deps.Client, deps.DB, lst, &requests)
			}
		}
	}

	if len(task.Lists) > 0 {
		for _, lst := range task.Lists {
			appendListMemberRequests(ctx, deps.Client, deps.DB, lst, &requests)
		}
	}

	seen := make(map[string]bool)
	uniqueRequests := make([]profile.DownloadRequest, 0)
	for _, req := range requests {
		if !seen[req.ScreenName] {
			seen[req.ScreenName] = true
			uniqueRequests = append(uniqueRequests, req)
		}
	}

	if len(uniqueRequests) == 0 {
		log.Infoln("no users to download profile")
		return
	}

	log.Infoln("starting profile download for", len(uniqueRequests), "users")

	results := profileDownloader.DownloadMultiple(ctx, uniqueRequests)

	success := 0
	failed := 0
	skipped := 0
	for _, r := range results {
		if r.Success {
			success++
		} else if r.Error != nil {
			failed++
		} else {
			skipped++
		}
	}

	log.Infoln("profile download completed - total:", len(results), "success:", success, "failed:", failed, "skipped:", skipped)

	fmt.Println("\n=== PROFILE_DOWNLOAD_RESULTS ===")
	for _, r := range results {
		if !r.Success {
			status := "SKIP"
			if r.Error != nil {
				status = "FAIL"
			}
			fmt.Printf("SCREEN_NAME:%s|STATUS:%s\n", r.ScreenName, status)
		}
	}
	fmt.Println("=== END_RESULTS ===")
}

func appendListMemberRequests(ctx context.Context, client *resty.Client, db *sqlx.DB, lst twitter.ListBase, requests *[]profile.DownloadRequest) {
	membersResult, err := lst.GetMembers(ctx, client)
	if err != nil {
		log.WithError(err).WithField("list", lst.Title()).Errorln("failed to get list members")
		return
	}

	uids := utils.ExtractIDs(membersResult.Users, func(u *twitter.User) uint64 { return u.Id })
	database.MarkListMembersAccessibleByIDs(db, uids)

	for _, member := range membersResult.Users {
		*requests = append(*requests, profile.DownloadRequest{
			ScreenName:  member.ScreenName,
			UserTitle:   member.Title(),
			Name:        member.Name,
			UserID:      member.Id,
			AvatarURL:   member.AvatarURL,
			BannerURL:   member.BannerURL,
			Description: member.Description,
			Location:    member.Location,
			URL:         member.URL,
			Verified:    member.Verified,
			Protected:   member.IsProtected,
			CreatedAt:   member.CreatedAt,
		})
	}
}
