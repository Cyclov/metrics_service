package agent

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryingTransportRetriesTemporaryStatusesAndReplaysBody(t *testing.T) {
	statuses := []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout, http.StatusOK}
	var bodies []string
	next := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		bodies = append(bodies, string(body))
		status := statuses[len(bodies)-1]
		return &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Request:    req,
		}, nil
	})
	req, err := http.NewRequest(http.MethodPost, "http://metrics/updates/", bytes.NewBufferString("metrics"))
	require.NoError(t, err)

	resp, err := newRetryingTransport(next, []time.Duration{0, 0, 0}).RoundTrip(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, []string{"metrics", "metrics", "metrics", "metrics"}, bodies)
}

func TestRetryingTransportRetriesTransportErrors(t *testing.T) {
	attempts := 0
	next := roundTripperFunc(func(*http.Request) (*http.Response, error) {
		attempts++
		return nil, &net.DNSError{Err: "temporary DNS failure", IsTemporary: true}
	})
	req, err := http.NewRequest(http.MethodGet, "http://metrics/", nil)
	require.NoError(t, err)

	_, err = newRetryingTransport(next, []time.Duration{0, 0, 0}).RoundTrip(req)
	require.Error(t, err)
	assert.Equal(t, 4, attempts)
}

func TestRetryingTransportStopsWhenContextIsCanceled(t *testing.T) {
	next := roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, &net.DNSError{Err: "temporary DNS failure", IsTemporary: true}
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://metrics/", nil)
	require.NoError(t, err)

	_, err = newRetryingTransport(next, []time.Duration{0}).RoundTrip(req)
	assert.True(t, errors.Is(err, context.Canceled))
}
