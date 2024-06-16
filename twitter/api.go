package twitter

import (
	"fmt"
	"net/url"
)

const HOST = "https://twitter.com"

type api interface {
	Path() string
	QueryParam() url.Values
}

type timelineApi interface {
	SetCursor(cursor string)
	api
}

func makeUrl(api api) string {
	u, _ := url.Parse(HOST) // 这里绝对不会出错
	u = u.JoinPath(api.Path())
	u.RawQuery = api.QueryParam().Encode()
	return u.String()
}

type userByRestId struct {
	restId uint64
}

func (*userByRestId) Path() string {
	return "/i/api/graphql/CO4_gU4G_MRREoqfiTh6Hg/UserByRestId"
}

func (a *userByRestId) QueryParam() url.Values {
	v := url.Values{}
	variables := `{"userId":"%d","withSafetyModeUserFields":true}`
	v.Set("variables", fmt.Sprintf(variables, a.restId))
	features := `{"hidden_profile_likes_enabled":true,"hidden_profile_subscriptions_enabled":true,"rweb_tipjar_consumption_enabled":true,"responsive_web_graphql_exclude_directive_enabled":true,"verified_phone_label_enabled":false,"highlights_tweets_tab_ui_enabled":true,"responsive_web_twitter_article_notes_tab_enabled":true,"subscriptions_feature_can_gift_premium":false,"creator_subscriptions_tweet_preview_api_enabled":true,"responsive_web_graphql_skip_user_profile_image_extensions_enabled":false,"responsive_web_graphql_timeline_navigation_enabled":true}`
	v.Set("features", features)
	return v
}

type userByScreenName struct {
	screenName string
}

func (*userByScreenName) Path() string {
	return "/i/api/graphql/xmU6X_CKVnQ5lSrCbAmJsg/UserByScreenName"
}

func (a *userByScreenName) QueryParam() url.Values {
	v := url.Values{}

	variables := `{"screen_name":"%s","withSafetyModeUserFields":true}`
	features := `{"hidden_profile_subscriptions_enabled":true,"rweb_tipjar_consumption_enabled":true,"responsive_web_graphql_exclude_directive_enabled":true,"verified_phone_label_enabled":false,"subscriptions_verification_info_is_identity_verified_enabled":true,"subscriptions_verification_info_verified_since_enabled":true,"highlights_tweets_tab_ui_enabled":true,"responsive_web_twitter_article_notes_tab_enabled":true,"subscriptions_feature_can_gift_premium":false,"creator_subscriptions_tweet_preview_api_enabled":true,"responsive_web_graphql_skip_user_profile_image_extensions_enabled":false,"responsive_web_graphql_timeline_navigation_enabled":true}`
	fieldToggles := `{"withAuxiliaryUserLabels":false}`

	v.Set("variables", fmt.Sprintf(variables, a.screenName))
	v.Set("features", features)
	v.Set("fieldToggles", fieldToggles)
	return v
}

type userMedia struct {
	userId uint64
	count  int
	cursor string
}

func (*userMedia) Path() string {
	return "/i/api/graphql/MOLbHrtk8Ovu7DUNOLcXiA/UserMedia"
}

