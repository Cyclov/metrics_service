package repository

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemStorageSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")
	storage := NewMemStorage()
	storage.AddGauge("Alloc", 12.5)
	storage.AddCounter("PollCount", 3)

	require.NoError(t, storage.Save(path))

	restored := NewMemStorage()
	require.NoError(t, restored.Load(path))
	gauge, found := restored.Gauge("Alloc")
	require.True(t, found)
	assert.Equal(t, 12.5, gauge)
	counter, found := restored.Counter("PollCount")
	require.True(t, found)
	assert.Equal(t, int64(3), counter)
}

func TestMemStorageLoadMissingFile(t *testing.T) {
	storage := NewMemStorage()
	require.NoError(t, storage.Load(filepath.Join(t.TempDir(), "missing.json")))
}
