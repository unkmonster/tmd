package downloading

import (
	"encoding/json"

	"github.com/unkmonster/tmd/internal/utils"
)

// ========== Source 结构体（新格式 - 第三方工具导出）==========

type sourceTweet struct {
	Typename           string          `json:"__typename"`
	Core               sourceCore      `json:"core"`
	EditControl        editControl     `json:"edit_control"`
	IsTranslatable     bool            `json:"is_translatable"`
	Legacy             sourceLegacy    `json:"legacy"`
	RestID             string          `json:"rest_id"`
	Source             string          `json:"source"`
	UnmentionData      json.RawMessage `json:"unmention_data"`
	Views              views           `json:"views"`
	GrokAnalysisButton bool            `json:"grok_analysis_button,omitempty"`
	GrokAnnotations    json.RawMessage `json:"grok_annotations,omitempty"`
	GrokTranslatedPost json.RawMessage `json:"grok_translated_post_with_availability,omitempty"`
	TwePrivateFields   json.RawMessage `json:"twe_private_fields,omitempty"`
}

type sourceCore struct {
	UserResults sourceUserResults `json:"user_results"`
}

type sourceUserResults struct {
	Result sourceUser `json:"result"`
}

type sourceUser struct {
	Typename                   string                    `json:"__typename"`
	AffiliatesHighlightedLabel map[string]interface{}    `json:"affiliates_highlighted_label"`
	HasGraduatedAccess         bool                      `json:"has_graduated_access"`
	IsBlueVerified             bool                      `json:"is_blue_verified"`
	Avatar                     *avatar                   `json:"avatar,omitempty"`
	Core                       userCore                  `json:"core"`
	DmPermissions              *dmPermissions            `json:"dm_permissions,omitempty"`
	FollowRequestSent          bool                      `json:"follow_request_sent"`
	GrokTranslatedBio          json.RawMessage           `json:"grok_translated_bio_with_availability,omitempty"`
	ID                         string                    `json:"id"`
	Legacy                     userLegacy                `json:"legacy"`
	Location                   *location                 `json:"location,omitempty"`
	MediaPermissions           *mediaPermissions         `json:"media_permissions,omitempty"`
	ParodyCommentaryFanLabel   string                    `json:"parody_commentary_fan_label,omitempty"`
	Privacy                    *privacy                  `json:"privacy,omitempty"`
	ProfileBio                 *profileBio               `json:"profile_bio,omitempty"`
	ProfileDescriptionLanguage string                    `json:"profile_description_language,omitempty"`
	ProfileImageShape          string                    `json:"profile_image_shape"`
	RelationshipPerspectives   *relationshipPerspectives `json:"relationship_perspectives,omitempty"`
	RestID                     string                    `json:"rest_id"`
	SuperFollowEligible        bool                      `json:"super_follow_eligible"`
	SuperFollowedBy            bool                      `json:"super_followed_by"`
	SuperFollowing             bool                      `json:"super_following"`
	TipjarSettings             map[string]interface{}    `json:"tipjar_settings,omitempty"`
	Verification               *verification             `json:"verification,omitempty"`
}

type profileBio struct {
	Description string `json:"description"`
}

type avatar struct {
	ImageURL string `json:"image_url"`
}

type userCore struct {
	CreatedAt  string `json:"created_at"`
	Name       string `json:"name"`
	ScreenName string `json:"screen_name"`
}

type dmPermissions struct {
	CanDm bool `json:"can_dm"`
}

type location struct {
	Location string `json:"location"`
}

type mediaPermissions struct {
	CanMediaTag bool `json:"can_media_tag"`
}

type privacy struct {
	Protected bool `json:"protected"`
}

type relationshipPerspectives struct {
	BlockedBy  bool `json:"blocked_by"`
	Blocking   bool `json:"blocking"`
	FollowedBy bool `json:"followed_by"`
	Following  bool `json:"following"`
	Muting     bool `json:"muting"`
}

type verification struct {
	Verified bool `json:"verified"`
}

type editControl struct {
	EditTweetIDs       []string `json:"edit_tweet_ids"`
	EditableUntilMsecs string   `json:"editable_until_msecs"`
	EditsRemaining     string   `json:"edits_remaining"`
	IsEditEligible     bool     `json:"is_edit_eligible"`
}

