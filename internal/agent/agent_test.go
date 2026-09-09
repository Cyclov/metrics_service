package agent

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
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
	sender := NewSender(server.URL, client)
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

	sender := NewSender(server.URL, resty.New().SetTransport(server.Client().Transport))
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
	sender := NewSender(server.URL, resty.New().SetTransport(server.Client().Transport))
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
	err := NewSender("http://metrics", client).Send([]models.Metrics{{ID: "Alloc", MType: models.Gauge, Value: &value}})
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
	err := NewSender("http://metrics", client).SendContext(ctx, []models.Metrics{{ID: "Alloc", MType: models.Gauge, Value: &value}})
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestRunStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Run(ctx, NewCollector(), NewSender("http://metrics", resty.New()), time.Hour, time.Hour)
	require.NoError(t, err)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }
