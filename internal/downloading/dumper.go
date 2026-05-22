package downloading

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/entity"
	"github.com/unkmonster/tmd/internal/twitter"
)

type TweetDumper struct {
	data  map[int][]*twitter.Tweet
	set   map[int]map[uint64]struct{}
	count int
}

func NewDumper() *TweetDumper {
	td := TweetDumper{}
	td.data = make(map[int][]*twitter.Tweet)
	td.set = make(map[int]map[uint64]struct{})
	return &td
}

func (td *TweetDumper) Push(eid int, tweet ...*twitter.Tweet) int {
	_, ok := td.data[eid]
	if !ok {
		td.data[eid] = make([]*twitter.Tweet, 0, len(tweet))
		td.set[eid] = make(map[uint64]struct{})
	}

	oldCount := td.count

	for _, tw := range tweet {
		_, exist := td.set[eid][tw.Id]
		if exist {
			continue
		}
		td.data[eid] = append(td.data[eid], tw)
		td.set[eid][tw.Id] = struct{}{}
		td.count++
	}
	return td.count - oldCount
}

func (td *TweetDumper) Load(path string) error {
	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	loaded := make(map[int][]*twitter.Tweet)
	err = json.Unmarshal(data, &loaded)
	if err != nil {
		return err
	}

	for k, v := range loaded {
		td.Push(k, v...)
	}
	return nil
}

