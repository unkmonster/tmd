package downloading

import (
	"encoding/json"
	"io"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/unkmonster/tmd2/database"
	"github.com/unkmonster/tmd2/twitter"
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
	return os.WriteFile(path, data, 0666)
}

func (td *TweetDumper) Clear() {
	td.data = make(map[int][]*twitter.Tweet)
	td.set = make(map[int]map[uint64]struct{})
	td.count = 0
}

func (td *TweetDumper) GetTotal(db *sqlx.DB) ([]*TweetInEntity, error) {
	results := make([]*TweetInEntity, 0, td.count)

	for k, v := range td.data {
		e, err := database.GetUserEntity(db, k)
		if err != nil {
			return nil, err
		}
		ue := UserEntity{db: db, entity: e}

		for _, tw := range v {
			results = append(results, &TweetInEntity{Tweet: tw, Entity: &ue})
		}
	}
	return results, nil
}

func (td *TweetDumper) Count() int {
	return td.count
}
