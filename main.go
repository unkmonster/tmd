package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/go-resty/resty/v2"
	"github.com/gookit/color"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/natefinch/lumberjack"
	"github.com/rifflock/lfshook"
	log "github.com/sirupsen/logrus"
	"github.com/unkmonster/tmd/internal/config"
	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/downloader"
	"github.com/unkmonster/tmd/internal/downloading"
	"github.com/unkmonster/tmd/internal/naming"
	"github.com/unkmonster/tmd/internal/profile"
	"github.com/unkmonster/tmd/internal/twitter"
	"github.com/unkmonster/tmd/internal/utils"
)

type userArgs struct {
	id         []uint64
	screenName []string
}

func (u *userArgs) GetUser(ctx context.Context, client *resty.Client, db *sqlx.DB) ([]*twitter.User, error) {
	users := []*twitter.User{}
	for _, id := range u.id {
		usr, uid, err := twitter.GetUserById(ctx, client, id)
		if err != nil {
			database.MarkUserInaccessible(db, uid, "")
			return nil, err
		}
		users = append(users, usr)
	}

	for _, screenName := range u.screenName {
		usr, uid, err := twitter.GetUserByScreenName(ctx, client, screenName)
		if err != nil {
			database.MarkUserInaccessible(db, uid, screenName)
			return nil, err
		}
		users = append(users, usr)
	}
	return users, nil
}

func (u *userArgs) Set(str string) error {
	if u.id == nil {
		u.id = make([]uint64, 0)
		u.screenName = make([]string, 0)
	}

	id, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		str, _ := strings.CutPrefix(str, "@")
		u.screenName = append(u.screenName, str)
	} else {
		u.id = append(u.id, id)
	}
	return nil
}

func (u *userArgs) String() string {
	return fmt.Sprintf("ids=%v screenNames=%v", u.id, u.screenName)
}

type intArgs struct {
	id []uint64
}

func (l *intArgs) Set(str string) error {
	if l.id == nil {
		l.id = make([]uint64, 0)
	}

	id, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		return err
	}
	l.id = append(l.id, id)
	return nil
}

func (a *intArgs) String() string {
	return fmt.Sprintf("%v", a.id)
}

type ListArgs struct {
	intArgs
}

type jsonPathsArgs struct {
	paths []string
}

func (j *jsonPathsArgs) Set(str string) error {
	if j.paths == nil {
		j.paths = make([]string, 0)
	}
	j.paths = append(j.paths, str)
	return nil
}

func (j *jsonPathsArgs) String() string {
	return strings.Join(j.paths, ",")
}

func (j *jsonPathsArgs) GetPaths() []string {
	return j.paths
}

func (l ListArgs) GetList(ctx context.Context, client *resty.Client) ([]*twitter.List, error) {
	lists := []*twitter.List{}
	for _, id := range l.id {
		list, err := twitter.GetLst(ctx, client, id)
		if err != nil {
			return nil, err
		}
		lists = append(lists, list)
	}
	return lists, nil
}

type Task struct {
	users []*twitter.User
	lists []twitter.ListBase
}

func printTask(task *Task) {
	if len(task.users) != 0 {
		fmt.Printf("users: %d\n", len(task.users))
	}
	for _, u := range task.users {
		fmt.Printf("    - %s\n", u.Title())
	}
	if len(task.lists) != 0 {
		fmt.Printf("lists: %d\n", len(task.lists))
	}
	for _, l := range task.lists {
		fmt.Printf("    - %s\n", l.Title())
	}
}

func MakeTask(ctx context.Context, client *resty.Client, db *sqlx.DB, usrArgs userArgs, listArgs ListArgs, follArgs userArgs) (*Task, error) {
	task := Task{}
	task.users = make([]*twitter.User, 0)
	task.lists = make([]twitter.ListBase, 0)

	users, err := usrArgs.GetUser(ctx, client, db)
	if err != nil {
		return nil, err
	}
	task.users = append(task.users, users...)

	lists, err := listArgs.GetList(ctx, client)
	if err != nil {
		return nil, err
	}
	for _, list := range lists {
		task.lists = append(task.lists, list)
	}

	users, err = follArgs.GetUser(ctx, client, db)
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		task.lists = append(task.lists, user.Following())
	}
	return &task, nil
}

