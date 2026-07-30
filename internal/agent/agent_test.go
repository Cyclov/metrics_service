package agent

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestCollectorPoll(t *testing.T) {
	collector := NewCollector()
	collector.Poll()
	first := collector.CurrentMetrics()
	collector.Poll()
	second := collector.CurrentMetrics()

	if len(first.Gauges) != 28 {
		t.Fatalf("got %d gauges, want 28", len(first.Gauges))
	}
	if first.PollCount != 1 || second.PollCount != 2 {
		t.Fatalf("unexpected PollCount values: %d, %d", first.PollCount, second.PollCount)
	}
	if _, ok := first.Gauges["Alloc"]; !ok {
		t.Error("Alloc metric is missing")
	}
	if _, ok := first.Gauges["RandomValue"]; !ok {
		t.Error("RandomValue metric is missing")
	}
}

func TestSenderSend(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "text/plain" {
			t.Errorf("Content-Type = %q, want text/plain", got)
		}
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewSender(server.URL, server.Client())
	err := sender.Send(Metrics{
		Gauges:    map[string]float64{"Alloc": 12.5},
		PollCount: 3,
	})
	if err != nil {
		t.Fatal(err)
	}

	joined := strings.Join(paths, "\n")
	if !strings.Contains(joined, "/update/gauge/Alloc/12.5") {
		t.Errorf("gauge request not found in %q", joined)
	}
	if !strings.Contains(joined, "/update/counter/PollCount/3") {
		t.Errorf("counter request not found in %q", joined)
	}
}