type views struct {
	Count string `json:"count"`
	State string `json:"state"`
}

type sourceLegacy struct {
	BookmarkCount             int               `json:"bookmark_count"`
	Bookmarked                bool              `json:"bookmarked"`
	ConversationIDStr         string            `json:"conversation_id_str"`
	CreatedAt                 string            `json:"created_at"`
	DisplayTextRange          []int             `json:"display_text_range"`
	Entities                  tweetEntities     `json:"entities"`
	ExtendedEntities          *extendedEntities `json:"extended_entities,omitempty"`
	FavoriteCount             int               `json:"favorite_count"`
	Favorited                 bool              `json:"favorited"`
	FullText                  string            `json:"full_text"`
	IDStr                     string            `json:"id_str,omitempty"`
	InReplyToScreenName       string            `json:"in_reply_to_screen_name,omitempty"`
	InReplyToUserIDStr        string            `json:"in_reply_to_user_id_str,omitempty"`
	IsQuoteStatus             bool              `json:"is_quote_status"`
	Lang                      string            `json:"lang"`
	PossiblySensitive         bool              `json:"possibly_sensitive"`
	PossiblySensitiveEditable bool              `json:"possibly_sensitive_editable"`
	QuoteCount                int               `json:"quote_count"`
	ReplyCount                int               `json:"reply_count"`
	RetweetCount              int               `json:"retweet_count"`
	Retweeted                 bool              `json:"retweeted"`
	UserIDStr                 string            `json:"user_id_str,omitempty"`
}

type userLegacy struct {
	DefaultProfile          bool         `json:"default_profile"`
	DefaultProfileImage     bool         `json:"default_profile_image"`
	Description             string       `json:"description"`
	Entities                userEntities `json:"entities"`
	FastFollowersCount      int          `json:"fast_followers_count"`
	FavouritesCount         int          `json:"favourites_count"`
	FollowRequestSent       bool         `json:"follow_request_sent"`
	FollowersCount          int          `json:"followers_count"`
	FriendsCount            int          `json:"friends_count"`
	HasCustomTimelines      bool         `json:"has_custom_timelines"`
	IsTranslator            bool         `json:"is_translator"`
	ListedCount             int          `json:"listed_count"`
	MediaCount              int          `json:"media_count"`
	NeedsPhoneVerification  bool         `json:"needs_phone_verification"`
	NormalFollowersCount    int          `json:"normal_followers_count"`
	Notifications           bool         `json:"notifications"`
	PinnedTweetIDsStr       []string     `json:"pinned_tweet_ids_str"`
	PossiblySensitive       bool         `json:"possibly_sensitive"`
	ProfileBannerURL        string       `json:"profile_banner_url,omitempty"`
	ProfileInterstitialType string       `json:"profile_interstitial_type,omitempty"`
	StatusesCount           int          `json:"statuses_count"`
	TimeZone                string       `json:"time_zone"`
	TranslatorType          string       `json:"translator_type"`
	URL                     string       `json:"url,omitempty"`
	UtcOffset               int          `json:"utc_offset"`
	WantRetweets            bool         `json:"want_retweets"`
	WithheldDescription     string       `json:"withheld_description"`
	WithheldScope           string       `json:"withheld_scope"`
}

type userEntities struct {
	Description entitiesDescription `json:"description"`
	URL         *urlEntities        `json:"url,omitempty"`
}

type urlEntities struct {
	URLs []entityURL `json:"urls"`
}

type entityURL struct {
	DisplayURL  string `json:"display_url"`
	ExpandedURL string `json:"expanded_url"`
	Indices     []int  `json:"indices"`
	URL         string `json:"url"`
}

type entitiesDescription struct {
	URLs []entityURL `json:"urls,omitempty"`
}

type hashtag struct {
	Indices []int  `json:"indices"`
	Text    string `json:"text"`
}

type userMention struct {
	IDStr      string `json:"id_str"`
	Indices    []int  `json:"indices"`
	Name       string `json:"name"`
	ScreenName string `json:"screen_name"`
}

