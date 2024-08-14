package twitter

import (
	"fmt"

	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
	"github.com/unkmonster/tmd2/internal/utils"
)

const (
	timelineTweet = iota
	timelineUser
)

func getInstructions(resp []byte, path string) gjson.Result {
	return gjson.GetBytes(resp, path)
}

func getEntries(instructions gjson.Result) gjson.Result {
	for _, inst := range instructions.Array() {
		if inst.Get("type").String() == "TimelineAddEntries" {
			return inst.Get("entries")
		}
	}
	return gjson.Result{}
}

func getModuleItems(instructions gjson.Result) gjson.Result {
	for _, inst := range instructions.Array() {
		if inst.Get("type").String() == "TimelineAddToModule" {
			return inst.Get("moduleItems")
		}
	}
	return gjson.Result{}
}

func getNextCursor(entries gjson.Result) string {
	array := entries.Array()
	// if len(array) == 2 {
	// 	return "" // no next page
	// }

	for i := len(array) - 1; i >= 0; i-- {
		if array[i].Get("content.entryType").String() == "TimelineTimelineCursor" &&
			array[i].Get("content.cursorType").String() == "Bottom" {
			return array[i].Get("content.value").String()
		}
	}

	panic(fmt.Sprintf("invalid entries: %s", entries.String()))
}

func getItemContentFromModuleItem(moduleItem gjson.Result) gjson.Result {
	return moduleItem.Get("item.itemContent")
}

func getItemContentsFromEntry(entry gjson.Result) []gjson.Result {
	content := entry.Get("content")
	ty := content.Get("entryType").String()
	if ty == "TimelineTimelineModule" {
		return content.Get("items.#.item.itemContent").Array()
	} else if ty == "TimelineTimelineItem" {
		return []gjson.Result{content.Get("itemContent")}
	}

	panic(fmt.Sprintf("invalid entry: %s", entry.String()))
}

func getResults(itemContent gjson.Result, itemType int) gjson.Result {
	if itemType == timelineTweet {
		return itemContent.Get("tweet_results")
	} else if itemType == timelineUser {
		return itemContent.Get("user_results")
	}

	panic(fmt.Sprintf("invalid itemContent: %s", itemContent.String()))
}

func getTimelineResp(api timelineApi, client *resty.Client) ([]byte, error) {
	url := makeUrl(api)
	resp, err := client.R().Get(url)
	if err != nil {
		return nil, err
	}
	if err = utils.CheckRespStatus(resp); err != nil {
		return nil, err
	}

	return resp.Body(), nil
}

// 获取时间线 API 并返回所有 itemContent 和 底部 cursor
func getTimelineItemContents(api timelineApi, client *resty.Client, instPath string) ([]gjson.Result, string, error) {
	resp, err := getTimelineResp(api, client)
	if err != nil {
		return nil, "", err
	}

	instructions := getInstructions(resp, instPath)
	entries := getEntries(instructions)
	moduleItems := getModuleItems(instructions)

	itemContents := make([]gjson.Result, 0)

	if entries.IsArray() {
		for _, entry := range entries.Array() {
			if entry.Get("content.entryType").String() != "TimelineTimelineCursor" {
				itemContents = append(itemContents, getItemContentsFromEntry(entry)...)
			}
		}
	}

	if moduleItems.IsArray() {
		for _, moduleItem := range moduleItems.Array() {
			itemContents = append(itemContents, getItemContentFromModuleItem(moduleItem))
		}
	}

	return itemContents, getNextCursor(entries), nil
}

func getTimelineItemContentsTillEnd(api timelineApi, client *resty.Client, instPath string) ([]gjson.Result, error) {
	res := make([]gjson.Result, 0)

	for {
		page, next, err := getTimelineItemContents(api, client, instPath)
		if err != nil {
			return nil, err
		}

		if len(page) == 0 {
			break // empty page
		}

		res = append(res, page...)
		api.SetCursor(next)
	}

	return res, nil
}
