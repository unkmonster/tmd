package utils

import "strings"

const twitterImageQuality4096 = "4096x4096"
const twitterImageQualityOrig = "orig"

func isHighestTwitterImageQuality(name string) bool {
	return name == twitterImageQualityOrig || name == twitterImageQuality4096
}

func isTwitterVideoMediaURL(rawURL string) bool {
	return strings.Contains(rawURL, "tweet_video") || strings.Contains(rawURL, "video.twimg.com")
}