type tweetEntities struct {
	Hashtags     []hashtag     `json:"hashtags,omitempty"`
	Media        []interface{} `json:"media,omitempty"`
	Smarttags    []interface{} `json:"smarttags,omitempty"`
	Symbols      []interface{} `json:"symbols,omitempty"`
	Timestamps   []interface{} `json:"timestamps,omitempty"`
	URLs         []interface{} `json:"urls,omitempty"`
	UserMentions []userMention `json:"user_mentions,omitempty"`
}

type extendedEntities struct {
	Media []media `json:"media"`
}

type media struct {
	AdditionalMediaInfo   map[string]interface{} `json:"additional_media_info,omitempty"`
	AllowDownloadStatus   map[string]interface{} `json:"allow_download_status,omitempty"`
	DisplayURL            string                 `json:"display_url"`
	ExpandedURL           string                 `json:"expanded_url"`
	ExtMediaAvailability  map[string]interface{} `json:"ext_media_availability,omitempty"`
	IDStr                 string                 `json:"id_str"`
	Indices               []int                  `json:"indices"`
	MediaKey              string                 `json:"media_key,omitempty"`
	MediaResults          map[string]interface{} `json:"media_results,omitempty"`
	MediaURLHTTPS         string                 `json:"media_url_https"`
	OriginalInfo          originalInfo           `json:"original_info"`
	SensitiveMediaWarning map[string]interface{} `json:"sensitive_media_warning,omitempty"`
	Sizes                 mediaSizes             `json:"sizes"`
	Type                  string                 `json:"type"`
	URL                   string                 `json:"url"`
	VideoInfo             *videoInfo             `json:"video_info,omitempty"`
}

type originalInfo struct {
	FocusRects []interface{} `json:"focus_rects,omitempty"`
	Height     int           `json:"height"`
	Width      int           `json:"width"`
}

type mediaSizes struct {
	Large  mediaSize `json:"large"`
	Medium mediaSize `json:"medium"`
	Small  mediaSize `json:"small"`
	Thumb  mediaSize `json:"thumb"`
}

type mediaSize struct {
	H      int    `json:"h"`
	Resize string `json:"resize"`
	W      int    `json:"w"`
}

type videoInfo struct {
	AspectRatio    []int          `json:"aspect_ratio"`
	DurationMillis int            `json:"duration_millis,omitempty"`
	Variants       []videoVariant `json:"variants"`
}

type videoVariant struct {
	Bitrate     int    `json:"bitrate,omitempty"`
	ContentType string `json:"content_type"`
	URL         string `json:"url"`
}

// ========== Target 结构体（旧格式 - 兼容 TMD）==========

type targetTweet struct {
	Typename       string          `json:"__typename,omitempty"`
	Core           targetCore      `json:"core"`
	EditControl    editControl     `json:"edit_control"`
	IsTranslatable bool            `json:"is_translatable"`
	Legacy         targetLegacy    `json:"legacy"`
	RestID         string          `json:"rest_id"`
	Source         string          `json:"source"`
	UnmentionData  json.RawMessage `json:"unmention_data"`
	Views          views           `json:"views"`
}

type targetCore struct {
	UserResults targetUserResults `json:"user_results"`
}

type targetUserResults struct {
	Result targetUser `json:"result"`
}

type targetUser struct {
	Typename                   string                 `json:"__typename"`
	AffiliatesHighlightedLabel map[string]interface{} `json:"affiliates_highlighted_label"`
	HasGraduatedAccess         bool                   `json:"has_graduated_access"`
	IsBlueVerified             bool                   `json:"is_blue_verified"`
	Legacy                     targetUserLegacy       `json:"legacy"`
	ProfileImageShape          string                 `json:"profile_image_shape"`
	RestID                     string                 `json:"rest_id"`
	SuperFollowEligible        bool                   `json:"super_follow_eligible"`
	SuperFollowedBy            bool                   `json:"super_followed_by"`
	SuperFollowing             bool                   `json:"super_following"`
	TipjarSettings             map[string]interface{} `json:"tipjar_settings"`
}

