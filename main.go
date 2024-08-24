package main

import (
	"bufio"
	"bytes"
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
	log "github.com/sirupsen/logrus"
	"github.com/unkmonster/tmd2/database"
	"github.com/unkmonster/tmd2/downloading"
	"github.com/unkmonster/tmd2/internal/utils"
	"github.com/unkmonster/tmd2/twitter"
	"gopkg.in/yaml.v3"
)

type Cookie struct {
	AuthCoken string `yaml:"auth_token"`
	Ct0       string `yaml:"ct0"`
}

type Config struct {
	RootPath           string `yaml:"root_path"`
	Cookie             Cookie `yaml:"cookie"`
	MaxDownloadRoutine int    `yaml:"max_download_routine"`
}

type userArgs struct {
	id         []uint64
	screenName []string
}

func (u *userArgs) GetUser(ctx context.Context, client *resty.Client) ([]*twitter.User, error) {
	users := []*twitter.User{}
	for _, id := range u.id {
		usr, err := twitter.GetUserById(ctx, client, id)
		if err != nil {
			return nil, err
		}
		users = append(users, usr)
	}

	for _, screenName := range u.screenName {
		usr, err := twitter.GetUserByScreenName(ctx, client, screenName)
		if err != nil {
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
	return "string"
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
	return "string array"
}

type ListArgs struct {
	intArgs
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
	fmt.Printf("users: %d\n", len(task.users))
	for _, u := range task.users {
		fmt.Printf("    - %s\n", u.Title())
	}
	fmt.Printf("lists: %d\n", len(task.lists))
	for _, l := range task.lists {
		fmt.Printf("    - %s\n", l.Title())
	}
}

func MakeTask(ctx context.Context, client *resty.Client, usrArgs userArgs, listArgs ListArgs, follArgs userArgs) (*Task, error) {
	task := Task{}
	task.users = make([]*twitter.User, 0)
	task.lists = make([]twitter.ListBase, 0)

	users, err := usrArgs.GetUser(ctx, client)
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

	// fo
	users, err = follArgs.GetUser(ctx, client)
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

	// ensure folder exist
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

func initLogger(dbg bool) {
	log.SetFormatter(&log.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
	})

	if dbg {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
}

func main() {
	//flag.
	var usrArgs userArgs
	var listArgs ListArgs
	var follArgs userArgs
	var confArg bool
	var dbg bool
	flag.BoolVar(&confArg, "conf", false, "reconfigure")
	flag.Var(&usrArgs, "user", "download tweets from the user specified by user_id/screen_name since the last download")
	flag.Var(&listArgs, "list", "batch download each member from list specified by list_id")
	flag.Var(&follArgs, "foll", "batch download each member followed by the user specified by user_id/screen_name")
	flag.BoolVar(&dbg, "dbg", false, "display debug message")
	flag.Parse()

	var err error

	initLogger(dbg)

	// context
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
	if err = os.MkdirAll(appRootPath, 0755); err != nil {
		log.Fatalln("failed to make app dir", err)
	}

	// read/write config
	conf, err := readConf(confPath)
	if os.IsNotExist(err) || confArg {
		conf, err = config(confPath)
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

	// ensure store path exist
	pathHelper, err := newStorePath(conf.RootPath)
	if err != nil {
		log.Fatalln("failed to make store dir:", err)
	}

	// sign in
	client, screenName, err := twitter.Login(ctx, conf.Cookie.AuthCoken, conf.Cookie.Ct0)
	if err != nil {
		log.Fatalln("failed to login:", err)
	}
	twitter.EnableRateLimit(client)
	log.Infoln("signed in as:", color.FgLightBlue.Render(screenName))
	if dbg {
		client.SetLogger(log.WithField("client", "resty"))
	}

	// load previous tweets
	dumper := downloading.NewDumper()
	err = dumper.Load(pathHelper.errorj)
	if err != nil {
		log.Fatalln("failed to load previous tweets", err)
	}
	log.Infoln("loaded previous failed tweets:", dumper.Count())

	// collect tasks
	task, err := MakeTask(ctx, client, usrArgs, listArgs, follArgs)
	if err != nil {
		log.Fatalln("failed to parse cmd args:", err)
	}

	// connect db
	db, err := connectDatabase(pathHelper.db)
	if err != nil {
		log.Fatalln("failed to connect to database:", err)
	}
	defer db.Close()
	log.Infoln("database is connected")

	// listen signal
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

	// dump failed tweets at exit
	var todump = make([]*downloading.TweetInEntity, 0)
	defer func() {
		dumper.Dump(pathHelper.errorj)
		log.Infof("%d tweets have been dumped and will be downloaded the next time the program runs", dumper.Count())
	}()

	defer func() {
		for _, te := range todump {
			dumper.Push(te.Entity.Id(), te.Tweet)
		}
		if ctx.Err() == nil {
			retryFailedTweets(ctx, dumper, db, client)
		}
	}()

	// do job
	log.Infoln("start working for...")
	printTask(task)

	if len(task.users) != 0 {
		todump, err = downloading.BatchUserDownload(ctx, client, db, task.users, pathHelper.users, nil)
		if err != nil {
			log.Errorln("failed to download users:", err)
			return
		}
	}

	for _, list := range task.lists {
		log.Debugln(list.Title())
		fails, err := downloading.DownloadList(ctx, client, db, list, pathHelper.root, pathHelper.users)
		todump = append(todump, fails...)
		if err != nil {
			log.WithField("list", list.Title()).Errorln("failed to download list:", err)
			return
		}
	}
}

func connectDatabase(path string) (*sqlx.DB, error) {
	ex, err := utils.PathExists(path)
	if err != nil {
		return nil, err
	}

	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&busy_timeout=120000", path)
	db, err := sqlx.Connect("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	database.CreateTables(db)
	//db.SetMaxOpenConns(1)
	if !ex {
		log.Debugln("created new db file", path)
	}
	return db, nil
}

func readConf(path string) (*Config, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var result Config
	err = yaml.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func writeConf(path string, conf *Config) error {
	file, err := os.OpenFile(path, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := yaml.Marshal(conf)
	if err != nil {
		return err
	}
	_, err = io.Copy(file, bytes.NewReader(data))
	return err
}

func config(saveto string) (*Config, error) {
	conf := Config{}
	scan := bufio.NewScanner(os.Stdin)

	print("enter storage dir: ")
	scan.Scan()
	storePath := scan.Text()
	// 确保路径可用
	err := os.MkdirAll(storePath, 0755)
	if err != nil {
		return nil, err
	}
	storePath, err = filepath.Abs(storePath)
	if err != nil {
		return nil, err
	}

	conf.RootPath = storePath

	print("enter auth_token: ")
	scan.Scan()
	conf.Cookie.AuthCoken = scan.Text()

	print("enter ct0: ")
	scan.Scan()
	conf.Cookie.Ct0 = scan.Text()

	print("enter max download routine: ")
	scan.Scan()
	conf.MaxDownloadRoutine, err = strconv.Atoi(scan.Text())
	if err != nil {
		return nil, err
	}

	return &conf, writeConf(saveto, &conf)
}

func retryFailedTweets(ctx context.Context, dumper *downloading.TweetDumper, db *sqlx.DB, client *resty.Client) error {
	if dumper.Count() == 0 {
		return nil
	}

	log.Infoln("starting to retry failed tweets")
	legacy, err := dumper.GetTotal(db)
	if err != nil {
		return err
	}

	toretry := make([]downloading.PackgedTweet, 0, len(legacy))
	for _, leg := range legacy {
		toretry = append(toretry, leg)
	}

	newFails := downloading.BatchDownloadTweet(ctx, client, toretry...)
	dumper.Clear()
	for _, pt := range newFails {
		te := pt.(*downloading.TweetInEntity)
		dumper.Push(te.Entity.Id(), te.Tweet)
	}

	return nil
}
