package qweather

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Nativu5/qweather-cli/internal/auth"
)

const DefaultMaxBodyBytes int64 = 16 << 20

type Request struct {
	CapabilityID string
	Path         string
	Query        url.Values
}

type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

type Doer interface {
	Do(context.Context, Request) (Response, error)
}

type ErrorKind string

const (
	ErrorNetwork  ErrorKind = "network"
	ErrorProtocol ErrorKind = "protocol"
	ErrorOversize ErrorKind = "oversize"
)

type ClientError struct {
	Kind ErrorKind
	Err  error
}

func (e *ClientError) Error() string {
	return e.Err.Error()
}

func (e *ClientError) Unwrap() error {
	return e.Err
}

type ClientOptions struct {
	Origin       *url.URL
	HTTPClient   *http.Client
	Now          func() time.Time
	MaxBodyBytes int64
}

type Client struct {
	origin       url.URL
	httpClient   *http.Client
	credentials  auth.Credentials
	now          func() time.Time
	maxBodyBytes int64
}

func NewClient(apiHost string, credentials auth.Credentials, options ClientOptions) (*Client, error) {
	origin := options.Origin
	if origin == nil {
		origin = &url.URL{Scheme: "https", Host: apiHost}
	}
	if origin.Scheme != "https" || origin.Host == "" || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return nil, errors.New("QWeather origin must be an HTTPS origin without a path or query")
	}
	client := options.HTTPClient
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = http.ProxyFromEnvironment
		client = &http.Client{Transport: transport}
	}
	clientCopy := *client
	clientCopy.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) == 0 {
			return nil
		}
		if !strings.EqualFold(request.URL.Host, via[0].URL.Host) {
			return errors.New("cross-host redirect rejected")
		}
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		return nil
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	maxBodyBytes := options.MaxBodyBytes
	if maxBodyBytes == 0 {
		maxBodyBytes = DefaultMaxBodyBytes
	}
	if maxBodyBytes < 0 {
		return nil, errors.New("maximum response size must be positive")
	}
	return &Client{
		origin:       *origin,
		httpClient:   &clientCopy,
		credentials:  credentials,
		now:          now,
		maxBodyBytes: maxBodyBytes,
	}, nil
}

func (c *Client) Do(ctx context.Context, request Request) (Response, error) {
	if request.Path == "" || request.Path[0] != '/' || strings.Contains(request.Path, "..") {
		return Response{}, &ClientError{Kind: ErrorProtocol, Err: errors.New("provider request path is invalid")}
	}
	target := c.origin
	target.Path = request.Path
	target.RawQuery = request.Query.Encode()
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return Response{}, &ClientError{Kind: ErrorProtocol, Err: fmt.Errorf("create provider request: %w", err)}
	}
	header, err := c.credentials.Header(c.now())
	if err != nil {
		return Response{}, &ClientError{Kind: ErrorProtocol, Err: fmt.Errorf("create authorization header: %w", err)}
	}
	httpRequest.Header.Set(header.Name, header.Value)
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("Accept-Encoding", "gzip")

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return Response{}, &ClientError{Kind: ErrorNetwork, Err: fmt.Errorf("perform provider request: %w", err)}
	}
	defer httpResponse.Body.Close()
	reader := io.Reader(httpResponse.Body)
	if encoding := strings.TrimSpace(httpResponse.Header.Get("Content-Encoding")); encoding != "" {
		if !strings.EqualFold(encoding, "gzip") {
			return Response{}, &ClientError{Kind: ErrorProtocol, Err: fmt.Errorf("unsupported content encoding %q", encoding)}
		}
		compressed, gzipErr := gzip.NewReader(httpResponse.Body)
		if gzipErr != nil {
			return Response{}, &ClientError{Kind: ErrorProtocol, Err: fmt.Errorf("open gzip response: %w", gzipErr)}
		}
		defer compressed.Close()
		reader = compressed
	}
	body, err := readBounded(reader, c.maxBodyBytes)
	if err != nil {
		return Response{}, err
	}
	return Response{StatusCode: httpResponse.StatusCode, Header: httpResponse.Header.Clone(), Body: body}, nil
}

func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	contents, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, &ClientError{Kind: ErrorNetwork, Err: fmt.Errorf("read provider response: %w", err)}
	}
	if int64(len(contents)) > limit {
		return nil, &ClientError{Kind: ErrorOversize, Err: fmt.Errorf("provider response exceeds %d bytes", limit)}
	}
	return contents, nil
}
