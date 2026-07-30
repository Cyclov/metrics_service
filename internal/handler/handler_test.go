package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
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

func TestUpdate(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{"gauge", http.MethodPost, "/update/gauge/Alloc/1.5", http.StatusOK},
		{"counter", http.MethodPost, "/update/counter/PollCount/2", http.StatusOK},
		{"bad type", http.MethodPost, "/update/unknown/x/1", http.StatusBadRequest},
		{"bad value", http.MethodPost, "/update/gauge/x/nope", http.StatusBadRequest},
		{"bad method", http.MethodGet, "/update/gauge/x/1", http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &storageMock{
				gauges:   make(map[string]float64),
				counters: make(map[string]int64),
			}
			h := New(storage)
			mux := http.NewServeMux()
			mux.HandleFunc("/update/{type}/{name}/{value}", h.Update)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
