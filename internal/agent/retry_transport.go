package agent

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

type retryingTransport struct {
	next   http.RoundTripper
	delays []time.Duration
}

func newRetryingTransport(next http.RoundTripper, delays []time.Duration) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return &retryingTransport{next: next, delays: append([]time.Duration(nil), delays...)}
}

func (t *retryingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		if err := req.Context().Err(); err != nil {
			return nil, err
		}
		attemptReq, err := requestForAttempt(req, attempt)
		if err != nil {
			return nil, err
		}
		resp, err := t.next.RoundTrip(attemptReq)
		if !shouldRetry(resp, err) || attempt == len(t.delays) {
			return resp, err
		}
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if err := waitForRetry(req.Context(), t.delays[attempt]); err != nil {
			return nil, err
		}
	}
}

func requestForAttempt(req *http.Request, attempt int) (*http.Request, error) {
	if attempt == 0 || req.Body == nil {
		return req, nil
	}
	if req.GetBody == nil {
		return nil, errors.New("cannot retry request with non-replayable body")
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	clone := req.Clone(req.Context())
	clone.Body = body
	return clone, nil
}

func shouldRetry(resp *http.Response, err error) bool {
	if err != nil {
		return isRetriableTransportError(err)
	}
	return resp != nil && (resp.StatusCode == http.StatusBadGateway ||
		resp.StatusCode == http.StatusServiceUnavailable ||
		resp.StatusCode == http.StatusGatewayTimeout)
}

func isRetriableTransportError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
