package repository

import "sync"

type Storage interface {
	AddGauge(string, float64)
	AddCounter(string, int64)
	Gauge(string) (float64, bool)
	Counter(string) (int64, bool)
	AllMetrics() (map[string]float64, map[string]int64)
}

type MemStorage struct {
	mu      sync.RWMutex
	gauge   map[string]float64
	counter map[string]int64
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
