package agent

import (
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	models "github.com/Cyclov/metrics_service/internal/model"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectorPoll(t *testing.T) {
	collector := NewCollector()
	collector.Poll()
	first := collector.CurrentMetrics()
	collector.Poll()
	second := collector.CurrentMetrics()

	assert.Len(t, first, 29)

	firstByID := metricsByID(first)
	secondByID := metricsByID(second)
	require.NotNil(t, firstByID["PollCount"].Delta)
	require.NotNil(t, secondByID["PollCount"].Delta)
	assert.Equal(t, int64(1), *firstByID["PollCount"].Delta)
	assert.Equal(t, int64(2), *secondByID["PollCount"].Delta)
	assert.Equal(t, models.Gauge, firstByID["Alloc"].MType)
	assert.Equal(t, models.Gauge, firstByID["RandomValue"].MType)
}

func TestCollectorSystemMetrics(t *testing.T) {
	collector := NewCollector()
	collector.setSystemMetrics(100, 40, []float64{12.5, 75})
	collector.Poll()
	metrics := metricsByID(collector.CurrentMetrics())
	for name, want := range map[string]float64{
		"TotalMemory":     100,
		"FreeMemory":      40,
		"CPUutilization1": 12.5,
		"CPUutilization2": 75,
	} {
		metric, ok := metrics[name]
		require.True(t, ok, "CurrentMetrics() missing %s", name)
		require.NotNil(t, metric.Value)
		assert.Equal(t, models.Gauge, metric.MType)
		assert.Equal(t, want, *metric.Value)
	}
	collector.setSystemMetrics(100, 30, []float64{25})
	assert.NotContains(t, metricsByID(collector.CurrentMetrics()), "CPUutilization2")
}

func metricsByID(metrics []models.Metrics) map[string]models.Metrics {
	result := make(map[string]models.Metrics, len(metrics))
	for _, metric := range metrics {
		result[metric.ID] = metric
	}
	return result
}

func TestSenderSend(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	var received []models.Metrics
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/updates/", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))
		assert.Contains(t, r.Header.Get("Accept-Encoding"), "gzip")
		zr, err := gzip.NewReader(r.Body)
		require.NoError(t, err)
		defer zr.Close()
		var metrics []models.Metrics
		require.NoError(t, json.NewDecoder(zr).Decode(&metrics))
		mu.Lock()
		requestCount++
		received = append(received, metrics...)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := resty.New().
		SetTransport(server.Client().Transport)
	sender := NewSender(server.URL, client, "")
	value := 12.5
	delta := int64(3)
	err := sender.Send([]models.Metrics{
		{ID: "Alloc", MType: models.Gauge, Value: &value},
		{ID: "PollCount", MType: models.Counter, Delta: &delta},
	})
	require.NoError(t, err)

	assert.Equal(t, 1, requestCount)
	require.Len(t, received, 2)
	assert.Equal(t, "Alloc", received[0].ID)
	assert.Equal(t, models.Gauge, received[0].MType)
	require.NotNil(t, received[0].Value)
	assert.Equal(t, 12.5, *received[0].Value)
	assert.Equal(t, "PollCount", received[1].ID)
	assert.Equal(t, models.Counter, received[1].MType)
	require.NotNil(t, received[1].Delta)
	assert.Equal(t, int64(3), *received[1].Delta)
}

func TestSenderDoesNotSendEmptyBatch(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewSender(server.URL, resty.New().SetTransport(server.Client().Transport), "")
	require.NoError(t, sender.Send(nil))
	assert.Zero(t, requestCount)
}

func TestSenderRetriesTemporaryStatuses(t *testing.T) {
	delays := sendRetryDelays
	sendRetryDelays = []time.Duration{0, 0, 0}
	t.Cleanup(func() { sendRetryDelays = delays })

	statuses := []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout, http.StatusOK}
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(statuses[requestCount])
		requestCount++
	}))
	defer server.Close()

	value := 1.0
	sender := NewSender(server.URL, resty.New().SetTransport(server.Client().Transport), "")
	require.NoError(t, sender.Send([]models.Metrics{{ID: "Alloc", MType: models.Gauge, Value: &value}}))
	assert.Equal(t, 4, requestCount)
}

