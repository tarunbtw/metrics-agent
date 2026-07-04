package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

type DiskStat struct {
	Mount       string  `json:"mount"`
	Total       uint64  `json:"total_bytes"`
	Used        uint64  `json:"used_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

type Snapshot struct {
	CollectedAt   time.Time  `json:"collected_at"`
	CPUPercent    float64    `json:"cpu_percent"`
	MemTotal      uint64     `json:"mem_total_bytes"`
	MemUsed       uint64     `json:"mem_used_bytes"`
	MemPercent    float64    `json:"mem_used_percent"`
	DiskStats     []DiskStat `json:"disk_stats"`
	LoadAvg1      float64    `json:"load_avg_1m"`
	LoadAvg5      float64    `json:"load_avg_5m"`
	LoadAvg15     float64    `json:"load_avg_15m"`
	ProcessCount  int        `json:"process_count"`
	UptimeSeconds float64    `json:"uptime_seconds"`
}

type Collector struct {
	mu        sync.RWMutex
	latest    *Snapshot
	interval  time.Duration
	startedAt time.Time
	done      chan struct{}
}

func New(interval time.Duration) *Collector {
	return &Collector{
		interval:  interval,
		startedAt: time.Now(),
		done:      make(chan struct{}),
	}
}

func (c *Collector) Start() {
	// scrape immediately on start
	c.scrape()

	go func() {
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.scrape()
			case <-c.done:
				return
			}
		}
	}()
}

// Stop signals the background goroutine to exit cleanly.
func (c *Collector) Stop() {
	close(c.done)
}

func (c *Collector) Latest() *Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.latest
}

func (c *Collector) scrape() {
	// Each scrape gets a 2-second deadline so a stuck syscall can't hang indefinitely.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	s := &Snapshot{
		CollectedAt:   time.Now(),
		UptimeSeconds: time.Since(c.startedAt).Seconds(),
	}

	// CPU
	percents, err := cpu.PercentWithContext(ctx, 500*time.Millisecond, false)
	if err != nil {
		slog.Warn("cpu scrape failed", "error", err)
	} else if len(percents) > 0 {
		s.CPUPercent = percents[0]
	}

	// Memory
	vm, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		slog.Warn("memory scrape failed", "error", err)
	} else {
		s.MemTotal = vm.Total
		s.MemUsed = vm.Used
		s.MemPercent = vm.UsedPercent
	}

	// Disk — scrape all partitions
	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		slog.Warn("disk partitions scrape failed", "error", err)
	} else {
		for _, p := range partitions {
			usage, err := disk.UsageWithContext(ctx, p.Mountpoint)
			if err != nil {
				slog.Warn("disk usage scrape failed", "mount", p.Mountpoint, "error", err)
				continue
			}
			s.DiskStats = append(s.DiskStats, DiskStat{
				Mount:       p.Mountpoint,
				Total:       usage.Total,
				Used:        usage.Used,
				UsedPercent: usage.UsedPercent,
			})
		}
	}

	// Load average
	avg, err := load.AvgWithContext(ctx)
	if err != nil {
		slog.Warn("load average scrape failed", "error", err)
	} else {
		s.LoadAvg1 = avg.Load1
		s.LoadAvg5 = avg.Load5
		s.LoadAvg15 = avg.Load15
	}

	// Process count
	procs, err := process.PidsWithContext(ctx)
	if err != nil {
		slog.Warn("process count scrape failed", "error", err)
	} else {
		s.ProcessCount = len(procs)
	}

	c.mu.Lock()
	c.latest = s
	c.mu.Unlock()

	slog.Info("scrape complete",
		"cpu_percent", s.CPUPercent,
		"mem_percent", s.MemPercent,
		"process_count", s.ProcessCount,
	)
}
