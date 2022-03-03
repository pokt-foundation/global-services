package common

import (
	"net/http"
	"time"
)

// RequestWithRetry recursively uses exponential backoff to retry a request a set amount
// of times. Inspired from https://upgear.io/blog/simple-golang-retry-function/
func RequestWithRetry(client http.Client, attempts int, sleep time.Duration,
	req *http.Request) (*http.Response, error) {
	response, err := client.Do(req)

	if err != nil {
		if attempts--; attempts > 0 {
			time.Sleep(sleep)
			return RequestWithRetry(client, attempts, 2*sleep, req)
		}
		return nil, err
	}
	return response, nil
}
