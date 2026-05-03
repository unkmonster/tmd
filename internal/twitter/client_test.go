package twitter

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func setProxyEnv(t *testing.T, httpProxy, httpsProxy string) {
	t.Helper()
	t.Setenv("http_proxy", "")
	t.Setenv("https_proxy", "")
	t.Setenv("HTTP_PROXY", httpProxy)
	t.Setenv("HTTPS_PROXY", httpsProxy)
}

func selectedProxy(t *testing.T, proxyFunc func(req *http.Request) (*url.URL, error), rawURL string) *url.URL {
	t.Helper()
	req := httptest.NewRequest("GET", rawURL, nil)
	proxyURL, err := proxyFunc(req)
	if err != nil {
		t.Fatalf("proxy function returned error: %v", err)
	}
	return proxyURL
}

func TestNewProxyFuncConfigWinsOverEnvironment(t *testing.T) {
	setProxyEnv(t, "http://env-proxy.example:8080", "http://env-https-proxy.example:8443")

	proxyURL := selectedProxy(t, newProxyFunc("http://user:pass@configured-proxy.example:9000"), "https://x.com/home")
	if proxyURL == nil {
		t.Fatal("expected configured proxy")
	}
	if proxyURL.Host != "configured-proxy.example:9000" {
		t.Fatalf("expected configured proxy host, got %q", proxyURL.Host)
	}
	if proxyURL.Redacted() != "http://user:xxxxx@configured-proxy.example:9000" {
		t.Fatalf("expected redacted proxy URL, got %q", proxyURL.Redacted())
	}
}

func TestNewProxyFuncHTTPSFallsBackToHTTPProxy(t *testing.T) {
	setProxyEnv(t, "http://env-proxy.example:8080", "")

	proxyURL := selectedProxy(t, newProxyFunc(""), "https://x.com/home")
	if proxyURL == nil {
		t.Fatal("expected HTTPS request to fall back to HTTP_PROXY")
	}
	if proxyURL.String() != "http://env-proxy.example:8080" {
		t.Fatalf("unexpected proxy: %s", proxyURL)
	}
}

func TestNewProxyFuncUsesSchemeSpecificEnvironmentProxyFirst(t *testing.T) {
	setProxyEnv(t, "http://env-http-proxy.example:8080", "http://env-https-proxy.example:8443")

	httpsProxy := selectedProxy(t, newProxyFunc(""), "https://x.com/home")
	if httpsProxy == nil || httpsProxy.String() != "http://env-https-proxy.example:8443" {
		t.Fatalf("expected HTTPS_PROXY, got %v", httpsProxy)
	}

	httpProxy := selectedProxy(t, newProxyFunc(""), "http://example.com/")
	if httpProxy == nil || httpProxy.String() != "http://env-http-proxy.example:8080" {
		t.Fatalf("expected HTTP_PROXY, got %v", httpProxy)
	}
}

func TestNewProxyFuncHTTPFallsBackToHTTPSProxy(t *testing.T) {
	setProxyEnv(t, "", "http://env-https-proxy.example:8443")

	proxyURL := selectedProxy(t, newProxyFunc(""), "http://example.com/")
	if proxyURL == nil {
		t.Fatal("expected HTTP request to fall back to HTTPS_PROXY")
	}
	if proxyURL.String() != "http://env-https-proxy.example:8443" {
		t.Fatalf("unexpected proxy: %s", proxyURL)
	}
}

func TestParseConfiguredProxyURLRequiresSchemeAndHost(t *testing.T) {
	if _, err := parseConfiguredProxyURL("http://127.0.0.1:7890"); err != nil {
		t.Fatalf("expected valid proxy URL: %v", err)
	}
	if _, err := parseConfiguredProxyURL("127.0.0.1:7890"); err == nil {
		t.Fatal("expected missing scheme/host to fail")
	}
}

func TestParseEnvProxyURLAcceptsHostPort(t *testing.T) {
	proxyURL, err := parseEnvProxyURL("127.0.0.1:7890")
	if err != nil {
		t.Fatalf("expected host:port env proxy to parse: %v", err)
	}
	if proxyURL.String() != "http://127.0.0.1:7890" {
		t.Fatalf("unexpected proxy URL: %s", proxyURL)
	}
}
