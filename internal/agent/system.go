package agent

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

func primeCPUPercent(ctx context.Context) {
	_, err := cpu.PercentWithContext(ctx, 0, true)
	if err != nil {
		logSystemError(err)
	}
}

func collectSystem(ctx context.Context, collector *Collector) {
	memory, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		logSystemError(err)
		return
	}
	usage, err := cpu.PercentWithContext(ctx, 0, true)
	if err != nil {
		logSystemError(err)
		return
	}

	collector.setSystemMetrics(memory.Total, memory.Free, usage)
}

func (c *Collector) setSystemMetrics(total, free uint64, usage []float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gauges["TotalMemory"] = float64(total)
	c.gauges["FreeMemory"] = float64(free)
	for name := range c.gauges {
		if strings.HasPrefix(name, "CPUutilization") {
			delete(c.gauges, name)
		}
	}
	for i, value := range usage {
		c.gauges["CPUutilization"+strconv.Itoa(i+1)] = value
	}
}

func logSystemError(err error) {
	if !errors.Is(err, context.Canceled) {
		log.Printf("failed to collect system metrics: %v", err)
	}
}
