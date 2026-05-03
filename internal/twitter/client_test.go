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

func TestProxyFromEnvironmentUsesHTTPSProxyForHTTPSRequests(t *testing.T) {
	setProxyEnv(t, "http://env-http-proxy.example:8080", "http://env-https-proxy.example:8443")

	proxyURL := selectedProxy(t, http.ProxyFromEnvironment, "https://x.com/home")
	if proxyURL == nil || proxyURL.String() != "http://env-https-proxy.example:8443" {
		t.Fatalf("expected HTTPS_PROXY for HTTPS request, got %v", proxyURL)
	}
}

func TestProxyFromEnvironmentUsesHTTPProxyForHTTPRequests(t *testing.T) {
	setProxyEnv(t, "http://env-http-proxy.example:8080", "http://env-https-proxy.example:8443")

	proxyURL := selectedProxy(t, http.ProxyFromEnvironment, "http://example.com/")
	if proxyURL == nil || proxyURL.String() != "http://env-http-proxy.example:8080" {
		t.Fatalf("expected HTTP_PROXY for HTTP request, got %v", proxyURL)
	}
}


