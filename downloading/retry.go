package downloading

import (
	"bytes"
	"encoding/json"
	"io"
	"os"

	"github.com/unkmonster/tmd2/internal/utils"
	"github.com/unkmonster/tmd2/twitter"
)

type PackagedTweet struct {
	data  map[int][]*twitter.Tweet
	set   map[int]map[uint64]struct{}
	init  bool
	count int
}

func (pt *PackagedTweet) initialize() {
	pt.data = make(map[int][]*twitter.Tweet)
	pt.set = make(map[int]map[uint64]struct{})
	pt.count = 0
}

func (pt *PackagedTweet) Push(eid int, tweets ...*twitter.Tweet) {
	if !pt.init {
		pt.initialize()
		pt.init = true
	}

	if _, ok := pt.data[eid]; !ok {
		pt.data[eid] = []*twitter.Tweet{}
		pt.set[eid] = map[uint64]struct{}{}
	}
	for _, tw := range tweets {
		_, exist := pt.set[eid][tw.Id]
		if exist {
			continue
		}

		arr := pt.data[eid]
		arr = append(arr, tw)
		pt.data[eid] = arr
		pt.set[eid][tw.Id] = struct{}{}
		pt.count++
	}
}

func (pt *PackagedTweet) Dump(path string) error {
	exist, err := utils.PathExists(path)
	if err != nil {
		return err
	}
	if exist {
		pt.Load(path)
	}

	data, err := json.MarshalIndent(&pt.data, "", "    ")
	if err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, bytes.NewReader(data))
	return err
}

func (pt *PackagedTweet) Load(path string) error {
	newPt := PackagedTweet{}
	file, err := os.OpenFile(path, os.O_RDONLY, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &newPt.data)
	if err != nil {
		return err
	}

	for eid, arr := range newPt.data {
		pt.Push(eid, arr...)
	}
	return nil
}

func (pt *PackagedTweet) Clear() {
	pt.initialize()
}

func (pt *PackagedTweet) Data() map[int][]*twitter.Tweet {
	newMap := map[int][]*twitter.Tweet{}
	for k, v := range pt.data {
		newMap[k] = make([]*twitter.Tweet, len(v))
		copy(newMap[k], v)
	}
	return newMap
}

func (pt *PackagedTweet) Count() int {
	return pt.count
}
