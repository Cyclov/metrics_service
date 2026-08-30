package server

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/Cyclov/metrics_service/internal/config"
	"github.com/Cyclov/metrics_service/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunServesRequestsAndStopsOnContextCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() {
		runErr <- Run(ctx, config.ServerSettings{SrvAdr: address})
	}()

	client := &http.Client{Timeout: 100 * time.Millisecond}
	require.Eventually(t, func() bool {
		resp, err := client.Get("http://" + address + "/ping")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 2*time.Second, 20*time.Millisecond)

	cancel()
	select {
	case err := <-runErr:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop after context cancellation")
	}
}

func TestStoreMetricsSavesDataOnContextCancellation(t *testing.T) {
	storage := repository.NewMemStorage()
	require.NoError(t, storage.SetGauge(context.Background(), "Alloc", 12.5))
	_, err := storage.AddCounter(context.Background(), "PollCount", 3)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "metrics.json")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		storeMetrics(ctx, storage, path, time.Hour)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("metrics store did not stop after context cancellation")
	}

	restored := repository.NewMemStorage()
	require.NoError(t, restored.Load(path))
	gauge, found, err := restored.Gauge(context.Background(), "Alloc")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 12.5, gauge)
	counter, found, err := restored.Counter(context.Background(), "PollCount")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, int64(3), counter)
}