func (td *TweetDumper) Dump(path string) error {
	data, err := json.MarshalIndent(td.data, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (td *TweetDumper) Clear() {
	td.data = make(map[int][]*twitter.Tweet)
	td.set = make(map[int]map[uint64]struct{})
	td.count = 0
}

func (td *TweetDumper) Merge(other *TweetDumper) {
	if td == nil || other == nil {
		return
	}
	for eid, tweets := range other.data {
		td.Push(eid, tweets...)
	}
}

func (td *TweetDumper) GetTotal(db *sqlx.DB) ([]*TweetInEntity, error) {
	results := make([]*TweetInEntity, 0, td.count)

	for k, v := range td.data {
		e, err := database.GetUserEntity(db, k)
		if err != nil {
			return nil, err
		}
		if e == nil {
			return nil, fmt.Errorf("entity %d does not exist", k)
		}

		ue := entity.NewUserEntityFromRecord(db, e)
		for _, tw := range v {
			results = append(results, &TweetInEntity{Tweet: tw, Entity: ue})
		}
	}
	return results, nil
}

func (td *TweetDumper) Count() int {
	return td.count
}

func (td *TweetDumper) EntityCount() int {
	return len(td.data)
}

func (td *TweetDumper) HasTweet(eid int, tweetID uint64) bool {
	tweets, ok := td.set[eid]
	if !ok {
		return false
	}
	_, ok = tweets[tweetID]
	return ok
}

func (td *TweetDumper) Remove(eid int, tweetID uint64) bool {
	if !td.HasTweet(eid, tweetID) {
		return false
	}
	delete(td.set[eid], tweetID)
	slice := td.data[eid]
	for i, tw := range slice {
		if tw.Id == tweetID {
			td.data[eid] = append(slice[:i], slice[i+1:]...)
			break
		}
	}
	if len(td.data[eid]) == 0 {
		delete(td.data, eid)
		delete(td.set, eid)
	}
	td.count--
	return true
}

type JsonDumpEntry struct {
	SourcePath string
	Type       string
	Tweets     []*twitter.Tweet
	Dirs       map[uint64]string
}

type JsonTweetDumper struct {
	data  map[string]*JsonDumpEntry
	set   map[string]map[uint64]struct{}
	count int
}

func NewJsonDumper() *JsonTweetDumper {
	d := JsonTweetDumper{}
	d.data = make(map[string]*JsonDumpEntry)
	d.set = make(map[string]map[uint64]struct{})
	return &d
}

func (d *JsonTweetDumper) Push(sourcePath string, entryType string, tweets ...*twitter.Tweet) int {
	e, ok := d.data[sourcePath]
	if !ok {
		e = &JsonDumpEntry{SourcePath: sourcePath, Type: entryType, Dirs: make(map[uint64]string)}
		d.data[sourcePath] = e
		d.set[sourcePath] = make(map[uint64]struct{})
	}
	oldCount := d.count
	for _, tw := range tweets {
		if _, exists := d.set[sourcePath][tw.Id]; exists {
			continue
		}
		e.Tweets = append(e.Tweets, tw)
		d.set[sourcePath][tw.Id] = struct{}{}
		d.count++
	}
	return d.count - oldCount
}

func (d *JsonTweetDumper) PushWithDir(sourcePath string, entryType string, dir string, tweet *twitter.Tweet) bool {
	e, ok := d.data[sourcePath]
	if !ok {
		e = &JsonDumpEntry{SourcePath: sourcePath, Type: entryType, Dirs: make(map[uint64]string)}
		d.data[sourcePath] = e
		d.set[sourcePath] = make(map[uint64]struct{})
	}
	if _, exists := d.set[sourcePath][tweet.Id]; exists {
		return false
	}
	e.Tweets = append(e.Tweets, tweet)
	e.Dirs[tweet.Id] = dir
	d.set[sourcePath][tweet.Id] = struct{}{}
	d.count++
	return true
}

func (d *JsonTweetDumper) Load(path string) error {
	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	var loaded map[string]*JsonDumpEntry
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}
	for k, v := range loaded {
		d.Push(k, v.Type, v.Tweets...)
		for tweetID, dir := range v.Dirs {
			if entry, ok := d.data[k]; ok {
				entry.Dirs[tweetID] = dir
			}
		}
	}
	return nil
}

func (d *JsonTweetDumper) Dump(path string) error {
	data, err := json.MarshalIndent(d.data, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (d *JsonTweetDumper) Clear() {
	d.data = make(map[string]*JsonDumpEntry)
	d.set = make(map[string]map[uint64]struct{})
	d.count = 0
}

func (d *JsonTweetDumper) Merge(other *JsonTweetDumper) {
	if d == nil || other == nil {
		return
	}
	for sourcePath, entry := range other.data {
		d.Push(sourcePath, entry.Type, entry.Tweets...)
		for tweetID, dir := range entry.Dirs {
			if e, ok := d.data[sourcePath]; ok {
				e.Dirs[tweetID] = dir
			}
		}
	}
}

func (d *JsonTweetDumper) GetTotal() []JsonPackagedTweet {
	results := make([]JsonPackagedTweet, 0, d.count)
	for _, entry := range d.data {
		for _, tw := range entry.Tweets {
			dir := entry.Dirs[tw.Id]
			results = append(results, JsonPackagedTweet{Tweet: tw, Dir: dir})
		}
	}
	return results
}

func (d *JsonTweetDumper) Count() int {
	return d.count
}

func (d *JsonTweetDumper) EntryCount() int {
	return len(d.data)
}

func (d *JsonTweetDumper) HasTweet(sourcePath string, tweetID uint64) bool {
	tweets, ok := d.set[sourcePath]
	if !ok {
		return false
	}
	_, ok = tweets[tweetID]
	return ok
}

func (d *JsonTweetDumper) Remove(sourcePath string, tweetID uint64) bool {
	if !d.HasTweet(sourcePath, tweetID) {
		return false
	}
	delete(d.set[sourcePath], tweetID)
	entry := d.data[sourcePath]
	for i, tw := range entry.Tweets {
		if tw.Id == tweetID {
			entry.Tweets = append(entry.Tweets[:i], entry.Tweets[i+1:]...)
			break
		}
	}
	if len(entry.Tweets) == 0 {
		delete(d.data, sourcePath)
		delete(d.set, sourcePath)
	}
	d.count--
	return true
}
