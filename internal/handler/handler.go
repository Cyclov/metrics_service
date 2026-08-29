package handler

import (
	"context"
	"encoding/json"
	"html"
	"net/http"
	"strconv"
	"strings"

	"github.com/Cyclov/metrics_service/internal/logger"
	models "github.com/Cyclov/metrics_service/internal/model"
	"github.com/Cyclov/metrics_service/internal/repository"
	"go.uber.org/zap"
)

type Handler struct {
	storage  repository.Storage
	onUpdate func() error
	database DatabasePinger
}

// DatabasePinger describes a database connection health check.
type DatabasePinger interface {
	PingContext(context.Context) error
}

func New(storage repository.Storage, onUpdate func() error, database ...DatabasePinger) *Handler {
	h := &Handler{
		storage:  storage,
		onUpdate: onUpdate,
	}
	if len(database) > 0 {
		h.database = database[0]
	}
	return h
}

func (h *Handler) Ping(resp http.ResponseWriter, req *http.Request) {
	if h.database == nil || h.database.PingContext(req.Context()) != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp.WriteHeader(http.StatusOK)
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
		if err := h.storage.SetGauge(req.Context(), metricName, value); err != nil {
			h.storageError(resp, err)
			return
		}
	case models.Counter:
		value, err := strconv.ParseInt(metricValue, 10, 64)
		if err != nil {
			http.Error(resp, "Wrong value type!", http.StatusBadRequest)
			return
		}
		if _, err := h.storage.AddCounter(req.Context(), metricName, value); err != nil {
			h.storageError(resp, err)
			return
		}
	default:
		http.Error(resp, "Wrong metric type, only gauge and counter types are allowed!", http.StatusBadRequest)
		return
	}
	if !h.storeMetrics(resp) {
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
	if !validateMetricIdentity(resp, metric) {
		return
	}

	switch metric.MType {
	case models.Gauge:
		if metric.Value == nil {
			http.Error(resp, "Gauge value is required", http.StatusBadRequest)
			return
		}
		if err := h.storage.SetGauge(req.Context(), metric.ID, *metric.Value); err != nil {
			h.storageError(resp, err)
			return
		}
	case models.Counter:
		if metric.Delta == nil {
			http.Error(resp, "Counter delta is required", http.StatusBadRequest)
			return
		}
		value, err := h.storage.AddCounter(req.Context(), metric.ID, *metric.Delta)
		if err != nil {
			h.storageError(resp, err)
			return
		}
		metric.Delta = &value
	default:
		http.Error(resp, "Wrong metric type", http.StatusBadRequest)
		return
	}
	if !h.storeMetrics(resp) {
		return
	}

	writeMetric(resp, metric)
}

func (h *Handler) storeMetrics(resp http.ResponseWriter) bool {
	if h.onUpdate == nil {
		return true
	}
	if err := h.onUpdate(); err != nil {
		logger.Log.Error("failed to store metrics", zap.Error(err))
		http.Error(resp, "Failed to store metrics", http.StatusInternalServerError)
		return false
	}
	return true
}

func (h *Handler) Value(resp http.ResponseWriter, req *http.Request) {
	metric, ok := decodeMetric(resp, req)
	if !ok {
		return
	}
	if !validateMetricIdentity(resp, metric) {
		return
	}

	switch metric.MType {
	case models.Gauge:
		value, found, err := h.storage.Gauge(req.Context(), metric.ID)
		if err != nil {
			h.storageError(resp, err)
			return
		}
		if !found {
			logger.Log.Warn("metric not found", zap.String("id", metric.ID), zap.String("type", metric.MType))
			http.NotFound(resp, req)
			return
		}
		metric.Value = &value
	case models.Counter:
		value, found, err := h.storage.Counter(req.Context(), metric.ID)
		if err != nil {
			h.storageError(resp, err)
			return
		}
		if !found {
			logger.Log.Warn("metric not found", zap.String("id", metric.ID), zap.String("type", metric.MType))
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
	defer req.Body.Close()

	var metric models.Metrics
	if err := json.NewDecoder(req.Body).Decode(&metric); err != nil {
		http.Error(resp, "Invalid JSON", http.StatusBadRequest)
		return models.Metrics{}, false
	}
	return metric, true
}

func validateMetricIdentity(resp http.ResponseWriter, metric models.Metrics) bool {
	if strings.TrimSpace(metric.ID) == "" {
		http.Error(resp, "Metric ID is required", http.StatusBadRequest)
		return false
	}
	if strings.TrimSpace(metric.MType) == "" {
		http.Error(resp, "Metric type is required", http.StatusBadRequest)
		return false
	}
	return true
}

func writeMetric(resp http.ResponseWriter, metric models.Metrics) {
	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(http.StatusOK) // не явный статус после ИИ ревью.
	_ = json.NewEncoder(resp).Encode(metric)
}

func (h *Handler) AllMetrics(resp http.ResponseWriter, req *http.Request) {
	gauges, counters, err := h.storage.AllMetrics(req.Context())
	if err != nil {
		h.storageError(resp, err)
		return
	}

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
		metricValue, ok, err := h.storage.Gauge(req.Context(), metricName)
		if err != nil {
			h.storageError(resp, err)
			return
		}
		if !ok {
			http.NotFound(resp, req)
			return
		}
		value = strconv.FormatFloat(metricValue, 'g', -1, 64)
	case models.Counter:
		metricValue, ok, err := h.storage.Counter(req.Context(), metricName)
		if err != nil {
			h.storageError(resp, err)
			return
		}
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

func (h *Handler) storageError(resp http.ResponseWriter, err error) {
	logger.Log.Error("storage operation failed", zap.Error(err))
	http.Error(resp, "Storage operation failed", http.StatusInternalServerError)
}
