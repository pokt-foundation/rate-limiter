package client

import (
	"net/http"
	"net/url"
	"time"

	"github.com/gojektech/heimdall"
	"github.com/gojektech/heimdall/httpclient"
)

const (
	initialBackoffTimeout = 2 * time.Millisecond
	maxBackoffTimeout     = 9 * time.Millisecond
	exponentFactor        = 2
	maxJitterInterval     = 2 * time.Millisecond
)

var (
	backoff = heimdall.NewExponentialBackoff(initialBackoffTimeout, maxBackoffTimeout, exponentFactor, maxJitterInterval)
	retrier = heimdall.NewRetrier(backoff)
)

// Client is a wrapper for the heimdall client
type Client struct {
	*httpclient.Client
}

// NewClient returns httpclient instance with given config
func NewClient(retries int, timeout time.Duration) *Client {
	return &Client{
		Client: httpclient.NewClient(
			httpclient.WithHTTPTimeout(timeout),
			httpclient.WithRetryCount(retries),
			httpclient.WithRetrier(retrier),
		),
	}
}

// GetWithURLAndParams does get request with url values as params
func (c *Client) GetWithURLAndParams(rawURL string, params url.Values, headers http.Header) (*http.Response, error) {
	urlStruct, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	urlStruct.RawQuery = params.Encode()

	return c.Get(urlStruct.String(), headers)
}
