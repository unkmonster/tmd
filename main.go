package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"github.com/gookit/color"
	_ "github.com/mattn/go-sqlite3"
	"github.com/natefinch/lumberjack"
	"github.com/rifflock/lfshook"
	log "github.com/sirupsen/logrus"

	"github.com/unkmonster/tmd/internal/api"
	"github.com/unkmonster/tmd/internal/cli"
	"github.com/unkmonster/tmd/internal/config"
	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/downloading"
	"github.com/unkmonster/tmd/internal/naming"
	"github.com/unkmonster/tmd/internal/path"
	"github.com/unkmonster/tmd/internal/twitter"
)

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
	var confArg bool
	var dbg bool
	var serverMode bool
	var serverPort int

	// 手动解析已知参数，保留未知参数传递给 cli
	args := os.Args[1:]
	var cliArgs []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-conf":
			confArg = true
		case "-dbg":
			dbg = true
		case "-server":
			serverMode = true
		case "-port":
			if i+1 < len(args) {
				serverPort, _ = strconv.Atoi(args[i+1])
				i++
			}
		default:
			cliArgs = append(cliArgs, args[i])
		}
	}
	if serverPort == 0 {
		serverPort = 25556
	}

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
		MaxSize:    2,
		MaxBackups: 2,
		MaxAge:     14,
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

	// Server 模式
	if serverMode {
		runServer(conf, appRootPath, serverPort)
		return
	}

	// CLI 模式
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
	additional := twitter.BatchLogin(ctx, dbg, twitterCookies, screenName)

	cliLogFile, err := os.OpenFile(cliLogPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalln("failed to create log file:", err)
	}
	defer cliLogFile.Close()
	cli.SetClientLogger(client, cliLogFile)
	for _, c := range additional {
		cli.SetClientLogger(c, cliLogFile)
	}

	pathHelper, err := path.NewStorePath(conf.RootPath)
	if err != nil {
		log.Fatalln("failed to make store dir:", err)
	}

	db, err := database.Connect(pathHelper.DB)
	if err != nil {
		log.Fatalln("failed to connect to database:", err)
	}
	defer db.Close()
	log.Infoln("database is connected")

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

	// 构造依赖
	deps := &cli.Dependencies{
		Client:            client,
		AdditionalClients: additional,
		DB:                db,
		Conf:              conf,
		AppRootPath:       appRootPath,
	}

	// 将 cli 参数传递给 Execute
	if err := cli.Execute(ctx, cliArgs, deps); err != nil {
		log.Fatalln("execute failed:", err)
	}
}

func runServer(conf *config.Config, appRootPath string, port int) {
	ctx := context.Background()

	// 登录
	client, screenName, err := twitter.Login(ctx, conf.Cookie.AuthToken, conf.Cookie.Ct0)
	if err != nil {
		log.Fatalln("failed to login:", err)
	}
	twitter.EnableRateLimit(client)
	log.Infoln("signed in as:", color.FgLightBlue.Render(screenName))

	// 加载额外 cookies
	additionalCookiesPath := filepath.Join(appRootPath, "additional_cookies.yaml")
	cookies, err := config.ReadAdditionalCookies(additionalCookiesPath)
	if err != nil {
		log.Warnln("failed to load additional cookies:", err)
	}
	twitterCookies := make([]twitter.AccountCookie, len(cookies))
	for i, c := range cookies {
		twitterCookies[i] = twitter.AccountCookie{AuthToken: c.AuthToken, Ct0: c.Ct0}
	}
	additional := twitter.BatchLogin(ctx, false, twitterCookies, screenName)

	// 连接数据库
	pathHelper, err := path.NewStorePath(conf.RootPath)
	if err != nil {
		log.Fatalln("failed to make store dir:", err)
	}

	db, err := database.Connect(pathHelper.DB)
	if err != nil {
		log.Fatalln("failed to connect to database:", err)
	}
	defer db.Close()
	log.Infoln("database is connected")

	// 创建并启动 API Server
	server := api.NewServer(client, additional, db, conf, appRootPath)
	if err := server.Start(port); err != nil {
		log.Fatalln("failed to start server:", err)
	}
}