type targetUserLegacy struct {
	BlockedBy               bool               `json:"blocked_by"`
	Blocking                bool               `json:"blocking"`
	CanDm                   bool               `json:"can_dm"`
	CanMediaTag             bool               `json:"can_media_tag"`
	CreatedAt               string             `json:"created_at"`
	DefaultProfile          bool               `json:"default_profile"`
	DefaultProfileImage     bool               `json:"default_profile_image"`
	Description             string             `json:"description"`
	Entities                targetUserEntities `json:"entities"`
	FastFollowersCount      int                `json:"fast_followers_count"`
	FavouritesCount         int                `json:"favourites_count"`
	FollowRequestSent       bool               `json:"follow_request_sent"`
	FollowedBy              bool               `json:"followed_by"`
	FollowersCount          int                `json:"followers_count"`
	Following               bool               `json:"following"`
	FriendsCount            int                `json:"friends_count"`
	HasCustomTimelines      bool               `json:"has_custom_timelines"`
	IsTranslator            bool               `json:"is_translator"`
	ListedCount             int                `json:"listed_count"`
	Location                string             `json:"location,omitempty"`
	MediaCount              int                `json:"media_count"`
	Muting                  bool               `json:"muting"`
	Name                    string             `json:"name"`
	NeedsPhoneVerification  bool               `json:"needs_phone_verification"`
	NormalFollowersCount    int                `json:"normal_followers_count"`
	Notifications           bool               `json:"notifications"`
	PinnedTweetIDsStr       []string           `json:"pinned_tweet_ids_str"`
	PossiblySensitive       bool               `json:"possibly_sensitive"`
	ProfileBannerURL        string             `json:"profile_banner_url"`
	ProfileImageURLHTTPS    string             `json:"profile_image_url_https,omitempty"`
	ProfileInterstitialType string             `json:"profile_interstitial_type,omitempty"`
	Protected               bool               `json:"protected"`
	ScreenName              string             `json:"screen_name"`
	StatusesCount           int                `json:"statuses_count"`
	TimeZone                string             `json:"time_zone"`
	TranslatorType          string             `json:"translator_type"`
	URL                     string             `json:"url"`
	UtcOffset               int                `json:"utc_offset"`
	Verified                bool               `json:"verified"`
	WantRetweets            bool               `json:"want_retweets"`
	WithheldDescription     string             `json:"withheld_description"`
	WithheldScope           string             `json:"withheld_scope"`
}

type targetUserEntities struct {
	Description entitiesDescription `json:"description"`
	URL         *urlEntities        `json:"url,omitempty"`
}

type targetLegacy struct {
	BookmarkCount             int                 `json:"bookmark_count"`
	Bookmarked                bool                `json:"bookmarked"`
	ConversationIDStr         string              `json:"conversation_id_str"`
	CreatedAt                 string              `json:"created_at"`
	DisplayTextRange          []int               `json:"display_text_range"`
	Entities                  targetTweetEntities `json:"entities"`
	ExtendedEntities          *extendedEntities   `json:"extended_entities,omitempty"`
	FavoriteCount             int                 `json:"favorite_count"`
	Favorited                 bool                `json:"favorited"`
	FullText                  string              `json:"full_text"`
	InReplyToScreenName       string              `json:"in_reply_to_screen_name,omitempty"`
	InReplyToUserIDStr        string              `json:"in_reply_to_user_id_str,omitempty"`
	IsQuoteStatus             bool                `json:"is_quote_status"`
	Lang                      string              `json:"lang"`
	PossiblySensitive         bool                `json:"possibly_sensitive"`
	PossiblySensitiveEditable bool                `json:"possibly_sensitive_editable"`
	QuoteCount                int                 `json:"quote_count"`
	ReplyCount                int                 `json:"reply_count"`
	RetweetCount              int                 `json:"retweet_count"`
	Retweeted                 bool                `json:"retweeted"`
}

type targetTweetEntities struct {
	Hashtags     []hashtag     `json:"hashtags"`
	UserMentions []userMention `json:"user_mentions,omitempty"`
}

// ========== 核心转换函数 ==========

// ConvertThirdPartyTweetJSON 将第三方工具导出的新格式 JSON 转换为旧格式
// 用于 -jsonfile 模式保存兼容的 JSON 文件
func ConvertThirdPartyTweetJSON(metadata json.RawMessage) ([]byte, error) {
	var source sourceTweet
	if err := json.Unmarshal(metadata, &source); err != nil {
		return nil, err
	}

	target := convertSourceToTarget(&source)

	return json.MarshalIndent(target, "", "  ")
}

