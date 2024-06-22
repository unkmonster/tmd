package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/gookit/color"
	"github.com/jmoiron/sqlx"
	"github.com/unkmonster/tmd2/database"
	"github.com/unkmonster/tmd2/downloading"
	"github.com/unkmonster/tmd2/twitter"
	"gopkg.in/yaml.v3"
)

type Config struct {
	RootPath string `yaml:"root_path"`
	Cookie   string `yaml:"cookie"`
	Token    string `yaml:"token"`
}

type userArgs struct {
	id         []uint64
	screenName []string
}

func (u *userArgs) GetUser(client *resty.Client) ([]*twitter.User, error) {
	users := []*twitter.User{}
	for _, id := range u.id {
		usr, err := twitter.GetUserById(client, id)
		if err != nil {
			return nil, err
		}
		users = append(users, usr)
	}

	for _, screenName := range u.screenName {
		usr, err := twitter.GetUserByScreenName(client, screenName)
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

func (l ListArgs) GetList(client *resty.Client) ([]*twitter.List, error) {
	lists := []*twitter.List{}
	for _, id := range l.id {
		list, err := twitter.GetLst(client, id)
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

func (t Task) String() string {
	//var buf bytes.Buffer
	users := make([]string, 0, len(t.users))
	lists := make([]string, 0, len(t.lists))

	for _, u := range t.users {
		users = append(users, u.Title())
	}
	for _, l := range t.lists {
		lists = append(lists, l.Title())
	}

	return fmt.Sprintf("user task: %v\nlist task: %v", users, lists)
}

func MakeTask(client *resty.Client, usrArgs userArgs, listArgs ListArgs, follArgs userArgs) (*Task, error) {
	task := Task{}
	task.users = make([]*twitter.User, 0)
	task.lists = make([]twitter.ListBase, 0)

	users, err := usrArgs.GetUser(client)
	if err != nil {
		return nil, err
	}
	task.users = append(task.users, users...)

	lists, err := listArgs.GetList(client)
	if err != nil {
		return nil, err
	}
	for _, list := range lists {
		task.lists = append(task.lists, list)
	}

	// fo
	users, err = follArgs.GetUser(client)
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		task.lists = append(task.lists, user.Following())
	}
	return &task, nil
}

type PathHelper struct {
	root   string
	users  string
	data   string
	db     string
	errorj string
}

func NewPathHelper(root string) (*PathHelper, error) {
	ph := PathHelper{}
	ph.root = root
	ph.users = filepath.Join(root, "users")
	ph.data = filepath.Join(root, "data")

	ph.db = filepath.Join(ph.data, "foo.db")
	ph.errorj = filepath.Join(ph.data, "errors.json")

	// ensure folder exist
	err := os.Mkdir(ph.root, 0655)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}

	err = os.Mkdir(ph.users, 0655)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}

	err = os.Mkdir(ph.data, 0655)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	return &ph, nil
}

func main() {
	//flag.
	var usrArgs userArgs
	var listArgs ListArgs
	var follArgs userArgs
	var confArg bool
	flag.BoolVar(&confArg, "conf", false, "to configure")
	flag.Var(&usrArgs, "user", "uid/screen_name to download tweets of specified user")
	flag.Var(&listArgs, "list", "list id to download specified list")
	flag.Var(&follArgs, "foll", "uid/screen_name to download following of specified user")
	flag.Parse()

	appdata := os.Getenv("appdata")
	if appdata == "" {
		panic("appdata is not exists")
	}
	appRootPath := filepath.Join(appdata, "tmd2")
	confPath := filepath.Join(appRootPath, "conf.yaml")
	err := os.Mkdir(appRootPath, 0666)
	if err != nil && !os.IsExist(err) {
		log.Fatalln("failed to make app dir:", err)
	}

	conf, err := readConf(confPath)
	if os.IsNotExist(err) || confArg {
		conf, err = config(confPath)
	}
	if err != nil {
		log.Fatalln("failed to load config:", err)
	}
	if confArg {
		return
	}

	// ensure path exist
	pathHelper, err := NewPathHelper(conf.RootPath)
	if err != nil {
		log.Fatalln("failed to make dir:", err)
	}

	// logging
	client, screenName, err := twitter.Login(conf.Cookie, conf.Token)
	if err != nil {
		log.Fatalln("failed to login:", err)
	}
	twitter.EnableRateLimit(client)
	fmt.Printf("Logged in as %s\n", color.FgLightBlue.Render(screenName))

	// connect db
	db, err := connectDatabase(pathHelper.db)
	if err != nil {
		log.Fatalln("failed to connect to database:", err)
	}
	defer db.Close()

	task, err := MakeTask(client, usrArgs, listArgs, follArgs)
	if err != nil {
		log.Fatalln("failed to parse args:", err)
	}
	fmt.Println(task)

	// do job
	dumper := downloading.NewDumper()
	err = dumper.Load(pathHelper.errorj)
	if err != nil {
		log.Fatalln("failed to load dumped tweets:", err)
	}

	// retry for legacy
	if dumper.Count() != 0 {
		fmt.Println("loaded legacy tweet:", dumper.Count())
		legacy, err := dumper.GetTotal(db)
		if err != nil {
			log.Fatalln("[Dumper] failed to get total tweet:", err)
		}

		toretry := make([]downloading.PackgedTweet, 0, len(legacy))
		for _, leg := range legacy {
			toretry = append(toretry, leg)
		}

		newFails := downloading.BatchDownloadTweet(client, toretry...)
		dumper.Clear()
		for _, pt := range newFails {
			te := pt.(*downloading.TweetInEntity)
			dumper.Push(te.Entity.Id(), te.Tweet)
		}
	}

	var todump = make([]*downloading.TweetInEntity, 0)
	defer func() {
		count := 0
		for _, te := range todump {
			count += dumper.Push(te.Entity.Id(), te.Tweet)
		}
		dumper.Dump(pathHelper.errorj)
		fmt.Printf("%d tweet have been dumped\n", count)
	}()

	if len(task.users) != 0 {
		todump = downloading.BatchUserDownload(client, db, task.users, pathHelper.users, nil)
	}
	for _, list := range task.lists {
		fails, err := downloading.DownloadList(client, db, list, pathHelper.root, pathHelper.users)
		if err != nil {
			fmt.Printf("failed to download list [%s]: %v\n", list.Title(), err)
			continue
		}
		todump = append(todump, fails...)
	}
}

func connectDatabase(path string) (*sqlx.DB, error) {
	dsn := fmt.Sprintf("file:%s?cache=shared", path)
	db, err := sqlx.Connect("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	database.CreateTables(db)
	db.SetMaxOpenConns(1)
	return db, nil
}

func readConf(path string) (*Config, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, 0666)
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
	scan := bufio.NewScanner(os.Stdin)
	print("enter storage dir: ")
	scan.Scan()
	storePath := scan.Text()
	storePath, err := filepath.Abs(storePath)
	if err != nil {
		return nil, err
	}

	print("enter cookie: ")
	scan.Scan()
	cookie := scan.Text()

	print("enter token: ")
	scan.Scan()
	token := scan.Text()

	// 确保路径可用
	err = os.MkdirAll(storePath, 0755)
	if err != nil {
		return nil, err
	}

	conf := Config{}
	conf.Cookie = cookie
	conf.RootPath = storePath
	conf.Token = token
	return &conf, writeConf(saveto, &conf)
}
