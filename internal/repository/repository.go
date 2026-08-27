package repository

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"sync"

	models "github.com/Cyclov/metrics_service/internal/model"
)

type Storage interface {
	AddGauge(string, float64)
	AddCounter(string, int64)
	Gauge(string) (float64, bool)
	Counter(string) (int64, bool)
	AllMetrics() (map[string]float64, map[string]int64)
}

type MemStorage struct {
	mu      sync.RWMutex
	fileMu  sync.Mutex
	gauge   map[string]float64
	counter map[string]int64
}

func (storage *MemStorage) Save(path string) error {
	storage.fileMu.Lock()
	defer storage.fileMu.Unlock()

	gauges, counters := storage.AllMetrics()
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

func (storage *MemStorage) AddCounter(name string, value int64) {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	storage.counter[name] += value
}

func (storage *MemStorage) AddGauge(name string, value float64) {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	storage.gauge[name] = value
}

func (storage *MemStorage) Gauge(name string) (float64, bool) {
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	value, ok := storage.gauge[name]
	return value, ok
}

func (storage *MemStorage) Counter(name string) (int64, bool) {
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	value, ok := storage.counter[name]
	return value, ok
}

func (storage *MemStorage) AllMetrics() (map[string]float64, map[string]int64) {
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

	return gauges, counters
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauge:   make(map[string]float64),
		counter: make(map[string]int64),
	}
}
