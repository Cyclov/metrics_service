package repository

type Storage interface {
	AddGauge(string, float64)
	AddCounter(string, int64)
}

type MemStorage struct {
	gauge   map[string]float64
	counter map[string]int64
}

func (storage *MemStorage) AddCounter(name string, value int64) {

	storage.counter[name] += value
}

func (storage *MemStorage) AddGauge(name string, value float64) {

	storage.gauge[name] = value
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauge:   make(map[string]float64),
		counter: make(map[string]int64),
	}
}
