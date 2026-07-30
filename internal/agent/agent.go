package agent

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"sync"
	"time"
)

type Metrics struct {
	Gauges    map[string]float64
	PollCount int64
}

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

func (c *Collector) CurrentMetrics() Metrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	gauges := make(map[string]float64, len(c.gauges))
	for name, value := range c.gauges {
		gauges[name] = value
	}
	return Metrics{Gauges: gauges, PollCount: c.pollCount}
}

type Sender struct {
	baseURL string
	client  *http.Client
}

func NewSender(baseURL string, client *http.Client) *Sender {

	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	return &Sender{baseURL: baseURL, client: client}
}

func (sender *Sender) Send(metrics Metrics) error {

	for name, value := range metrics.Gauges {
		if err := sender.post("gauge", name, strconv.FormatFloat(value, 'g', -1, 64)); err != nil {
			return err
		}
	}

	return sender.post("counter", "PollCount", strconv.FormatInt(metrics.PollCount, 10))
}

func (s *Sender) post(metricType, name, value string) error {

	endpoint := fmt.Sprintf("%s/update/%s/%s/%s",
		s.baseURL, metricType, url.PathEscape(name), url.PathEscape(value))
	req, err := http.NewRequest(http.MethodPost, endpoint, nil)

	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "text/plain")

	resp, err := s.client.Do(req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("server returned status %s", resp.Status)
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
				return err
			}
		}
	}
}