func TestSenderRetriesTransportError(t *testing.T) {
	delays := sendRetryDelays
	sendRetryDelays = []time.Duration{0, 0, 0}
	t.Cleanup(func() { sendRetryDelays = delays })

	requestCount := 0
	client := resty.New().SetTransport(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		requestCount++
		return nil, &net.DNSError{Err: "temporary DNS failure", IsTemporary: true}
	}))
	value := 1.0
	err := NewSender("http://metrics", client, "").Send([]models.Metrics{{ID: "Alloc", MType: models.Gauge, Value: &value}})
	require.Error(t, err)
	assert.Equal(t, 4, requestCount)
}

func TestSenderStopsWhenContextIsCanceled(t *testing.T) {
	client := resty.New().SetTransport(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, &net.DNSError{Err: "temporary DNS failure", IsTemporary: true}
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	value := 1.0
	err := NewSender("http://metrics", client, "").SendContext(ctx, []models.Metrics{{ID: "Alloc", MType: models.Gauge, Value: &value}})
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestRunStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Run(ctx, NewCollector(), NewSender("http://metrics", resty.New(), ""), time.Hour, time.Hour, 1)
	require.NoError(t, err)
}

func TestRunBoundsRequestsWithoutBlockingPoll(t *testing.T) {
	var inFlight atomic.Int32
	started := make(chan int, 3)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		zr, err := gzip.NewReader(r.Body)
		if err != nil {
			started <- 0
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer zr.Close()
		var batch []models.Metrics
		if err := json.NewDecoder(zr).Decode(&batch); err != nil {
			started <- 0
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		inFlight.Add(1)
		defer inFlight.Add(-1)
		started <- len(batch)
		select {
		case <-release:
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	collector := NewCollector()
	sender := NewSender(server.URL, resty.New().SetTransport(server.Client().Transport), "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, collector, sender, 10*time.Millisecond, 10*time.Millisecond, 2) }()

	for range 2 {
		select {
		case count := <-started:
			assert.Greater(t, count, 1, "Run() should send a batch")
		case <-time.After(2 * time.Second):
			t.Fatal("Run() did not start two concurrent requests")
		}
	}
	before := *metricsByID(collector.CurrentMetrics())["PollCount"].Delta
	require.Eventually(t, func() bool {
		return *metricsByID(collector.CurrentMetrics())["PollCount"].Delta > before
	}, time.Second, 10*time.Millisecond, "runtime polling stopped while requests were blocked")
	assert.Equal(t, int32(2), inFlight.Load())
	select {
	case <-started:
		t.Error("Run() exceeded rate limit of 2")
	default:
	}

	cancel()
	close(release)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not stop after cancellation")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func TestSenderHashSHA256(t *testing.T) {
	delays := sendRetryDelays
	sendRetryDelays = []time.Duration{0}
	t.Cleanup(func() { sendRetryDelays = delays })
	for _, key := range []string{"", "test-key", "another-test-key"} {
		t.Run("key="+key, func(t *testing.T) {
			requests := 0
			var firstBody []byte
			client := resty.New().SetTransport(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				require.NoError(t, req.Body.Close())
				if requests == 0 {
					firstBody = body
				} else {
					assert.Equal(t, firstBody, body)
				}
				if key == "" {
					assert.NotContains(t, req.Header, http.CanonicalHeaderKey("HashSHA256"))
				} else {
					mac := hmac.New(sha256.New, []byte(key))
					_, err = mac.Write(body)
					require.NoError(t, err)
					assert.Equal(t, hex.EncodeToString(mac.Sum(nil)), req.Header.Get("HashSHA256"))
				}
				requests++
				status := http.StatusOK
				if requests == 1 {
					status = http.StatusServiceUnavailable
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: http.NoBody, Request: req}, nil
			}))
			sender := NewSender("http://metrics", client, key)
			value := 12.5
			require.NoError(t, sender.Send([]models.Metrics{{ID: "Alloc", MType: models.Gauge, Value: &value}}))
			assert.Equal(t, 2, requests)
		})
	}
}

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }
