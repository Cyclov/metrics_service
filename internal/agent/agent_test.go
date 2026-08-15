package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

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
	var received []models.Metrics
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/update", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var metric models.Metrics
		require.NoError(t, json.NewDecoder(r.Body).Decode(&metric))
		mu.Lock()
		received = append(received, metric)
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
