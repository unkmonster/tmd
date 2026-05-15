package api

// Version is injected by release builds with:
//
//	go build -ldflags "-X github.com/unkmonster/tmd/internal/api.Version=vX.Y.Z"
//
// Keep the default stable for local development and tests.
var Version = "dev"

func buildVersion() string {
	if Version == "" {
		return "dev"
	}
	return Version
}
