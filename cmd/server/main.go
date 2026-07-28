package main

import (
	"net/http"
	"strconv"
	"strings"
)

const (
	TypeGauge   = "gauge"
	TypeCounter = "counter"
)

var storage Storage

func init() {
	storage = NewMemStorage()
}

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

func main() {

	mux := http.NewServeMux()
	mux.HandleFunc("/update/{type}/{name}/{value}", updateHandler)

	err := http.ListenAndServe(":8080", mux)

	if err != nil {
		panic(err)
	}

}

func updateHandler(resp http.ResponseWriter, req *http.Request) {

	if req.Method != http.MethodPost {
		http.Error(resp, "Only POST requests are allowed!", http.StatusMethodNotAllowed)
		return
	}
	metricType := strings.ToLower(req.PathValue("type"))
	metricName := req.PathValue("name")
	metricValue := req.PathValue("value")

	//nameCheck

	//typeCheck
	switch metricType {

	case TypeGauge:
		value, err := strconv.ParseFloat(metricValue, 64)

		if err != nil {
			http.Error(resp, "Wrong value type!", http.StatusBadRequest)
			return
		}

		storage.AddGauge(metricName, value)

	case TypeCounter:
		value, err := strconv.ParseInt(metricValue, 10, 64)

		if err != nil {
			http.Error(resp, "Wrong value type!", http.StatusBadRequest)
			return
		}

		storage.AddCounter(metricName, value)

	default:
		http.Error(resp, "Wrong metric type, only gauge and counter types are allowed!", http.StatusBadRequest)
		return
	}
	resp.Write([]byte(metricName))

}
