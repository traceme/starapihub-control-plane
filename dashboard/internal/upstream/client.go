package upstream

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"time"
)

// NewHTTPClient creates a shared HTTP client with sensible timeouts.
func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).DialContext,
			MaxIdleConns:        50,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

// doWithRetry executes an HTTP request with 1 retry on 5xx errors after a 1-second delay.
// The request body is buffered so it can be replayed on retry.
func doWithRetry(client *http.Client, req *http.Request) (*http.Response, error) {
	// Buffer the body if present so we can replay it on retry
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// Retry once on 5xx
	if resp.StatusCode >= 500 {
		resp.Body.Close()
		time.Sleep(1 * time.Second)

		// Rebuild the request body for retry
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
		resp, err = client.Do(req)
		if err != nil {
			return nil, err
		}
	}

	return resp, nil
}
