package twitter

import (
	"fmt"
	"io"
	"net/http"

	"github.com/tidwall/gjson"
)

type itemContent struct {
	*gjson.Result
}

func (itemcontent itemContent) GetItemType() string {
	return itemcontent.Get("itemType").String()
}

func (itemcontent itemContent) GetUserResults() gjson.Result {
	return itemcontent.Get("user_results")
}

func (itemcontent itemContent) GetTweetResults() gjson.Result {
	return itemcontent.Get("tweet_results")
}

type entriesMethod interface {
	GetNextCursor() string
	GetItemContents() []itemContent
}

type instructionsMethod interface {
	GetEntries() entriesMethod
}

type timelineMethod interface {
	GetInstructions() instructionsMethod
}

type itemEntries struct {
	gjson.Result
}

func (entries *itemEntries) GetNextCursor() string {
	array := entries.Array()
	for i := len(array) - 1; i >= 0; i-- {
		if array[i].Get("content.entryType").String() == "TimelineTimelineCursor" &&
			array[i].Get("content.cursorType").String() == "Bottom" {
			return array[i].Get("content.value").String()
		}
	}
	panic("invalid entries")
}

func (entries *itemEntries) GetItemContents() []itemContent {
	itemContents := entries.Get("#.content.itemContent").Array()
	results := make([]itemContent, len(itemContents))
	for i := 0; i < len(itemContents); i++ {
		results[i] = itemContent{&itemContents[i]}
	}
	return results
}

type itemInstructions struct {
	*gjson.Result
}

func (insts *itemInstructions) GetEntries() entriesMethod {
	for _, inst := range insts.Array() {
		if inst.Get("type").String() == "TimelineAddEntries" {
			return &itemEntries{inst.Get("entries")}
		}
	}
	panic("invalid instructions")
}

type moduleEntries struct {
	itemEntries
}

func (entries *moduleEntries) GetItemContents() []itemContent {
	itemContents := entries.Get("0.content.items.#.item.itemContent").Array()
	results := make([]itemContent, len(itemContents))
	for i := 0; i < len(itemContents); i++ {
		results[i] = itemContent{&itemContents[i]}
	}
	return results
}

type moduleInstructions struct {
	itemInstructions
}

func (insts *moduleInstructions) GetEntries() entriesMethod {
	r := insts.itemInstructions.GetEntries()
	pe := r.(*itemEntries)
	return &moduleEntries{*pe}
}

func getTimeline(client *http.Client, api timelineApi) (string, error) {
	url := makeUrl(api)
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("%d %s", resp.StatusCode, data)
	}

	return string(data), nil
}
