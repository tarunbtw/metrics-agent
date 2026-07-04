package exporter

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tarunbtw/metrics-agent/internal/collector"
)

// SnapshotSource is satisfied by *collector.Collector and allows easy stubbing in tests.
type SnapshotSource interface {
	Latest() *collector.Snapshot
}

// Exporter writes Prometheus exposition format and JSON responses.
type Exporter struct {
	collector SnapshotSource
	hostname  string
	env       string
}

// New creates an Exporter that reads hostname from os.Hostname and env from
// the ENV environment variable.
func New(c SnapshotSource) *Exporter {
	hostname, _ := os.Hostname()
	env := os.Getenv("ENV")
	if env == "" {
		env = "development"
	}
	return NewWithOpts(c, hostname, env)
}

// NewWithOpts creates an Exporter with explicit hostname and env values.
// Intended for use in tests.
func NewWithOpts(c SnapshotSource, hostname, env string) *Exporter {
	return &Exporter{
		collector: c,
		hostname:  hostname,
		env:       env,
	}
}

func (e *Exporter) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	s := e.collector.Latest()
	if s == nil {
		slog.Warn("HandleMetrics: no snapshot available yet")
		http.Error(w, "no data yet", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(e.prometheusFormat(s))) //nolint:errcheck
}

func (e *Exporter) HandleSnapshot(w http.ResponseWriter, r *http.Request) {
	s := e.collector.Latest()
	if s == nil {
		slog.Warn("HandleSnapshot: no snapshot available yet")
		http.Error(w, "no data yet", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(s); err != nil {
		slog.Error("HandleSnapshot: encode error", "error", err)
	}
}

func (e *Exporter) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":    "ok",
		"timestamp": time.Now().UTC(),
		"hostname":  e.hostname,
		"env":       e.env,
	}); err != nil {
		slog.Error("HandleHealth: encode error", "error", err)
	}
}

func (e *Exporter) prometheusFormat(s *collector.Snapshot) string {
	// Use %q for hostname and env so a double-quote in either value is escaped
	// and cannot corrupt the Prometheus exposition format.
	labels := fmt.Sprintf(`hostname=%q,env=%q`, e.hostname, e.env)
	var b strings.Builder

	// CPU
	b.WriteString("# HELP agent_cpu_percent Current CPU usage percentage\n")
	b.WriteString("# TYPE agent_cpu_percent gauge\n")
	b.WriteString(fmt.Sprintf("agent_cpu_percent{%s} %.2f\n\n", labels, s.CPUPercent))

	// Memory
	b.WriteString("# HELP agent_memory_total_bytes Total memory in bytes\n")
	b.WriteString("# TYPE agent_memory_total_bytes gauge\n")
	b.WriteString(fmt.Sprintf("agent_memory_total_bytes{%s} %d\n\n", labels, s.MemTotal))

	b.WriteString("# HELP agent_memory_used_bytes Used memory in bytes\n")
	b.WriteString("# TYPE agent_memory_used_bytes gauge\n")
	b.WriteString(fmt.Sprintf("agent_memory_used_bytes{%s} %d\n\n", labels, s.MemUsed))

	b.WriteString("# HELP agent_memory_used_percent Memory usage percentage\n")
	b.WriteString("# TYPE agent_memory_used_percent gauge\n")
	b.WriteString(fmt.Sprintf("agent_memory_used_percent{%s} %.2f\n\n", labels, s.MemPercent))

	// Disk
	b.WriteString("# HELP agent_disk_used_percent Disk usage percentage per mount\n")
	b.WriteString("# TYPE agent_disk_used_percent gauge\n")
	for _, d := range s.DiskStats {
		b.WriteString(fmt.Sprintf(
			"agent_disk_used_percent{%s,mount=%q} %.2f\n",
			labels, d.Mount, d.UsedPercent,
		))
	}
	b.WriteString("\n")

	b.WriteString("# HELP agent_disk_used_bytes Disk used bytes per mount\n")
	b.WriteString("# TYPE agent_disk_used_bytes gauge\n")
	for _, d := range s.DiskStats {
		b.WriteString(fmt.Sprintf(
			"agent_disk_used_bytes{%s,mount=%q} %d\n",
			labels, d.Mount, d.Used,
		))
	}
	b.WriteString("\n")

	// Load
	b.WriteString("# HELP agent_load_avg_1m Load average 1 minute\n")
	b.WriteString("# TYPE agent_load_avg_1m gauge\n")
	b.WriteString(fmt.Sprintf("agent_load_avg_1m{%s} %.2f\n\n", labels, s.LoadAvg1))

	b.WriteString("# HELP agent_load_avg_5m Load average 5 minutes\n")
	b.WriteString("# TYPE agent_load_avg_5m gauge\n")
	b.WriteString(fmt.Sprintf("agent_load_avg_5m{%s} %.2f\n\n", labels, s.LoadAvg5))

	b.WriteString("# HELP agent_load_avg_15m Load average 15 minutes\n")
	b.WriteString("# TYPE agent_load_avg_15m gauge\n")
	b.WriteString(fmt.Sprintf("agent_load_avg_15m{%s} %.2f\n\n", labels, s.LoadAvg15))

	// Processes
	b.WriteString("# HELP agent_process_count Number of running processes\n")
	b.WriteString("# TYPE agent_process_count gauge\n")
	b.WriteString(fmt.Sprintf("agent_process_count{%s} %d\n\n", labels, s.ProcessCount))

	// Uptime
	b.WriteString("# HELP agent_uptime_seconds Agent uptime in seconds\n")
	b.WriteString("# TYPE agent_uptime_seconds counter\n")
	b.WriteString(fmt.Sprintf("agent_uptime_seconds{%s} %.0f\n\n", labels, s.UptimeSeconds))

	return b.String()
}