func convertSourceToTarget(source *sourceTweet) *targetTweet {
	sourceUser := source.Core.UserResults.Result
	sourceUserLegacy := sourceUser.Legacy
	sourceLegacy := source.Legacy

	targetUserLegacy := targetUserLegacy{
		BlockedBy:               getBool(sourceUser.RelationshipPerspectives, "blocked_by"),
		Blocking:                getBool(sourceUser.RelationshipPerspectives, "blocking"),
		CanDm:                   getBoolFromDmPermissions(sourceUser.DmPermissions),
		CanMediaTag:             getBoolFromMediaPermissions(sourceUser.MediaPermissions),
		CreatedAt:               sourceUser.Core.CreatedAt,
		DefaultProfile:          sourceUserLegacy.DefaultProfile,
		DefaultProfileImage:     sourceUserLegacy.DefaultProfileImage,
		Description:             sourceUserLegacy.Description,
		Entities:                convertUserEntities(sourceUserLegacy.Entities),
		FastFollowersCount:      sourceUserLegacy.FastFollowersCount,
		FavouritesCount:         sourceUserLegacy.FavouritesCount,
		FollowRequestSent:       sourceUser.FollowRequestSent,
		FollowedBy:              getBool(sourceUser.RelationshipPerspectives, "followed_by"),
		FollowersCount:          sourceUserLegacy.FollowersCount,
		Following:               getBool(sourceUser.RelationshipPerspectives, "following"),
		FriendsCount:            sourceUserLegacy.FriendsCount,
		HasCustomTimelines:      sourceUserLegacy.HasCustomTimelines,
		IsTranslator:            sourceUserLegacy.IsTranslator,
		ListedCount:             sourceUserLegacy.ListedCount,
		Location:                getLocation(sourceUser.Location),
		MediaCount:              sourceUserLegacy.MediaCount,
		Muting:                  getBool(sourceUser.RelationshipPerspectives, "muting"),
		Name:                    sourceUser.Core.Name,
		NeedsPhoneVerification:  sourceUserLegacy.NeedsPhoneVerification,
		NormalFollowersCount:    sourceUserLegacy.NormalFollowersCount,
		Notifications:           sourceUserLegacy.Notifications,
		PinnedTweetIDsStr:       sourceUserLegacy.PinnedTweetIDsStr,
		PossiblySensitive:       sourceUserLegacy.PossiblySensitive,
		ProfileBannerURL:        sourceUserLegacy.ProfileBannerURL,
		ProfileImageURLHTTPS:    getAvatarURL(sourceUser.Avatar),
		ProfileInterstitialType: sourceUserLegacy.ProfileInterstitialType,
		Protected:               getBoolFromPrivacy(sourceUser.Privacy),
		ScreenName:              sourceUser.Core.ScreenName,
		StatusesCount:           sourceUserLegacy.StatusesCount,
		TimeZone:                sourceUserLegacy.TimeZone,
		TranslatorType:          sourceUserLegacy.TranslatorType,
		URL:                     sourceUserLegacy.URL,
		UtcOffset:               sourceUserLegacy.UtcOffset,
		Verified:                getBoolFromVerification(sourceUser.Verification),
		WantRetweets:            sourceUserLegacy.WantRetweets,
		WithheldDescription:     sourceUserLegacy.WithheldDescription,
		WithheldScope:           sourceUserLegacy.WithheldScope,
	}

	tipjarSettings := sourceUser.TipjarSettings
	if tipjarSettings == nil {
		tipjarSettings = make(map[string]interface{})
	}

	targetUser := targetUser{
		Typename:                   sourceUser.Typename,
		AffiliatesHighlightedLabel: sourceUser.AffiliatesHighlightedLabel,
		HasGraduatedAccess:         sourceUser.HasGraduatedAccess,
		IsBlueVerified:             sourceUser.IsBlueVerified,
		Legacy:                     targetUserLegacy,
		ProfileImageShape:          sourceUser.ProfileImageShape,
		RestID:                     sourceUser.RestID,
		SuperFollowEligible:        sourceUser.SuperFollowEligible,
		SuperFollowedBy:            sourceUser.SuperFollowedBy,
		SuperFollowing:             sourceUser.SuperFollowing,
		TipjarSettings:             tipjarSettings,
	}

	targetTweetLegacy := targetLegacy{
		BookmarkCount:             sourceLegacy.BookmarkCount,
		Bookmarked:                sourceLegacy.Bookmarked,
		ConversationIDStr:         sourceLegacy.ConversationIDStr,
		CreatedAt:                 sourceLegacy.CreatedAt,
		DisplayTextRange:          sourceLegacy.DisplayTextRange,
		Entities:                  convertTweetEntities(sourceLegacy.Entities),
		ExtendedEntities:          cleanMediaHighQuality(sourceLegacy.ExtendedEntities),
		FavoriteCount:             sourceLegacy.FavoriteCount,
		Favorited:                 sourceLegacy.Favorited,
		FullText:                  sourceLegacy.FullText,
		InReplyToScreenName:       sourceLegacy.InReplyToScreenName,
		InReplyToUserIDStr:        sourceLegacy.InReplyToUserIDStr,
		IsQuoteStatus:             sourceLegacy.IsQuoteStatus,
		Lang:                      sourceLegacy.Lang,
		PossiblySensitive:         sourceLegacy.PossiblySensitive,
		PossiblySensitiveEditable: sourceLegacy.PossiblySensitiveEditable,
		QuoteCount:                sourceLegacy.QuoteCount,
		ReplyCount:                sourceLegacy.ReplyCount,
		RetweetCount:              sourceLegacy.RetweetCount,
		Retweeted:                 sourceLegacy.Retweeted,
	}

	return &targetTweet{
		Typename:       source.Typename,
		Core:           targetCore{UserResults: targetUserResults{Result: targetUser}},
		EditControl:    source.EditControl,
		IsTranslatable: source.IsTranslatable,
		Legacy:         targetTweetLegacy,
		RestID:         source.RestID,
		Source:         source.Source,
		UnmentionData:  source.UnmentionData,
		Views:          source.Views,
	}
}

