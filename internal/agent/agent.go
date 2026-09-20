package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"sync"
	"time"

	models "github.com/Cyclov/metrics_service/internal/model"
	"github.com/Cyclov/metrics_service/internal/sign"
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

	runtimeGauges := map[string]float64{
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
	for name, value := range runtimeGauges {
		c.gauges[name] = value
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
	key     string
}

var sendRetryDelays = []time.Duration{time.Second, 3 * time.Second, 5 * time.Second}

func NewSender(baseURL string, client *resty.Client, key string) *Sender {
	if client == nil {
		client = resty.New().SetTimeout(5 * time.Second)
	}
	client.SetTransport(newRetryingTransport(client.GetClient().Transport, sendRetryDelays))

	return &Sender{baseURL: baseURL, client: client, key: key}
}

func (s *Sender) Send(metrics []models.Metrics) error {
	return s.SendContext(context.Background(), metrics)
}

func (s *Sender) SendContext(ctx context.Context, metrics []models.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}
	return s.post(ctx, metrics)
}

func (s *Sender) post(ctx context.Context, metrics []models.Metrics) error {
	var body bytes.Buffer
	zw := gzip.NewWriter(&body)
	if err := json.NewEncoder(zw).Encode(metrics); err != nil {
		_ = zw.Close()
		return fmt.Errorf("encode metrics batch: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("compress metrics batch: %w", err)
	}

	req := s.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Content-Encoding", "gzip").
		SetHeader("Accept-Encoding", "gzip").
		SetBody(body.Bytes())
	if s.key != "" {
		req.SetHeader("HashSHA256", sign.SignHex(body.Bytes(), s.key))
	}
	resp, err := req.Post(s.baseURL + "/updates/")
	if err != nil {
		return err
	}
	if !resp.IsSuccess() {
		return fmt.Errorf("server returned status %s", resp.Status())
	}
	return nil
}

func Run(ctx context.Context, collector *Collector, sender *Sender, pollInterval, reportInterval time.Duration, rateLimit int64) error {
	if pollInterval <= 0 || reportInterval <= 0 {
		return errors.New("poll and report intervals must be positive")
	}
	if rateLimit < 1 {
		return fmt.Errorf("rate limit must be positive: %d", rateLimit)
	}
	if ctx.Err() != nil {
		return nil
	}

	jobs := make(chan []models.Metrics)
	var wg sync.WaitGroup

	wg.Go(func() {
		runCollector(ctx, pollInterval, collector.Poll)
	})

	wg.Go(func() {
		primeCPUPercent(ctx)
		runCollector(ctx, pollInterval, func() { collectSystem(ctx, collector) })
	})

	wg.Go(func() {
		defer close(jobs)
		ticker := time.NewTicker(reportInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case <-ctx.Done():
					return
				case jobs <- collector.CurrentMetrics():
				}
			}
		}
	})

	for range rateLimit {
		wg.Go(func() {
			for metrics := range jobs {
				if err := sender.SendContext(ctx, metrics); err != nil && !errors.Is(err, context.Canceled) {
					log.Printf("failed to send metrics: %v", err)
				}
			}
		})
	}

	<-ctx.Done()
	wg.Wait()
	return nil
}

func runCollector(ctx context.Context, interval time.Duration, collect func()) {
	collect()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			collect()
		}
	}
}
