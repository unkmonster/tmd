package profile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/unkmonster/tmd/internal/twitter"
)

type TwitterFetcher struct {
	clients []*resty.Client
	mu      sync.Mutex
}

var reNormalAvatarURL = regexp.MustCompile(`_normal(\.[a-zA-Z]+)$`)

func NewTwitterFetcherWithClients(clients []*resty.Client) *TwitterFetcher {
	if len(clients) == 0 {
		panic("clients cannot be empty")
	}
	return &TwitterFetcher{clients: clients}
}

func (tf *TwitterFetcher) FetchProfile(ctx context.Context, screenName string) (*ProfileInfo, error) {
	tf.mu.Lock()
	client := twitter.SelectClient(ctx, tf.clients, "/i/api/graphql/xmU6X_CKVnQ5lSrCbAmJsg/UserByScreenName")
	tf.mu.Unlock()

	if client == nil {
		return nil, &ProfileError{Op: "fetch", User: screenName, Err: fmt.Errorf("no available client")}
	}

	profile, err := tf.fetchFullProfile(ctx, client, screenName)
	if err != nil {
		tf.handleClientError(client, err)
		return nil, &ProfileError{Op: "fetch", User: screenName, Err: err}
	}

	return profile, nil
}

func (tf *TwitterFetcher) fetchFullProfile(ctx context.Context, client *resty.Client, screenName string) (*ProfileInfo, error) {
	usr, _, err := twitter.GetUserByScreenName(ctx, client, screenName)
	if err != nil {
		return nil, err
	}
	return userToProfileInfo(usr), nil
}

func userToProfileInfo(u *twitter.User) *ProfileInfo {
	return &ProfileInfo{
		ID:          u.Id,
		Name:        u.Name,
		ScreenName:  u.ScreenName,
		AvatarURL:   u.AvatarURL,
		BannerURL:   u.BannerURL,
		Description: u.Description,
		Location:    u.Location,
		URL:         u.URL,
		Verified:    u.Verified,
		Protected:   u.IsProtected,
		CreatedAt:   u.CreatedAt,
	}
}

func (tf *TwitterFetcher) handleClientError(client *resty.Client, err error) {
	var apiErr *twitter.TwitterApiError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case twitter.ErrExceedPostLimit, twitter.ErrAccountLocked:
			twitter.SetClientError(client, apiErr)
		}
	}
}

func (tf *TwitterFetcher) Client() *resty.Client {
	tf.mu.Lock()
	defer tf.mu.Unlock()

	for _, client := range tf.clients {
		if twitter.GetClientError(client) == nil {
			return client
		}
	}

	if len(tf.clients) > 0 {
		return tf.clients[0]
	}
	return nil
}

func GetHighResAvatarURL(url string, quality string) string {
	if url == "" {
		return ""
	}
	return reNormalAvatarURL.ReplaceAllString(url, "_"+quality+"$1")
}

func ProfileToJSON(profile *ProfileInfo) ([]byte, error) {
	return json.MarshalIndent(profile, "", "  ")
}
