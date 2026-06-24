package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/gookit/color"
	"github.com/jmoiron/sqlx"
	"github.com/natefinch/lumberjack"
	"github.com/rifflock/lfshook"
	log "github.com/sirupsen/logrus"

	"github.com/unkmonster/tmd/internal/api"
	"github.com/unkmonster/tmd/internal/cli"
	"github.com/unkmonster/tmd/internal/config"
	"github.com/unkmonster/tmd/internal/consolelog"
	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/downloading"
	"github.com/unkmonster/tmd/internal/path"
	"github.com/unkmonster/tmd/internal/service"
	"github.com/unkmonster/tmd/internal/twitter"
	"github.com/unkmonster/tmd/internal/utils"
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
		log.Warnf("[startup] Failed to start console log capture: %v", err)
	} else {
		log.SetOutput(os.Stderr)
	}
	log.AddHook(lfshook.NewHook(logFile, nil))
}

func main() {
	var serverPort int
	var err error

	bootstrap, err := parseBootstrapArgs(os.Args[1:])
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !bootstrap.serverPortSet {
		serverPort, err = serverPortFromEnv()
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else {
		serverPort = bootstrap.serverPort
	}
	if serverPort == 0 {
		serverPort = 25556
	}

	ctx, cancel := context.WithCancel(context.Background())

	appRootPath, err := resolveAppRootPath()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	confPath := filepath.Join(appRootPath, "conf.yaml")
	cliLogPath := filepath.Join(appRootPath, "client.log")
	logPath := filepath.Join(appRootPath, "tmd2.log")
	if err = os.MkdirAll(appRootPath, 0755); err != nil {
		log.Fatalln("[startup] Failed to make app dir", err)
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
	initLogger(bootstrap.dbg, logWriter, consoleLogHub)

	defer func() {
		if bootstrap.dbg {
			twitter.ReportRequestCount()
		}
	}()

	loadResult, err := config.LoadStartupConfig(confPath, bootstrap.confArg, os.Stderr)
	if err != nil {
		log.Fatalln("[startup] Config failure with", err)
	}
	conf := loadResult.Config
	if loadResult.UsedEnvFallback {
		log.Infoln("Config file not found, using TMD_* environment configuration")
	}
	if loadResult.EnvApplied {
		log.Infoln("TMD_* environment configuration applied")
	}
	if bootstrap.confArg {
		log.Infoln("[config] Config done")
		return
	}
	if err := config.Validate(conf); err != nil {
		log.Fatalln("[startup] Invalid config:", err)
	}
	log.Infoln("[startup] Config is loaded")
	log.Infoln("[startup] Download path:", conf.RootPath)
	maxDownloadRoutine := conf.MaxDownloadRoutine
	if maxDownloadRoutine <= 0 {
		maxDownloadRoutine = config.DefaultMaxDownloadRoutine()
	}
	log.Infoln("[startup] Max download routine set to:", maxDownloadRoutine)
	maxFileNameLen := conf.MaxFileNameLen
	if maxFileNameLen <= 0 {
		maxFileNameLen = utils.DefaultMaxFileNameLen
	}
	log.Infoln("[startup] Max file name length set to:", maxFileNameLen)

	if conf.ProxyURL != "" {
		os.Setenv("HTTP_PROXY", conf.ProxyURL)
		os.Setenv("HTTPS_PROXY", conf.ProxyURL)
	} else {
		// conf 没设代理，检查系统环境变量，只设一个时同步到另一个
		httpProxy := os.Getenv("HTTP_PROXY")
		httpsProxy := os.Getenv("HTTPS_PROXY")
		if httpProxy == "" {
			httpProxy = os.Getenv("http_proxy")
		}
		if httpsProxy == "" {
			httpsProxy = os.Getenv("https_proxy")
		}

		if httpProxy != "" && httpsProxy == "" {
			os.Setenv("HTTPS_PROXY", httpProxy)
			os.Setenv("https_proxy", httpProxy)
		} else if httpsProxy != "" && httpProxy == "" {
			os.Setenv("HTTP_PROXY", httpsProxy)
			os.Setenv("http_proxy", httpsProxy)
		}
	}

	loginOpts := twitter.LoginOptions{}

	// Server 模式
	if bootstrap.serverMode {
		runServer(conf, appRootPath, serverPort, loginOpts, logWriter, consoleLogHub)
		return
	}

	// CLI 模式
	client, additional, _, db := initializeClients(ctx, conf, appRootPath, loginOpts, bootstrap.dbg)
	if client == nil || db == nil {
		log.Fatalln("[startup] Failed to initialize clients or database")
	}
	defer db.Close()

	// 设置客户端日志
	cliLogFile, err := os.OpenFile(cliLogPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalln("[startup] Failed to create log file:", err)
	}
	defer cliLogFile.Close()
	cli.SetClientLogger(client, cliLogFile)
	for _, c := range additional {
		cli.SetClientLogger(c, cliLogFile)
	}

	// 信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, shutdownSignals()...)
	defer close(sigChan)
	defer signal.Stop(sigChan)
	go func() {
		sig, ok := <-sigChan
		if ok {
			log.Warnln("[listener] Caught signal:", sig)
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
			ListSyncManager:   downloading.NewListSyncManager(db),
		},
	}

	// 将 cli 参数传递给 Execute
	if err := cli.Execute(ctx, bootstrap.cliArgs, deps); err != nil {
		log.Fatalln("[startup] Execute failed:", err)
	}
}

type bootstrapArgs struct {
	confArg       bool
	dbg           bool
	serverMode    bool
	serverPort    int
	serverPortSet bool
	cliArgs       []string
}

func parseBootstrapArgs(args []string) (bootstrapArgs, error) {
	var parsed bootstrapArgs
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-conf":
			parsed.confArg = true
		case "-dbg":
			parsed.dbg = true
		case "-server":
			parsed.serverMode = true
		case "-port":
			if i+1 >= len(args) {
				return parsed, fmt.Errorf("-port requires a value")
			}
			port, err := strconv.Atoi(args[i+1])
			if err != nil || port <= 0 || port > 65535 {
				return parsed, fmt.Errorf("invalid -port %q: must be an integer from 1 to 65535", args[i+1])
			}
			parsed.serverPort = port
			parsed.serverPortSet = true
			i++
		default:
			parsed.cliArgs = append(parsed.cliArgs, arg)
		}
	}
	return parsed, nil
}

