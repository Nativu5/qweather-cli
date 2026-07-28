package qweather

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Nativu5/qweather-cli/internal/auth"
)

func apiKeyCredentials(t *testing.T) auth.Credentials {
	t.Helper()
	credentials, err := auth.NewAPIKey("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	return credentials
}

func parsedURL(t *testing.T, value string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestClientBuildsOneSafeGETAndDecompressesGzip(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.Method != http.MethodGet || request.URL.Path != "/v7/weather/now" {
			t.Errorf("request = %s %s", request.Method, request.URL.Path)
		}
		if got := request.URL.Query().Get("location"); got != "北京 & city" {
			t.Errorf("location = %q", got)
		}
		if got := request.Header.Get("X-QW-Api-Key"); got != "test-secret" {
			t.Errorf("API key header = %q", got)
		}
		if got := request.Header.Get("Accept-Encoding"); got != "gzip" {
			t.Errorf("Accept-Encoding = %q", got)
		}
		writer.Header().Set("Content-Encoding", "gzip")
		compressed := gzip.NewWriter(writer)
		_, _ = io.WriteString(compressed, `{"code":"200","now":{"temp":"20"}}`)
		_ = compressed.Close()
	}))
	defer server.Close()
	client, err := NewClient("", apiKeyCredentials(t), ClientOptions{
		Origin:     parsedURL(t, server.URL),
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(context.Background(), Request{
		CapabilityID: "weather.city.current",
		Path:         "/v7/weather/now",
		Query:        url.Values{"location": {"北京 & city"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || string(response.Body) != `{"code":"200","now":{"temp":"20"}}` {
		t.Fatalf("response = %#v", response)
	}
	if requests.Load() != 1 {
		t.Fatalf("request count = %d", requests.Load())
	}
}

func TestClientRejectsOversizedDecompressedBody(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, "123456")
	}))
	defer server.Close()
	client, err := NewClient("", apiKeyCredentials(t), ClientOptions{
		Origin:       parsedURL(t, server.URL),
		HTTPClient:   server.Client(),
		MaxBodyBytes: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Do(context.Background(), Request{Path: "/test"})
	var clientError *ClientError
	if !errors.As(err, &clientError) || clientError.Kind != ErrorOversize {
		t.Fatalf("Do() error = %v", err)
	}
}

func TestClientRejectsCrossHostRedirectBeforeTargetRequest(t *testing.T) {
	var targetRequests atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		targetRequests.Add(1)
	}))
	defer target.Close()
	redirect := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL+"/stolen", http.StatusFound)
	}))
	defer redirect.Close()
	client, err := NewClient("", apiKeyCredentials(t), ClientOptions{
		Origin:     parsedURL(t, redirect.URL),
		HTTPClient: redirect.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Do(context.Background(), Request{Path: "/redirect"})
	if err == nil || !strings.Contains(err.Error(), "cross-host redirect rejected") {
		t.Fatalf("Do() error = %v", err)
	}
	var clientError *ClientError
	if !errors.As(err, &clientError) || clientError.Kind != ErrorProtocol {
		t.Fatalf("Do() ClientError = %#v", clientError)
	}
	if targetRequests.Load() != 0 {
		t.Fatalf("redirect target received %d request(s)", targetRequests.Load())
	}
}

func TestClientRejectsHTTPSDowngradeRedirectBeforeTargetRequest(t *testing.T) {
	var targetRequests atomic.Int32
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Scheme == "http" {
			targetRequests.Add(1)
		}
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": {"http://example.com/downgraded"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    request,
		}, nil
	})
	client, err := NewClient("", apiKeyCredentials(t), ClientOptions{
		Origin:     parsedURL(t, "https://example.com"),
		HTTPClient: &http.Client{Transport: transport},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Do(context.Background(), Request{Path: "/redirect"})
	if err == nil || !strings.Contains(err.Error(), "non-HTTPS redirect rejected") {
		t.Fatalf("Do() error = %v", err)
	}
	var clientError *ClientError
	if !errors.As(err, &clientError) || clientError.Kind != ErrorProtocol {
		t.Fatalf("Do() ClientError = %#v", clientError)
	}
	problem := ProblemForError(err, "weather.city.current")
	if problem.ExitCode != 9 || problem.Code != "UPSTREAM_PROTOCOL_ERROR" || problem.Retryable {
		t.Fatalf("ProblemForError() = %#v", problem)
	}
	if targetRequests.Load() != 0 {
		t.Fatalf("downgraded target received %d request(s)", targetRequests.Load())
	}
}

func TestClientAllowsSameHostHTTPSRedirect(t *testing.T) {
	var targetRequests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/redirect" {
			http.Redirect(writer, request, "/target", http.StatusFound)
			return
		}
		if request.URL.Path != "/target" {
			t.Errorf("unexpected request path %q", request.URL.Path)
			return
		}
		targetRequests.Add(1)
		if got := request.Header.Get("X-QW-Api-Key"); got != "test-secret" {
			t.Errorf("API key header = %q", got)
		}
		_, _ = io.WriteString(writer, `{\"code\":\"200\",\"now\":{\"temp\":\"20\"}}`)
	}))
	defer server.Close()
	client, err := NewClient("", apiKeyCredentials(t), ClientOptions{
		Origin:     parsedURL(t, server.URL),
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(context.Background(), Request{Path: "/redirect"})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || targetRequests.Load() != 1 {
		t.Fatalf("response status = %d, target requests = %d", response.StatusCode, targetRequests.Load())
	}
}

func TestClientRequiresHTTPSOrigin(t *testing.T) {
	_, err := NewClient("", apiKeyCredentials(t), ClientOptions{Origin: parsedURL(t, "http://example.com")})
	if err == nil || !strings.Contains(err.Error(), "HTTPS origin") {
		t.Fatalf("NewClient() error = %v", err)
	}
}

func TestDefaultClientUsesStandardProxyResolver(t *testing.T) {
	client, err := NewClient("example.qweatherapi.com", apiKeyCredentials(t), ClientOptions{})
	if err != nil {
		t.Fatal(err)
	}
	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok || transport.Proxy == nil {
		t.Fatal("default HTTP transport does not use the standard proxy resolver")
	}
}
