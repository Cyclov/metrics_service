package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"sync"
	"time"

	models "github.com/Cyclov/metrics_service/internal/model"
	"github.com/go-resty/resty/v2"
)

type Collector struct {
	mu        sync.RWMutex
	gauges    map[string]float64
	pollCount int64
	random    *rand.Rand
}

func NewCollector() *Collector {
	return &Collector{
		gauges: make(map[string]float64, 29),
		random: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (c *Collector) Poll() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.gauges = map[string]float64{
		"Alloc":         float64(m.Alloc),
		"BuckHashSys":   float64(m.BuckHashSys),
		"Frees":         float64(m.Frees),
		"GCCPUFraction": m.GCCPUFraction,
		"GCSys":         float64(m.GCSys),
		"HeapAlloc":     float64(m.HeapAlloc),
		"HeapIdle":      float64(m.HeapIdle),
		"HeapInuse":     float64(m.HeapInuse),
		"HeapObjects":   float64(m.HeapObjects),
		"HeapReleased":  float64(m.HeapReleased),
		"HeapSys":       float64(m.HeapSys),
		"LastGC":        float64(m.LastGC),
		"Lookups":       float64(m.Lookups),
		"MCacheInuse":   float64(m.MCacheInuse),
		"MCacheSys":     float64(m.MCacheSys),
		"MSpanInuse":    float64(m.MSpanInuse),
		"MSpanSys":      float64(m.MSpanSys),
		"Mallocs":       float64(m.Mallocs),
		"NextGC":        float64(m.NextGC),
		"NumForcedGC":   float64(m.NumForcedGC),
		"NumGC":         float64(m.NumGC),
		"OtherSys":      float64(m.OtherSys),
		"PauseTotalNs":  float64(m.PauseTotalNs),
		"StackInuse":    float64(m.StackInuse),
		"StackSys":      float64(m.StackSys),
		"Sys":           float64(m.Sys),
		"TotalAlloc":    float64(m.TotalAlloc),
		"RandomValue":   c.random.Float64(),
	}
	c.pollCount++
}

func (c *Collector) CurrentMetrics() []models.Metrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metrics := make([]models.Metrics, 0, len(c.gauges)+1)
	for name, value := range c.gauges {
		value := value
		metrics = append(metrics, models.Metrics{
			ID:    name,
			MType: models.Gauge,
			Value: &value,
		})
	}

	pollCount := c.pollCount
	metrics = append(metrics, models.Metrics{
		ID:    "PollCount",
		MType: models.Counter,
		Delta: &pollCount,
	})

	return metrics
}

type Sender struct {
	baseURL string
	client  *resty.Client
}

func NewSender(baseURL string, client *resty.Client) *Sender {

	if client == nil {
		client = resty.New().SetTimeout(5 * time.Second)
	}

	return &Sender{baseURL: baseURL, client: client}
}

func (s *Sender) Send(metrics []models.Metrics) error {
	for _, metric := range metrics {
		if err := s.post(metric); err != nil {
			return err
		}
	}

	return nil
}

func (s *Sender) post(metric models.Metrics) error {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(metric); err != nil {
		return fmt.Errorf("encode metric %q: %w", metric.ID, err)
	}

	resp, err := s.client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(body.Bytes()).
		Post(s.baseURL + "/update")

	if err != nil {
		return err
	}

	if !resp.IsSuccess() {
		return fmt.Errorf("server returned status %s", resp.Status())
	}

	return nil
}

func Run(collector *Collector, sender *Sender, pollInterval, reportInterval time.Duration) error {

	collector.Poll()
	pollTicker := time.NewTicker(pollInterval)
	reportTicker := time.NewTicker(reportInterval)

	defer pollTicker.Stop()
	defer reportTicker.Stop()

	for {
		select {
		case <-pollTicker.C:
			collector.Poll()
		case <-reportTicker.C:
			if err := sender.Send(collector.CurrentMetrics()); err != nil {
				log.Printf("failed to send metrics: %v", err)
			}
		}
	}
}
