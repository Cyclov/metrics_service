package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Cyclov/metrics_service/internal/repository"
)

const (
	TypeGauge   = "gauge"
	TypeCounter = "counter"
)

type Handler struct {
	storage repository.Storage
}

func New(storage repository.Storage) *Handler {
	return &Handler{storage: storage}
}

func (h *Handler) Update(resp http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(resp, "Only POST requests are allowed!", http.StatusMethodNotAllowed)
		return
	}

	metricType := strings.ToLower(req.PathValue("type"))
	metricName := req.PathValue("name")
	metricValue := req.PathValue("value")

	switch metricType {
	case TypeGauge:
		value, err := strconv.ParseFloat(metricValue, 64)
		if err != nil {
			http.Error(resp, "Wrong value type!", http.StatusBadRequest)
			return
		}
		h.storage.AddGauge(metricName, value)
	case TypeCounter:
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
