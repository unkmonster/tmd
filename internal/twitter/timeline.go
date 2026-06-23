package twitter

import (
	"context"
	"fmt"

	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
)

const (
	timelineTweet = iota
	timelineUser
)

func getInstructions(resp []byte, path string) (gjson.Result, error) {
	inst := gjson.GetBytes(resp, path)
	if !inst.Exists() {
		typeName := gjson.GetBytes(resp, "data.user.result.__typename").String()
		if typeName != "" && typeName != "User" {
			return gjson.Result{}, fmt.Errorf("user unavailable: __typename is %s", typeName)
		}
		return gjson.Result{}, fmt.Errorf("unable to get instructions: %s path: '%s'", resp, path)
	}
	return inst, nil
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

func getNextCursor(entries gjson.Result) (string, error) {
	array := entries.Array()
	// if len(array) == 2 {
	// 	return "" // no next page
	// }

	for i := len(array) - 1; i >= 0; i-- {
		if array[i].Get("content.entryType").String() == "TimelineTimelineCursor" &&
			array[i].Get("content.cursorType").String() == "Bottom" {
			return array[i].Get("content.value").String(), nil
		}
	}

	return "", fmt.Errorf("invalid entries: %s", entries.String())
}

func getItemContentFromModuleItem(moduleItem gjson.Result) (gjson.Result, error) {
	res := moduleItem.Get("item.itemContent")
	if !res.Exists() {
		return gjson.Result{}, fmt.Errorf("invalid ModuleItem: %s", moduleItem.String())
	}
	return res, nil
}

func getItemContentsFromEntry(entry gjson.Result) ([]gjson.Result, error) {
	content := entry.Get("content")
	ty := content.Get("entryType").String()
	if ty == "TimelineTimelineModule" {
		return content.Get("items.#.item.itemContent").Array(), nil
	} else if ty == "TimelineTimelineItem" {
		return []gjson.Result{content.Get("itemContent")}, nil
	}

	return nil, fmt.Errorf("invalid entry: %s", entry.String())
}

func getResults(itemContent gjson.Result, itemType int) gjson.Result {
	if itemType == timelineTweet {
		return itemContent.Get("tweet_results")
	} else if itemType == timelineUser {
		return itemContent.Get("user_results")
	}

	return gjson.Result{}
}

func getTimelineResp(ctx context.Context, api timelineApi, client *resty.Client) ([]byte, error) {
	url := makeUrl(api)
	resp, err := client.R().SetContext(ctx).Get(url)
	if err != nil {
		return nil, err
	}
	return resp.Body(), nil
}

// 获取时间线 API 并返回所有 itemContent 和 底部 cursor
func getTimelineItemContents(ctx context.Context, api timelineApi, client *resty.Client, instPath string) ([]gjson.Result, string, error) {
	resp, err := getTimelineResp(ctx, api, client)
	if err != nil {
		return nil, "", err
	}

	// is temporarily unavailable because it violates the Twitter Media Policy.
	// Protected User's following: Permission denied
	if string(resp) == "{\"data\":{\"user\":{}}}" {
		return nil, "", nil
	}
	instructions, err := getInstructions(resp, instPath)
	if err != nil {
		return nil, "", err
	}
	entries := getEntries(instructions)
	moduleItems := getModuleItems(instructions)
	if !entries.Exists() && !moduleItems.Exists() {
		return nil, "", fmt.Errorf("invalid instructions: %s", instructions.String())
	}

	itemContents := make([]gjson.Result, 0)
	if entries.IsArray() {
		for _, entry := range entries.Array() {
			if entry.Get("content.entryType").String() != "TimelineTimelineCursor" {
				contents, err := getItemContentsFromEntry(entry)
				if err != nil {
					return nil, "", err
				}
				itemContents = append(itemContents, contents...)
			}
		}
	}
	if moduleItems.IsArray() {
		for _, moduleItem := range moduleItems.Array() {
			content, err := getItemContentFromModuleItem(moduleItem)
			if err != nil {
				return nil, "", err
			}
			itemContents = append(itemContents, content)
		}
	}
	cursor, err := getNextCursor(entries)
	if err != nil {
		return nil, "", err
	}
	return itemContents, cursor, nil
}

func getTimelineItemContentsTillEnd(ctx context.Context, api timelineApi, client *resty.Client, instPath string) ([]gjson.Result, error) {
	res := make([]gjson.Result, 0)

	for {
		page, next, err := getTimelineItemContents(ctx, api, client, instPath)
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