// ========== 辅助函数 ==========

func getBool(rp *relationshipPerspectives, field string) bool {
	if rp == nil {
		return false
	}
	switch field {
	case "blocked_by":
		return rp.BlockedBy
	case "blocking":
		return rp.Blocking
	case "followed_by":
		return rp.FollowedBy
	case "following":
		return rp.Following
	case "muting":
		return rp.Muting
	}
	return false
}

func getBoolFromDmPermissions(dp *dmPermissions) bool {
	if dp == nil {
		return false
	}
	return dp.CanDm
}

func getBoolFromMediaPermissions(mp *mediaPermissions) bool {
	if mp == nil {
		return false
	}
	return mp.CanMediaTag
}

func getBoolFromPrivacy(p *privacy) bool {
	if p == nil {
		return false
	}
	return p.Protected
}

func getBoolFromVerification(v *verification) bool {
	if v == nil {
		return false
	}
	return v.Verified
}

func getLocation(l *location) string {
	if l == nil {
		return ""
	}
	return l.Location
}

func getAvatarURL(a *avatar) string {
	if a == nil {
		return ""
	}
	return a.ImageURL
}

func cleanMediaHighQuality(ee *extendedEntities) *extendedEntities {
	if ee == nil || len(ee.Media) == 0 {
		return ee
	}
	for i := range ee.Media {
		if ee.Media[i].Type == "photo" {
			ee.Media[i].MediaURLHTTPS = utils.EnsurePhotoHighQuality(ee.Media[i].MediaURLHTTPS)
		}
	}
	return ee
}

func convertUserEntities(ue userEntities) targetUserEntities {
	result := targetUserEntities{
		Description: ue.Description,
	}
	if ue.URL != nil {
		result.URL = ue.URL
	}
	return result
}

func convertTweetEntities(te tweetEntities) targetTweetEntities {
	hashtags := te.Hashtags
	if hashtags == nil {
		hashtags = []hashtag{}
	}
	return targetTweetEntities{
		Hashtags:     hashtags,
		UserMentions: te.UserMentions,
	}
}
