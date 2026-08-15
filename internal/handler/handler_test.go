package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cyclov/metrics_service/internal/agent"
	models "github.com/Cyclov/metrics_service/internal/model"
	"github.com/Cyclov/metrics_service/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type storageMock struct {
	gauges   map[string]float64
	counters map[string]int64
}

func (s *storageMock) AddGauge(name string, value float64) {
	s.gauges[name] = value
}

func (s *storageMock) AddCounter(name string, value int64) {
	s.counters[name] += value
}

func (s *storageMock) Gauge(name string) (float64, bool) {
	value, ok := s.gauges[name]
	return value, ok
}

func (s *storageMock) Counter(name string) (int64, bool) {
	value, ok := s.counters[name]
	return value, ok
}

func (s *storageMock) AllMetrics() (map[string]float64, map[string]int64) {
	return s.gauges, s.counters
}

func TestUpdatePath(t *testing.T) {
	tests := []struct {
		name       string
		metricType string
		metricName string
		value      string
		wantStatus int
	}{
		{"gauge", "gauge", "Alloc", "1.5", http.StatusOK},
		{"counter", "counter", "PollCount", "2", http.StatusOK},
		{"bad type", "unknown", "x", "1", http.StatusBadRequest},
		{"bad value", "gauge", "x", "nope", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &storageMock{
				gauges:   make(map[string]float64),
				counters: make(map[string]int64),
			}
			h := New(storage)

			req := httptest.NewRequest(http.MethodPost, "/update", nil)
			req.SetPathValue("type", tt.metricType)
			req.SetPathValue("name", tt.metricName)
			req.SetPathValue("value", tt.value)
			rec := httptest.NewRecorder()

			h.UpdatePath(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestValuePath(t *testing.T) {
	storage := &storageMock{
		gauges: map[string]float64{
			"Alloc": 12.5,
		},
		counters: map[string]int64{
			"PollCount": 3,
		},
	}
	h := New(storage)

	tests := []struct {
		name       string
		metricType string
		metricName string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "gauge",
			metricType: "gauge",
			metricName: "Alloc",
			wantStatus: http.StatusOK,
			wantBody:   "12.5",
		},
		{
			name:       "counter",
			metricType: "counter",
			metricName: "PollCount",
			wantStatus: http.StatusOK,
			wantBody:   "3",
		},
		{
			name:       "metric not found",
			metricType: "gauge",
			metricName: "Unknown",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "wrong metric type",
			metricType: "unknown",
			metricName: "Alloc",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/value", nil)
			req.SetPathValue("type", tt.metricType)
			req.SetPathValue("name", tt.metricName)
			rec := httptest.NewRecorder()

			h.ValuePath(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantBody != "" {
				assert.Equal(t, tt.wantBody, rec.Body.String())
			}
			if tt.wantStatus == http.StatusOK {
				assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
			}
		})
	}
}

func TestAllMetrics(t *testing.T) {
	storage := &storageMock{
		gauges: map[string]float64{
			"Alloc":       12.5,
			"RandomValue": 0.25,
		},
		counters: map[string]int64{
			"PollCount": 3,
		},
	}
	h := New(storage)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.AllMetrics(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))

	body := rec.Body.String()
	wantFragments := []string{
		"<h2>Gauge</h2>",
		"<p>Alloc: 12.5</p>",
		"<p>RandomValue: 0.25</p>",
		"<h2>Counters</h2>",
		"<p>PollCount: 3</p>",
	}
	for _, fragment := range wantFragments {
		assert.Contains(t, body, fragment)
	}
}

func TestJSONEndpoints(t *testing.T) {
	storage := &storageMock{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
	h := New(storage)

	value := 1744184459.0
	updateBody, err := json.Marshal(models.Metrics{ID: "LastGC", MType: models.Gauge, Value: &value})
	require.NoError(t, err)
	updateReq := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	h.Update(updateRec, updateReq)

	require.Equal(t, http.StatusOK, updateRec.Code)
	assert.Equal(t, "application/json", updateRec.Header().Get("Content-Type"))
	assert.Equal(t, value, storage.gauges["LastGC"])

	valueBody, err := json.Marshal(models.Metrics{ID: "LastGC", MType: models.Gauge})
	require.NoError(t, err)
	valueReq := httptest.NewRequest(http.MethodPost, "/value", bytes.NewReader(valueBody))
	valueReq.Header.Set("Content-Type", "application/json")
	valueRec := httptest.NewRecorder()
	h.Value(valueRec, valueReq)

	require.Equal(t, http.StatusOK, valueRec.Code)
	assert.Equal(t, "application/json", valueRec.Header().Get("Content-Type"))
	var got models.Metrics
	require.NoError(t, json.NewDecoder(valueRec.Body).Decode(&got))
	require.NotNil(t, got.Value)
	assert.Equal(t, value, *got.Value)
}

func TestJSONEndpointsRejectEmptyRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		body    string
	}{
		{name: "update empty id", handler: New(newStorageMock()).Update, body: `{"type":"gauge","value":1}`},
		{name: "update blank id", handler: New(newStorageMock()).Update, body: `{"id":"   ","type":"gauge","value":1}`},
		{name: "update empty type", handler: New(newStorageMock()).Update, body: `{"id":"Alloc","value":1}`},
		{name: "update gauge without value", handler: New(newStorageMock()).Update, body: `{"id":"Alloc","type":"gauge"}`},
		{name: "update counter without delta", handler: New(newStorageMock()).Update, body: `{"id":"PollCount","type":"counter"}`},
		{name: "value empty id", handler: New(newStorageMock()).Value, body: `{"type":"gauge"}`},
		{name: "value empty type", handler: New(newStorageMock()).Value, body: `{"id":"Alloc"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			tt.handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func newStorageMock() *storageMock {
	return &storageMock{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func TestAgentUpdateAndValueIntegration(t *testing.T) {
	storage := repository.NewMemStorage()
	h := New(storage)
	router := chi.NewRouter()
	router.Post("/update", h.Update)
	router.Post("/value/", h.Value)
	server := httptest.NewServer(router)
	defer server.Close()

	client := resty.New().SetTransport(server.Client().Transport)
	sender := agent.NewSender(server.URL, client)
	value := 42.5
	require.NoError(t, sender.Send([]models.Metrics{
		{ID: "Alloc", MType: models.Gauge, Value: &value},
	}))

	body, err := json.Marshal(models.Metrics{ID: "Alloc", MType: models.Gauge})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var metric models.Metrics
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&metric))
	require.NotNil(t, metric.Value)
	assert.Equal(t, value, *metric.Value)
}
