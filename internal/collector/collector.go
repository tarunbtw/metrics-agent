package collector

import (
	"log"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

type DiskStat struct {
	Mount       string
	Total       uint64
	Used        uint64
	UsedPercent float64
}

type Snapshot struct {
	CollectedAt    time.Time
	CPUPercent     float64
	MemTotal       uint64
	MemUsed        uint64
	MemPercent     float64
	DiskStats      []DiskStat
	LoadAvg1       float64
	LoadAvg5       float64
	LoadAvg15      float64
	ProcessCount   int
	UptimeSeconds  float64
}

type Collector struct {
	mu       sync.RWMutex
	latest   *Snapshot
	interval time.Duration
	startedAt time.Time
}

func New(interval time.Duration) *Collector {
	return &Collector{
		interval:  interval,
		startedAt: time.Now(),
	}
}

func (c *Collector) Start() {
	// scrape immediately on start
	c.scrape()

	go func() {
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		for range ticker.C {
			c.scrape()
		}
	}()
}

func (c *Collector) Latest() *Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.latest
}

func (c *Collector) scrape() {
	s := &Snapshot{
		CollectedAt:   time.Now(),
		UptimeSeconds: time.Since(c.startedAt).Seconds(),
	}

	// CPU
	percents, err := cpu.Percent(500*time.Millisecond, false)
	if err == nil && len(percents) > 0 {
		s.CPUPercent = percents[0]
	}

	// Memory
	vm, err := mem.VirtualMemory()
	if err == nil {
		s.MemTotal = vm.Total
		s.MemUsed = vm.Used
		s.MemPercent = vm.UsedPercent
	}

	// Disk — scrape all partitions
	partitions, err := disk.Partitions(false)
	if err == nil {
		for _, p := range partitions {
			usage, err := disk.Usage(p.Mountpoint)
			if err != nil {
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
	avg, err := load.Avg()
	if err == nil {
		s.LoadAvg1 = avg.Load1
		s.LoadAvg5 = avg.Load5
		s.LoadAvg15 = avg.Load15
	}

	// Process count
	procs, err := process.Pids()
	if err == nil {
		s.ProcessCount = len(procs)
	}

	c.mu.Lock()
	c.latest = s
	c.mu.Unlock()

	log.Printf("scraped: cpu=%.1f%% mem=%.1f%% processes=%d\n",
		s.CPUPercent, s.MemPercent, s.ProcessCount)
}