func serverPortFromEnv() (int, error) {
	raw := strings.TrimSpace(os.Getenv("TMD_PORT"))
	if raw == "" {
		return 0, nil
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port <= 0 || port > 65535 {
		return 0, fmt.Errorf("invalid TMD_PORT %q: must be an integer from 1 to 65535", raw)
	}
	return port, nil
}

func resolveAppRootPath() (string, error) {
	if tmdHome := strings.TrimSpace(os.Getenv("TMD_HOME")); tmdHome != "" {
		absPath, err := filepath.Abs(tmdHome)
		if err != nil {
			return "", fmt.Errorf("failed to resolve TMD_HOME %q: %w", tmdHome, err)
		}
		return absPath, nil
	}

	var homepath string
	if runtime.GOOS == "windows" {
		homepath = os.Getenv("APPDATA")
	} else {
		homepath = os.Getenv("HOME")
	}
	if homepath == "" {
		return "", fmt.Errorf("failed to get home path from env; set TMD_HOME to the app config directory")
	}
	return filepath.Join(homepath, ".tmd2"), nil
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
		log.Fatalln("[startup] Failed to login:", err)
	}
	twitter.EnableRateLimit(client)
	if enableRequestCounting {
		twitter.EnableRequestCounting(client)
	}
	log.Infoln("[startup] Signed in as:", color.FgLightBlue.Render(screenName))

	// 加载额外 cookies
	additionalCookiesPath := filepath.Join(appRootPath, "additional_cookies.yaml")
	cookies, err := config.ReadAdditionalCookies(additionalCookiesPath)
	if err != nil {
		log.Warnln("[startup] Failed to load additional cookies:", err)
	}
	log.Debugln("[startup] Loaded additional cookies:", len(cookies))

	twitterCookies := make([]twitter.AccountCookie, len(cookies))
	for i, c := range cookies {
		twitterCookies[i] = twitter.AccountCookie{AuthToken: c.AuthToken, Ct0: c.Ct0}
	}

	batchOpts := twitter.BatchLoginOptions{Debug: enableRequestCounting}
	additional := twitter.BatchLogin(ctx, batchOpts, twitterCookies, screenName)

	// 初始化路径和数据库
	pathHelper, err := path.NewStorePath(conf.RootPath)
	if err != nil {
		log.Warnln("[startup] Failed to make store dir:", err)
		return nil, nil, nil, nil
	}

	db, err := database.Connect(pathHelper.DB)
	if err != nil {
		log.Fatalln("[startup] Failed to connect to database:", err)
	}
	log.Infoln("[startup] Database is connected")

	return client, additional, pathHelper, db
}

func runServer(conf *config.Config, appRootPath string, port int, loginOpts twitter.LoginOptions, logWriter io.Closer, logHub *consolelog.Hub) {
	ctx := context.Background()

	client, additional, _, db := initializeClients(ctx, conf, appRootPath, loginOpts, false)
	if client == nil {
		_, _ = fmt.Fprintln(os.Stderr, "Failed to initialize: unable to create store directory.")
		return
	}

	// 设置客户端日志
	cliLogPath := filepath.Join(appRootPath, "client.log")
	// 注意：Server 模式下通常长久运行，这里使用 O_APPEND 追加模式，而不是 O_TRUNC 截断
	cliLogFile, err := os.OpenFile(cliLogPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalln("[startup] Failed to create log file:", err)
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
	signal.Notify(sigChan, shutdownSignals()...)
	defer signal.Stop(sigChan)
	startServerSignalHandler(sigChan, server.GracefulShutdown)

	err = server.Start(port)
	if err != nil && err != http.ErrServerClosed {
		log.Fatalln("[startup] Failed to start server:", err)
	}
	if err == http.ErrServerClosed {
		server.WaitForShutdown()
	}
}

func startServerSignalHandler(sigChan <-chan os.Signal, shutdown func(string)) {
	go func() {
		sig := <-sigChan
		log.Warnln("[server] Caught signal:", sig)
		// SIGKILL 无法捕获；这里只处理可拦截的退出信号，确保数据库等资源优雅关闭。
		shutdown("signal:" + sig.String())
	}()
}
