package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cyclov/metrics_service/internal/agent"
	"github.com/Cyclov/metrics_service/internal/handler"
	models "github.com/Cyclov/metrics_service/internal/model"
	"github.com/Cyclov/metrics_service/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashMiddlewareWithAgent(t *testing.T) {
	storage := repository.NewMemStorage()
	h := handler.HashMiddleware("test-key")(
		handler.GzipMiddleware(http.HandlerFunc(handler.New(storage, nil).Updates)),
	)
	server := httptest.NewServer(h)
	defer server.Close()

	sender := agent.NewSender(server.URL, resty.New(), "test-key")
	delta := int64(3)
	require.NoError(t, sender.Send([]models.Metrics{{
		ID:    "PollCount",
		MType: models.Counter,
		Delta: &delta,
	}}))

	value, found, err := storage.Counter(context.Background(), "PollCount")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, delta, value)
}

func TestAgentUpdateAndValue(t *testing.T) {
	storage := repository.NewMemStorage()
	h := handler.New(storage, nil)
	router := chi.NewRouter()
	router.Use(handler.GzipMiddleware)
	router.Post("/update/", h.Update)
	router.Post("/updates/", h.Updates)
	router.Post("/value/", h.Value)
	server := httptest.NewServer(router)
	defer server.Close()

	client := resty.New().SetTransport(server.Client().Transport)
	sender := agent.NewSender(server.URL, client, "")
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
