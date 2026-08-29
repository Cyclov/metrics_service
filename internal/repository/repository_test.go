package repository

import (
	"context"
	"path/filepath"
	"testing"

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
