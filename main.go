package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"github.com/go-resty/resty/v2"
	"github.com/gookit/color"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/natefinch/lumberjack"
	"github.com/rifflock/lfshook"
	log "github.com/sirupsen/logrus"

	"github.com/unkmonster/tmd/internal/api"
	"github.com/unkmonster/tmd/internal/cli"
	"github.com/unkmonster/tmd/internal/config"
	"github.com/unkmonster/tmd/internal/consolelog"
	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/downloading"
	"github.com/unkmonster/tmd/internal/naming"
	"github.com/unkmonster/tmd/internal/path"
	"github.com/unkmonster/tmd/internal/service"
	"github.com/unkmonster/tmd/internal/twitter"
)

func initLogger(dbg bool, logFile io.Writer, logHub *consolelog.Hub) {
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

	if err := consolelog.StartCapture(logHub); err != nil {
		log.Warnf("failed to start console log capture: %v", err)
	} else {
		log.SetOutput(os.Stderr)
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
	consoleLogHub := consolelog.DefaultHub()
	initLogger(dbg, logWriter, consoleLogHub)

	defer func() {
		if dbg {
			twitter.ReportRequestCount()
		}
	}()

	conf, err := config.ReadConf(confPath)
	if os.IsNotExist(err) || confArg {
		conf, err = config.PromptConfig(confPath)
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
	log.Infoln("download path:", conf.RootPath)
	if conf.MaxDownloadRoutine > 0 {
		downloading.MaxDownloadRoutine = conf.MaxDownloadRoutine
	}
	log.Infoln("max download routine set to:", downloading.MaxDownloadRoutine)
	if conf.MaxFileNameLen > 0 {
		naming.MaxFileNameLen = conf.MaxFileNameLen
	}
	log.Infoln("max file name length set to:", naming.MaxFileNameLen)

	loginOpts := twitter.LoginOptions{ProxyURL: conf.ProxyURL}

	// Server 模式
	if serverMode {
		runServer(conf, appRootPath, serverPort, loginOpts, logWriter, consoleLogHub)
		return
	}

	// CLI 模式
	client, additional, _, db := initializeClients(ctx, conf, appRootPath, loginOpts, dbg)
	defer db.Close()

	// 设置客户端日志
	cliLogFile, err := os.OpenFile(cliLogPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalln("failed to create log file:", err)
	}
	defer cliLogFile.Close()
	cli.SetClientLogger(client, cliLogFile)
	for _, c := range additional {
		cli.SetClientLogger(c, cliLogFile)
	}

	// 信号处理
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
		Dependencies: service.Dependencies{
			Client:            client,
			AdditionalClients: additional,
			DB:                db,
			Config:            conf,
		},
	}

	// 将 cli 参数传递给 Execute
	if err := cli.Execute(ctx, cliArgs, deps); err != nil {
		log.Fatalln("execute failed:", err)
	}
}

// initializeClients 初始化 Twitter 客户端和数据库连接
// 返回主客户端、附加客户端列表、路径助手和数据库连接
func initializeClients(
	ctx context.Context,
	conf *config.Config,
	appRootPath string,
	loginOpts twitter.LoginOptions,
	enableRequestCounting bool,
) (*resty.Client, []*resty.Client, *path.StorePath, *sqlx.DB) {
	// 登录主账户
	client, screenName, err := twitter.LoginWithOptions(ctx, conf.Cookie.AuthToken, conf.Cookie.Ct0, loginOpts)
	if err != nil {
		log.Fatalln("failed to login:", err)
	}
	twitter.EnableRateLimit(client)
	if enableRequestCounting {
		twitter.EnableRequestCounting(client)
	}
	log.Infoln("signed in as:", color.FgLightBlue.Render(screenName))

	// 加载额外 cookies
	additionalCookiesPath := filepath.Join(appRootPath, "additional_cookies.yaml")
	cookies, err := config.ReadAdditionalCookies(additionalCookiesPath)
	if err != nil {
		log.Warnln("failed to load additional cookies:", err)
	}
	log.Debugln("loaded additional cookies:", len(cookies))

	twitterCookies := make([]twitter.AccountCookie, len(cookies))
	for i, c := range cookies {
		twitterCookies[i] = twitter.AccountCookie{AuthToken: c.AuthToken, Ct0: c.Ct0}
	}

	batchOpts := twitter.BatchLoginOptions{Debug: enableRequestCounting, ProxyURL: conf.ProxyURL}
	additional := twitter.BatchLogin(ctx, batchOpts, twitterCookies, screenName)

	// 初始化路径和数据库
	pathHelper, err := path.NewStorePath(conf.RootPath)
	if err != nil {
		log.Fatalln("failed to make store dir:", err)
	}

	db, err := database.Connect(pathHelper.DB)
	if err != nil {
		log.Fatalln("failed to connect to database:", err)
	}
	log.Infoln("database is connected")
	downloading.InitListSyncManager(db)

	return client, additional, pathHelper, db
}

func runServer(conf *config.Config, appRootPath string, port int, loginOpts twitter.LoginOptions, logWriter io.Closer, logHub *consolelog.Hub) {
	ctx := context.Background()

	client, additional, _, db := initializeClients(ctx, conf, appRootPath, loginOpts, false)

	// 设置客户端日志
	cliLogPath := filepath.Join(appRootPath, "client.log")
	// 注意：Server 模式下通常长久运行，这里使用 O_APPEND 追加模式，而不是 O_TRUNC 截断
	cliLogFile, err := os.OpenFile(cliLogPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalln("failed to create log file:", err)
	}
	// 在目前的简单实现中，我们把它交给 resty 管理，它会在应用退出时随进程关闭。
	cli.SetClientLogger(client, cliLogFile)
	for _, c := range additional {
		cli.SetClientLogger(c, cliLogFile)
	}

	// 创建并启动 API Server
	// 注意：不再使用 defer db.Close()，因为 GracefulShutdown 会处理所有资源清理
	server := api.NewServerWithConsoleLogHub(client, additional, db, conf, appRootPath, logWriter, logHub)

	// 信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(sigChan)
	startServerSignalHandler(sigChan, server.GracefulShutdown)

	err = server.Start(port)
	if err != nil && err != http.ErrServerClosed {
		log.Fatalln("failed to start server:", err)
	}
	if err == http.ErrServerClosed {
		server.WaitForShutdown()
	}
}

func startServerSignalHandler(sigChan <-chan os.Signal, shutdown func(string)) {
	go func() {
		sig := <-sigChan
		log.Warnln("[server] caught signal:", sig)
		// SIGKILL 无法捕获；这里只处理可拦截的退出信号，确保数据库等资源优雅关闭。
		shutdown("signal:" + sig.String())
	}()
}
