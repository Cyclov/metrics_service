package agent

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

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

	assert.Len(t, first.Gauges, 28)
	assert.Equal(t, int64(1), first.PollCount)
	assert.Equal(t, int64(2), second.PollCount)
	assert.Contains(t, first.Gauges, "Alloc")
	assert.Contains(t, first.Gauges, "RandomValue")
}

func TestSenderSend(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "text/plain", r.Header.Get("Content-Type"))
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := resty.New().
		SetTransport(server.Client().Transport)
	sender := NewSender(server.URL, client)
	err := sender.Send(Metrics{
		Gauges:    map[string]float64{"Alloc": 12.5},
		PollCount: 3,
	})
	require.NoError(t, err)

	joined := strings.Join(paths, "\n")
	assert.Contains(t, joined, "/update/gauge/Alloc/12.5")
	assert.Contains(t, joined, "/update/counter/PollCount/3")
}
