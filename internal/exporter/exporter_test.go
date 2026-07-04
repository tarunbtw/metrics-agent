package exporter_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tarunbtw/metrics-agent/internal/collector"
	"github.com/tarunbtw/metrics-agent/internal/exporter"
)

// stubCollector is a minimal collector double used in tests.
type stubCollector struct {
	snap *collector.Snapshot
}

func (s *stubCollector) Latest() *collector.Snapshot { return s.snap }

// newTestExporter builds an Exporter with an injected hostname and env,
// bypassing os.Hostname / os.Getenv.
func newTestExporter(c exporter.SnapshotSource, hostname, env string) *exporter.Exporter {
	return exporter.NewWithOpts(c, hostname, env)
}

func goodSnapshot() *collector.Snapshot {
	return &collector.Snapshot{
		CollectedAt:   time.Now(),
		CPUPercent:    12.34,
		MemTotal:      8 * 1024 * 1024 * 1024,
		MemUsed:       4 * 1024 * 1024 * 1024,
		MemPercent:    50.0,
		LoadAvg1:      0.5,
		LoadAvg5:      0.6,
		LoadAvg15:     0.7,
		ProcessCount:  100,
		UptimeSeconds: 3600,
		DiskStats: []collector.DiskStat{
			{Mount: "/", Total: 100e9, Used: 40e9, UsedPercent: 40.0},
		},
	}
}

// --- HandleMetrics tests ---

func TestHandleMetrics_503WhenNoSnapshot(t *testing.T) {
	exp := newTestExporter(&stubCollector{nil}, "host1", "test")
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	exp.HandleMetrics(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

func TestHandleMetrics_ContainsExpectedNames(t *testing.T) {
	exp := newTestExporter(&stubCollector{goodSnapshot()}, "node-1", "production")
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	exp.HandleMetrics(rec, req)

	body := rec.Body.String()

	wantSubstrings := []string{
		"# HELP agent_cpu_percent",
		"# TYPE agent_cpu_percent gauge",
		"# HELP agent_memory_used_percent",
		"# TYPE agent_memory_used_percent gauge",
		`hostname="node-1"`,
		`env="production"`,
	}
	for _, s := range wantSubstrings {
		if !strings.Contains(body, s) {
			t.Errorf("body missing %q\nfull body:\n%s", s, body)
		}
	}
}

func TestHandleMetrics_QuoteInLabel(t *testing.T) {
	// hostname containing a double-quote must not corrupt the Prometheus format.
	exp := newTestExporter(&stubCollector{goodSnapshot()}, `bad"host`, "prod")
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	exp.HandleMetrics(rec, req)
	body := rec.Body.String()

	// Each line must have balanced label braces: no raw unescaped quote after =
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		// The label section is between the first '{' and '}'
		open := strings.Index(line, "{")
		close := strings.LastIndex(line, "}")
		if open == -1 || close == -1 || close < open {
			t.Errorf("malformed metric line: %q", line)
			continue
		}
		// Verify that the hostname value is properly quoted (escaped)
		labels := line[open+1 : close]
		if strings.Contains(labels, `hostname="bad"host"`) {
			t.Errorf("unescaped quote in labels: %q", labels)
		}
	}
}

// --- HandleSnapshot tests ---

func TestHandleSnapshot_ValidJSON(t *testing.T) {
	exp := newTestExporter(&stubCollector{goodSnapshot()}, "h", "e")
	req := httptest.NewRequest(http.MethodGet, "/snapshot", nil)
	rec := httptest.NewRecorder()

	exp.HandleSnapshot(rec, req)

	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected application/json content-type, got %s", ct)
	}

	var out collector.Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not valid JSON: %v\nbody: %s", err, rec.Body.String())
	}
}

// --- HandleHealth tests ---

func TestHandleHealth_RequiredKeys(t *testing.T) {
	exp := newTestExporter(&stubCollector{goodSnapshot()}, "h", "e")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	exp.HandleHealth(rec, req)

	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}

	for _, key := range []string{"status", "hostname", "env", "timestamp"} {
		if _, ok := out[key]; !ok {
			t.Errorf("missing key %q in health response", key)
		}
	}
	if out["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", out["status"])
	}
}
