package repository

import (
	"context"
	"path/filepath"
	"testing"

	models "github.com/Cyclov/metrics_service/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemStorageSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")
	storage := NewMemStorage()
	require.NoError(t, storage.SetGauge(context.Background(), "Alloc", 12.5))
	_, err := storage.AddCounter(context.Background(), "PollCount", 3)
	require.NoError(t, err)

	require.NoError(t, storage.Save(path))

	restored := NewMemStorage()
	require.NoError(t, restored.Load(path))
	gauge, found, err := restored.Gauge(context.Background(), "Alloc")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 12.5, gauge)
	counter, found, err := restored.Counter(context.Background(), "PollCount")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, int64(3), counter)
}

func TestMemStorageLoadMissingFile(t *testing.T) {
	storage := NewMemStorage()
	require.NoError(t, storage.Load(filepath.Join(t.TempDir(), "missing.json")))
}

func TestMemStorageUpdateBatch(t *testing.T) {
	storage := NewMemStorage()
	gauge := 12.5
	delta1 := int64(2)
	delta2 := int64(3)

	require.NoError(t, storage.UpdateBatch(context.Background(), []models.Metrics{
		{ID: "Alloc", MType: models.Gauge, Value: &gauge},
		{ID: "PollCount", MType: models.Counter, Delta: &delta1},
		{ID: "PollCount", MType: models.Counter, Delta: &delta2},
	}))

	storedGauge, found, err := storage.Gauge(context.Background(), "Alloc")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, gauge, storedGauge)
	storedCounter, found, err := storage.Counter(context.Background(), "PollCount")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, int64(5), storedCounter)
}

func TestMemStorageRejectsWholeInvalidBatch(t *testing.T) {
	storage := NewMemStorage()
	gauge := 12.5
	err := storage.UpdateBatch(context.Background(), []models.Metrics{
		{ID: "Alloc", MType: models.Gauge, Value: &gauge},
		{ID: "Broken", MType: models.Counter},
	})
	require.Error(t, err)
	_, found, getErr := storage.Gauge(context.Background(), "Alloc")
	require.NoError(t, getErr)
	assert.False(t, found)
}