type storePath struct {
	root   string
	users  string
	data   string
	db     string
	errorj string
}

func newStorePath(root string) (*storePath, error) {
	ph := storePath{}
	ph.root = root
	ph.users = filepath.Join(root, "users")
	ph.data = filepath.Join(root, ".data")

	ph.db = filepath.Join(ph.data, "foo.db")
	ph.errorj = filepath.Join(ph.data, "errors.json")

	err := os.Mkdir(ph.root, 0755)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}

	err = os.Mkdir(ph.users, 0755)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}

	err = os.Mkdir(ph.data, 0755)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	return &ph, nil
}

func initLogger(dbg bool, logFile io.Writer) {
	log.SetFormatter(&log.TextFormatter{
		ForceColors:    true,
		FullTimestamp:  true,
		DisableSorting: true,
		PadLevelText:   false,
	})

	if dbg {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	log.AddHook(lfshook.NewHook(logFile, nil))
}

func main() {
	var usrArgs userArgs
	var listArgs ListArgs
	var follArgs userArgs
	var confArg bool
	var dbg bool
	var autoFollow bool
	var noRetry bool
	var markDownloaded bool
	var markTime string
	var noProfile bool
	var profileUsers userArgs
	var profileList ListArgs
	var jsonArgs jsonPathsArgs

	flag.BoolVar(&confArg, "conf", false, "reconfigure")
	flag.Var(&usrArgs, "user", "download tweets from the user specified by user_id/screen_name since the last download")
	flag.Var(&listArgs, "list", "batch download each member from list specified by list_id")
	flag.Var(&follArgs, "foll", "batch download each member followed by the user specified by user_id/screen_name")
	flag.BoolVar(&dbg, "dbg", false, "display debug message")
	flag.BoolVar(&autoFollow, "auto-follow", false, "send follow request automatically to protected users (enabled by default for list downloads)")
	flag.BoolVar(&noRetry, "no-retry", false, "quickly exit without retrying failed tweets")
	flag.BoolVar(&markDownloaded, "mark-downloaded", false, "mark users as downloaded without downloading content (sets latest_release_time to now)")
	flag.StringVar(&markTime, "mark-time", "", "timestamp for mark-downloaded (format: 2006-01-02T15:04:05), empty means now")
	flag.BoolVar(&noProfile, "noprofile", false, "skip downloading user profiles")
	flag.Var(&profileUsers, "profile-user", "download profile for specified user (can be used multiple times)")
	flag.Var(&profileList, "profile-list", "download profiles for all members in the specified list")
	flag.Var(&jsonArgs, "json", "download media from JSON file(s) exported by other tools (supports raw API JSON and formatted .loongtweet JSON)")
	flag.Parse()

	var err error

	ctx, cancel := context.WithCancel(context.Background())

	var homepath string
	if runtime.GOOS == "windows" {
		homepath = os.Getenv("appdata")
	} else {
		homepath = os.Getenv("HOME")
	}
	if homepath == "" {
		panic("failed to get home path from env")
	}

	appRootPath := filepath.Join(homepath, ".tmd2")
	confPath := filepath.Join(appRootPath, "conf.yaml")
	cliLogPath := filepath.Join(appRootPath, "client.log")
	logPath := filepath.Join(appRootPath, "tmd2.log")
	additionalCookiesPath := filepath.Join(appRootPath, "additional_cookies.yaml")
	if err = os.MkdirAll(appRootPath, 0755); err != nil {
		log.Fatalln("failed to make app dir", err)
	}

	logWriter := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    5,
		MaxBackups: 2,
		MaxAge:     7,
		Compress:   false,
	}
	defer logWriter.Close()
	initLogger(dbg, logWriter)

	defer func() {
		if dbg {
			twitter.ReportRequestCount()
		}
	}()

	conf, err := config.ReadConf(confPath)
	if os.IsNotExist(err) || confArg {
		if confArg {
			conf, err = config.PromptPartialConfig(confPath)
		} else {
			conf, err = config.PromptConfig(confPath)
		}
		if err != nil {
			log.Fatalln("config failure with", err)
		}
	}
	if err != nil {
		log.Fatalln("failed to load config:", err)
	}
	if confArg {
		log.Println("config done")
		return
	}
	log.Infoln("config is loaded")
	if conf.MaxDownloadRoutine > 0 {
		downloading.MaxDownloadRoutine = conf.MaxDownloadRoutine
	}
	if conf.MaxFileNameLen > 0 {
		if conf.MaxFileNameLen < 50 {
			conf.MaxFileNameLen = 50
		}
		if conf.MaxFileNameLen > 250 {
			conf.MaxFileNameLen = 250
		}
		naming.MaxFileNameLen = conf.MaxFileNameLen
		log.Infoln("max file name length set to:", naming.MaxFileNameLen)
	}

	pathHelper, err := newStorePath(conf.RootPath)
	if err != nil {
		log.Fatalln("failed to make store dir:", err)
	}

	client, screenName, err := twitter.Login(ctx, conf.Cookie.AuthToken, conf.Cookie.Ct0)
	if err != nil {
		log.Fatalln("failed to login:", err)
	}
	twitter.EnableRateLimit(client)
	if dbg {
		twitter.EnableRequestCounting(client)
	}
	log.Infoln("signed in as:", color.FgLightBlue.Render(screenName))

	cookies, err := config.ReadAdditionalCookies(additionalCookiesPath)
	if err != nil {
		log.Warnln("failed to load additional cookies:", err)
	}
	log.Debugln("loaded additional cookies:", len(cookies))
	twitterCookies := make([]twitter.AccountCookie, len(cookies))
	for i, c := range cookies {
		twitterCookies[i] = twitter.AccountCookie{AuthToken: c.AuthToken, Ct0: c.Ct0}
	}
	addtional := twitter.BatchLogin(ctx, dbg, twitterCookies, screenName)

	cliLogFile, err := os.OpenFile(cliLogPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalln("failed to create log file:", err)
	}
	defer cliLogFile.Close()
	setClientLogger(client, cliLogFile)
	for _, cli := range addtional {
		setClientLogger(cli, cliLogFile)
	}

	dumper := downloading.NewDumper()
	err = dumper.Load(pathHelper.errorj)
	if err != nil {
		log.Fatalln("failed to load previous tweets", err)
	}
	log.Infoln("loaded previous failed tweets:", dumper.Count())

	db, err := database.Connect(pathHelper.db)
	if err != nil {
		log.Fatalln("failed to connect to database:", err)
	}
	defer db.Close()
	log.Infoln("database is connected")

	task, err := MakeTask(ctx, client, db, usrArgs, listArgs, follArgs)
	if err != nil {
		log.Fatalln("failed to parse cmd args:", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer close(sigChan)
	defer signal.Stop(sigChan)
	go func() {
		sig, ok := <-sigChan
		if ok {
			log.Warnln("[listener] caught signal:", sig)
			cancel()
		}
	}()

	versionManager := downloader.NewVersionManagerWithWriter(".versions", nil)
	fileWriter := downloader.NewFileWriter(versionManager)
	versionManager.SetFileWriter(fileWriter)
	dwn := downloader.NewDownloader(fileWriter)

	var todump = make([]*downloading.TweetInEntity, 0)
	defer func() {
		dumper.Dump(pathHelper.errorj)
		log.Infof("%d tweets have been dumped and will be downloaded the next time the program runs", dumper.Count())
	}()

	defer func() {
		for _, te := range todump {
			eid, err := te.Entity.Id()
			if err != nil {
				log.Warnln("failed to get entity id:", err)
				continue
			}
			dumper.Push(eid, te.Tweet)
		}
		if ctx.Err() != context.Canceled && !noRetry {
			downloading.RetryFailedTweets(ctx, dumper, db, client, dwn, fileWriter)
		}
	}()

	if len(task.users) == 0 && len(task.lists) == 0 && len(jsonArgs.GetPaths()) == 0 {
		goto handleProfile
	}
	log.Infoln("start working for...")
	printTask(task)

	if markDownloaded {
		results, err := downloading.MarkUsersAsDownloaded(ctx, client, db, task.lists, task.users, pathHelper.users, markTime)
		if err != nil {
			log.Errorln("failed to mark users as downloaded:", err)
			os.Exit(1)
		}
		if len(results) > 0 {
			fmt.Println("\n=== MARK_DOWNLOADED_RESULTS ===")
			for _, r := range results {
				status := "OK"
				if !r.Success {
					status = "FAIL"
				}
				fmt.Printf("ENTITY_ID:%d|USER_ID:%d|SCREEN_NAME:%s|STATUS:%s\n", r.EntityID, r.UserID, r.ScreenName, status)
			}
			fmt.Println("=== END_RESULTS ===")
		}
	} else if len(jsonArgs.GetPaths()) > 0 {
		log.Infof("downloading from %d JSON file(s)...", len(jsonArgs.GetPaths()))
		results := downloading.DownloadJsonDir(ctx, client, pathHelper.root, dwn, fileWriter, jsonArgs.GetPaths()...)
		var successCount, failCount int
		for _, r := range results {
			if r.Success {
				successCount++
				log.Infof("✓ %s: %d tweets processed in %v", filepath.Base(r.Path), r.TweetCount, r.Duration)
			} else {
				failCount++
				log.Errorf("✗ %s: %v", filepath.Base(r.Path), r.Error)
			}
		}
		log.Infof("JSON download completed: %d success, %d failed", successCount, failCount)
	} else {
		todump, err = downloading.BatchDownloadAny(ctx, client, db, task.lists, task.users, pathHelper.root, pathHelper.users, autoFollow, addtional, dwn, fileWriter)
		if err != nil {
			log.Errorln("failed to download:", err)
		}
	}

handleProfile:
	shouldDownloadProfile := !noProfile && (len(usrArgs.screenName) > 0 || len(listArgs.id) > 0 || len(follArgs.screenName) > 0)

	if shouldDownloadProfile || len(profileUsers.screenName) > 0 || len(profileList.id) > 0 {
		profileCtx, profileCancel := context.WithCancel(context.Background())
		profileDone := make(chan struct{})
		go func() {
			defer close(profileDone)
			handleProfileDownload(profileCtx, client, addtional, pathHelper.users, profileUsers, profileList, task, db, shouldDownloadProfile, dwn, fileWriter, versionManager)
		}()
		select {
		case <-profileDone:
		case <-ctx.Done():
			profileCancel()
			<-profileDone
		}
		profileCancel()
	}
}

func setClientLogger(client *resty.Client, out io.Writer) {
	logger := log.New()
	logger.SetLevel(log.InfoLevel)
	logger.SetOutput(out)
	logger.SetFormatter(&log.TextFormatter{
		FullTimestamp:  true,
		DisableQuote:   true,
		DisableSorting: true,
		PadLevelText:   false,
	})
	client.SetLogger(logger)
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

func handleProfileDownload(ctx context.Context, client *resty.Client, additional []*resty.Client, usersPath string, profileUsers userArgs, profileList ListArgs, task *Task, db *sqlx.DB, skipAPIFetch bool, dwn downloader.Downloader, fileWriter downloader.FileWriter, versionManager downloader.VersionManager) {
	clients := make([]*resty.Client, 0)
	clients = append(clients, client)
	clients = append(clients, additional...)

	storage, err := profile.NewFileStorageManager(usersPath)
	if err != nil {
		log.Fatalln("failed to create profile storage:", err)
	}
	storage.SetVersionManager(versionManager)

	profileDownloader := profile.NewProfileDownloaderWithDB(nil, storage, clients, db, dwn, fileWriter)

	requests := make([]profile.DownloadRequest, 0)

	if len(task.users) > 0 {
		for _, user := range task.users {
			req := profile.DownloadRequest{
				ScreenName: user.ScreenName,
				UserTitle:  user.Title(),
				Name:       user.Name,
				UserID:     user.Id,
			}
			if skipAPIFetch {
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

	for _, screenName := range profileUsers.screenName {
		requests = append(requests, profile.DownloadRequest{
			ScreenName: screenName,
			UserTitle:  "",
			Name:       "",
			UserID:     0,
		})
	}

	if len(profileList.id) > 0 {
		lists, err := profileList.GetList(ctx, client)
		if err != nil {
			log.WithError(err).Errorln("failed to get profile lists")
		} else {
			for _, lst := range lists {
				appendListMemberRequests(ctx, client, db, lst, &requests)
			}
		}
	}

	if len(task.lists) > 0 {
		for _, lst := range task.lists {
			appendListMemberRequests(ctx, client, db, lst, &requests)
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
