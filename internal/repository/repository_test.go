package repository

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	models "github.com/Cyclov/metrics_service/internal/model"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
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

func TestValidateBatchJoinsAllErrors(t *testing.T) {
	err := ValidateBatch([]models.Metrics{
		{MType: models.Gauge},
		{ID: "PollCount", MType: models.Counter},
		{ID: "Broken", MType: "histogram"},
	})
	require.Error(t, err)
	for _, fragment := range []string{
		"metric[0]: metric ID is required",
		"gauge \"\" has no value",
		"metric[1]: counter \"PollCount\" has no delta",
		"metric[2]: unsupported metric type \"histogram\"",
	} {
		assert.True(t, strings.Contains(err.Error(), fragment), "missing %q in %q", fragment, err)
	}
}

func TestRetryPostgresRetriesAllConnectionErrors(t *testing.T) {
	delays := postgresRetryDelays
	postgresRetryDelays = []time.Duration{0, 0, 0}
	t.Cleanup(func() { postgresRetryDelays = delays })

	attempts := 0
	err := retryPostgres(context.Background(), func() error {
		attempts++
		return &pgconn.PgError{Code: pgerrcode.TransactionResolutionUnknown}
	})
	require.Error(t, err)
	assert.Equal(t, 4, attempts)
}

func TestPostgresRetryClassification(t *testing.T) {
	assert.True(t, isRetriablePostgresError(&pgconn.PgError{Code: pgerrcode.ConnectionFailure}))
	assert.True(t, isRetriablePostgresError(&pgconn.PgError{Code: pgerrcode.TransactionResolutionUnknown}))
	assert.False(t, isRetriablePostgresError(&pgconn.PgError{Code: pgerrcode.UniqueViolation}))
	assert.False(t, isRetriablePostgresError(errors.New("ordinary error")))
}