func (a *userMedia) QueryParam() url.Values {
	v := url.Values{}

	variables := `{"userId":"%d","count":%d,"cursor":"%s","includePromotedContent":false,"withClientEventToken":false,"withBirdwatchNotes":false,"withVoice":true,"withV2Timeline":true}`
	features := `{"rweb_tipjar_consumption_enabled":true,"responsive_web_graphql_exclude_directive_enabled":true,"verified_phone_label_enabled":false,"creator_subscriptions_tweet_preview_api_enabled":true,"responsive_web_graphql_timeline_navigation_enabled":true,"responsive_web_graphql_skip_user_profile_image_extensions_enabled":false,"communities_web_enable_tweet_community_results_fetch":true,"c9s_tweet_anatomy_moderator_badge_enabled":true,"articles_preview_enabled":true,"tweetypie_unmention_optimization_enabled":true,"responsive_web_edit_tweet_api_enabled":true,"graphql_is_translatable_rweb_tweet_is_translatable_enabled":true,"view_counts_everywhere_api_enabled":true,"longform_notetweets_consumption_enabled":true,"responsive_web_twitter_article_tweet_consumption_enabled":true,"tweet_awards_web_tipping_enabled":false,"creator_subscriptions_quote_tweet_preview_enabled":false,"freedom_of_speech_not_reach_fetch_enabled":true,"standardized_nudges_misinfo":true,"tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled":true,"rweb_video_timestamps_enabled":true,"longform_notetweets_rich_text_read_enabled":true,"longform_notetweets_inline_media_enabled":true,"responsive_web_enhance_cards_enabled":false}`
	fieldToggles := `{"withArticlePlainText":false}`

	v.Set("variables", fmt.Sprintf(variables, a.userId, a.count, a.cursor))
	v.Set("features", features)
	v.Set("fieldToggles", fieldToggles)
	return v
}

func (a *userMedia) SetCursor(cursor string) {
	a.cursor = cursor
}

type ListByRestId struct {
	id uint64
}

func (*ListByRestId) Path() string {
	return "/i/api/graphql/ZMQOSpxDo0cP5Cdt8MgEVA/ListByRestId"
}

func (a *ListByRestId) QueryParam() url.Values {
	v := url.Values{}

	variables := `{"listId":"%d"}`
	features := `{"rweb_tipjar_consumption_enabled":true,"responsive_web_graphql_exclude_directive_enabled":true,"verified_phone_label_enabled":false,"responsive_web_graphql_skip_user_profile_image_extensions_enabled":false,"responsive_web_graphql_timeline_navigation_enabled":true}`

	v.Set("variables", fmt.Sprintf(variables, a.id))
	v.Set("features", features)
	return v
}

type ListMembers struct {
	id     uint64
	count  int
	cursor string
}

func (*ListMembers) Path() string {
	return "/i/api/graphql/3dQPyRyAj6Lslp4e0ClXzg/ListMembers"
}

func (a *ListMembers) QueryParam() url.Values {
	v := url.Values{}
	variables := `{"listId":"%d","count":%d,"withSafetyModeUserFields":true, "cursor":"%s"}`
	features := `{"rweb_tipjar_consumption_enabled":true,"responsive_web_graphql_exclude_directive_enabled":true,"verified_phone_label_enabled":false,"creator_subscriptions_tweet_preview_api_enabled":true,"responsive_web_graphql_timeline_navigation_enabled":true,"responsive_web_graphql_skip_user_profile_image_extensions_enabled":false,"communities_web_enable_tweet_community_results_fetch":true,"c9s_tweet_anatomy_moderator_badge_enabled":true,"articles_preview_enabled":true,"tweetypie_unmention_optimization_enabled":true,"responsive_web_edit_tweet_api_enabled":true,"graphql_is_translatable_rweb_tweet_is_translatable_enabled":true,"view_counts_everywhere_api_enabled":true,"longform_notetweets_consumption_enabled":true,"responsive_web_twitter_article_tweet_consumption_enabled":true,"tweet_awards_web_tipping_enabled":false,"creator_subscriptions_quote_tweet_preview_enabled":false,"freedom_of_speech_not_reach_fetch_enabled":true,"standardized_nudges_misinfo":true,"tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled":true,"rweb_video_timestamps_enabled":true,"longform_notetweets_rich_text_read_enabled":true,"longform_notetweets_inline_media_enabled":true,"responsive_web_enhance_cards_enabled":false}`

	v.Set("variables", fmt.Sprintf(variables, a.id, a.count, a.cursor))
	v.Set("features", features)
	return v
}

func (a *ListMembers) SetCursor(cursor string) {
	a.cursor = cursor
}
