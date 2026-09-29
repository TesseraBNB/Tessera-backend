package data

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

// sharedHTTP is the HTTP client used for all upstream API calls. A single
// client lets connections be pooled and reused across data sources.
var sharedHTTP = &http.Client{Timeout: 30 * time.Second}

const (
	maxRetries  = 3
	baseBackoff = 300 * time.Millisecond
)

// getJSON issues a GET with retry/backoff on transient failures (network
// errors, HTTP 429, and 5xx) and returns the response body. headers may be nil.
func getJSON(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	return doWithRetry(ctx, http.MethodGet, url, nil, headers)
}

// postJSON issues a POST with the given body and the same retry semantics.
func postJSON(ctx context.Context, url string, body []byte, headers map[string]string) ([]byte, error) {
	return doWithRetry(ctx, http.MethodPost, url, body, headers)
}

func doWithRetry(ctx context.Context, method, url string, body []byte, headers map[string]string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if err := sleepBackoff(ctx, attempt); err != nil {
				return nil, err
			}
		}

		var rdr io.Reader
		if body != nil {
			rdr = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, rdr)
		if err != nil {
			return nil, err // construction errors are not retryable
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := sharedHTTP.Do(req)
		if err != nil {
			lastErr = err
			continue // network error — retry
		}
		data, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}

		switch {
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, clip(data, 200))
			continue // transient — retry
		case resp.StatusCode >= 400:
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, clip(data, 300)) // client error — do not retry
		default:
			return data, nil
		}
	}
	return nil, fmt.Errorf("request to %s failed after %d retries: %w", url, maxRetries, lastErr)
}

// sleepBackoff waits an exponentially increasing, jittered interval, aborting
// early if the context is cancelled.
func sleepBackoff(ctx context.Context, attempt int) error {
	d := baseBackoff*time.Duration(1<<(attempt-1)) + time.Duration(rand.Int63n(int64(baseBackoff)))
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func clip(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n])
	}
	return string(b)
}
