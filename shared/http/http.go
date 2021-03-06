package clienthttp

import (
	"time"

	"github.com/Pocket/global-services/shared/environment"
	"github.com/gojektech/heimdall"
	"github.com/gojektech/heimdall/httpclient"
)

var (
	httpClientTimeout           = time.Duration(environment.GetInt64("HTTP_CLIENT_TIMEOUT", 8)) * time.Second
	httpClientRetries           = int(environment.GetInt64("HTTP_CLIENT_RETRIES", 0))
	httpClientBackoffMultiplier = int(environment.GetInt64("HTTP_CLIENT_BACKOFF_MULTIPLIER", 2))
)

// Client is the struct of the HTTP client
type Client struct {
	*httpclient.Client
}

// retries returns duration for linear backoff client interface
func retrier(retry int) time.Duration {
	if retry <= 0 {
		return 0 * time.Millisecond
	}

	return time.Duration(httpClientBackoffMultiplier*retry) * time.Millisecond
}

// NewClient returns httpclient instance with default config
func NewClient() *Client {
	return &Client{
		Client: httpclient.NewClient(
			httpclient.WithHTTPTimeout(httpClientTimeout),
			httpclient.WithRetryCount(httpClientRetries),
			httpclient.WithRetrier(heimdall.NewRetrierFunc(retrier)),
		),
	}
}
