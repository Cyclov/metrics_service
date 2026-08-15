package handler

import (
	"encoding/json"
	"html"
	"net/http"
	"strconv"
	"strings"

	models "github.com/Cyclov/metrics_service/internal/model"
	"github.com/Cyclov/metrics_service/internal/repository"
)

type Handler struct {
	storage repository.Storage
}

func New(storage repository.Storage) *Handler {
	return &Handler{storage: storage}
}

func (h *Handler) UpdatePath(resp http.ResponseWriter, req *http.Request) {
	metricType := strings.ToLower(req.PathValue("type"))
	metricName := req.PathValue("name")
	metricValue := req.PathValue("value")

	switch metricType {
	case models.Gauge:
		value, err := strconv.ParseFloat(metricValue, 64)
		if err != nil {
			http.Error(resp, "Wrong value type!", http.StatusBadRequest)
			return
		}
		h.storage.AddGauge(metricName, value)
	case models.Counter:
		value, err := strconv.ParseInt(metricValue, 10, 64)
		if err != nil {
			http.Error(resp, "Wrong value type!", http.StatusBadRequest)
			return
		}
		h.storage.AddCounter(metricName, value)
	default:
		http.Error(resp, "Wrong metric type, only gauge and counter types are allowed!", http.StatusBadRequest)
		return
	}

	resp.Header().Set("Content-Type", "text/plain; charset=utf-8")
	resp.WriteHeader(http.StatusOK)

}

func (h *Handler) Update(resp http.ResponseWriter, req *http.Request) {
	metric, ok := decodeMetric(resp, req)
	if !ok {
		return
	}

	switch metric.MType {
	case models.Gauge:
		if metric.Value == nil {
			http.Error(resp, "Gauge value is required", http.StatusBadRequest)
			return
		}
		h.storage.AddGauge(metric.ID, *metric.Value)
	case models.Counter:
		if metric.Delta == nil {
			http.Error(resp, "Counter delta is required", http.StatusBadRequest)
			return
		}
		h.storage.AddCounter(metric.ID, *metric.Delta)
		value, _ := h.storage.Counter(metric.ID)
		metric.Delta = &value
	default:
		http.Error(resp, "Wrong metric type", http.StatusBadRequest)
		return
	}

	writeMetric(resp, metric)
}

func (h *Handler) Value(resp http.ResponseWriter, req *http.Request) {
	metric, ok := decodeMetric(resp, req)
	if !ok {
		return
	}

	switch metric.MType {
	case models.Gauge:
		value, found := h.storage.Gauge(metric.ID)
		if !found {
			http.NotFound(resp, req)
			return
		}
		metric.Value = &value
	case models.Counter:
		value, found := h.storage.Counter(metric.ID)
		if !found {
			http.NotFound(resp, req)
			return
		}
		metric.Delta = &value
	default:
		http.Error(resp, "Wrong metric type", http.StatusBadRequest)
		return
	}

	writeMetric(resp, metric)
}

func decodeMetric(resp http.ResponseWriter, req *http.Request) (models.Metrics, bool) {
	var metric models.Metrics
	if err := json.NewDecoder(req.Body).Decode(&metric); err != nil {
		http.Error(resp, "Invalid JSON", http.StatusBadRequest)
		return models.Metrics{}, false
	}
	return metric, true
}

func writeMetric(resp http.ResponseWriter, metric models.Metrics) {
	resp.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(resp).Encode(metric)
}

func (h *Handler) AllMetrics(resp http.ResponseWriter, req *http.Request) {
	gauges, counters := h.storage.AllMetrics()

	var body strings.Builder
	body.WriteString("<!doctype html><html><body><h2>Gauge</h2>")
	for name, value := range gauges {
		body.WriteString("<p>")
		body.WriteString(html.EscapeString(name))
		body.WriteString(": ")
		body.WriteString(strconv.FormatFloat(value, 'g', -1, 64))
		body.WriteString("</p>")
	}

	body.WriteString("<h2>Counters</h2>")
	for name, value := range counters {
		body.WriteString("<p>")
		body.WriteString(html.EscapeString(name))
		body.WriteString(": ")
		body.WriteString(strconv.FormatInt(value, 10))
		body.WriteString("</p>")
	}
	body.WriteString("</body></html>")

	resp.Header().Set("Content-Type", "text/html; charset=utf-8")
	resp.WriteHeader(http.StatusOK)
	_, _ = resp.Write([]byte(body.String()))
}

func (h *Handler) ValuePath(resp http.ResponseWriter, req *http.Request) {
	metricType := strings.ToLower(req.PathValue("type"))
	metricName := req.PathValue("name")

	var value string
	switch metricType {
	case models.Gauge:
		metricValue, ok := h.storage.Gauge(metricName)
		if !ok {
			http.NotFound(resp, req)
			return
		}
		value = strconv.FormatFloat(metricValue, 'g', -1, 64)
	case models.Counter:
		metricValue, ok := h.storage.Counter(metricName)
		if !ok {
			http.NotFound(resp, req)
			return
		}
		value = strconv.FormatInt(metricValue, 10)
	default:
		http.Error(resp, "Wrong metric type", http.StatusBadRequest)
		return
	}

	resp.Header().Set("Content-Type", "text/plain; charset=utf-8")
	resp.WriteHeader(http.StatusOK)
	_, _ = resp.Write([]byte(value))
}
