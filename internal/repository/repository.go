package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	models "github.com/Cyclov/metrics_service/internal/model"
)

type Storage interface {
	SetGauge(context.Context, string, float64) error
	AddCounter(context.Context, string, int64) (int64, error)
	UpdateBatch(context.Context, []models.Metrics) error
	Gauge(context.Context, string) (float64, bool, error)
	Counter(context.Context, string) (int64, bool, error)
	AllMetrics(context.Context) (map[string]float64, map[string]int64, error)
}

type MemStorage struct {
	mu      sync.RWMutex
	fileMu  sync.Mutex
	gauge   map[string]float64
	counter map[string]int64
}

var _ Storage = (*MemStorage)(nil)

func (storage *MemStorage) Save(path string) error {
	storage.fileMu.Lock()
	defer storage.fileMu.Unlock()

	gauges, counters, err := storage.AllMetrics(context.Background())
	if err != nil {
		return err
	}
	metrics := make([]models.Metrics, 0, len(gauges)+len(counters))

	for name, value := range gauges {
		value := value
		metrics = append(metrics, models.Metrics{ID: name, MType: models.Gauge, Value: &value})
	}
	for name, value := range counters {
		value := value
		metrics = append(metrics, models.Metrics{ID: name, MType: models.Counter, Delta: &value})
	}
	sort.Slice(metrics, func(i, j int) bool { return metrics[i].ID < metrics[j].ID })

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(file).Encode(metrics); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func (storage *MemStorage) Load(path string) error {
	storage.fileMu.Lock()
	defer storage.fileMu.Unlock()

	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	var metrics []models.Metrics
	if err := json.NewDecoder(file).Decode(&metrics); err != nil {
		return err
	}

	gauges := make(map[string]float64)
	counters := make(map[string]int64)
	for _, metric := range metrics {
		switch metric.MType {
		case models.Gauge:
			if metric.Value != nil {
				gauges[metric.ID] = *metric.Value
			}
		case models.Counter:
			if metric.Delta != nil {
				counters[metric.ID] = *metric.Delta
			}
		}
	}

	storage.mu.Lock()
	storage.gauge = gauges
	storage.counter = counters
	storage.mu.Unlock()
	return nil
}

func (storage *MemStorage) AddCounter(ctx context.Context, name string, value int64) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	storage.mu.Lock()
	defer storage.mu.Unlock()
	storage.counter[name] += value
	return storage.counter[name], nil
}

func (storage *MemStorage) SetGauge(ctx context.Context, name string, value float64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	storage.mu.Lock()
	defer storage.mu.Unlock()
	storage.gauge[name] = value
	return nil
}

func (storage *MemStorage) UpdateBatch(ctx context.Context, metrics []models.Metrics) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateBatch(metrics); err != nil {
		return err
	}

	storage.mu.Lock()
	defer storage.mu.Unlock()

	for _, metric := range metrics {
		switch metric.MType {
		case models.Gauge:
			storage.gauge[metric.ID] = *metric.Value
		case models.Counter:
			storage.counter[metric.ID] += *metric.Delta
		}
	}
	return nil
}

func validateBatch(metrics []models.Metrics) error {
	var validationErr error
	for index, metric := range metrics {
		var metricErr error
		if strings.TrimSpace(metric.ID) == "" {
			metricErr = errors.Join(metricErr, errors.New("metric ID is required"))
		}
		switch metric.MType {
		case models.Gauge:
			if metric.Value == nil {
				metricErr = errors.Join(metricErr, fmt.Errorf("gauge %q has no value", metric.ID))
			}
		case models.Counter:
			if metric.Delta == nil {
				metricErr = errors.Join(metricErr, fmt.Errorf("counter %q has no delta", metric.ID))
			}
		default:
			metricErr = errors.Join(metricErr, fmt.Errorf("unsupported metric type %q", metric.MType))
		}
		if metricErr != nil {
			validationErr = errors.Join(validationErr, fmt.Errorf("metric[%d]: %w", index, metricErr))
		}
	}
	return validationErr
}

func ValidateBatch(metrics []models.Metrics) error { return validateBatch(metrics) }

func (storage *MemStorage) Gauge(ctx context.Context, name string) (float64, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	value, ok := storage.gauge[name]
	return value, ok, nil
}

func (storage *MemStorage) Counter(ctx context.Context, name string) (int64, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	value, ok := storage.counter[name]
	return value, ok, nil
}

func (storage *MemStorage) AllMetrics(ctx context.Context) (map[string]float64, map[string]int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	storage.mu.RLock()
	defer storage.mu.RUnlock()

	gauges := make(map[string]float64, len(storage.gauge))
	for name, value := range storage.gauge {
		gauges[name] = value
	}

	counters := make(map[string]int64, len(storage.counter))
	for name, value := range storage.counter {
		counters[name] = value
	}

	return gauges, counters, nil
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauge:   make(map[string]float64),
		counter: make(map[string]int64),
	}
}
