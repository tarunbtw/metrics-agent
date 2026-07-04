package collector

import (
	"sync"
	"testing"
	"time"
)

// TestNew_NoLatestBeforeScrape checks that Latest() returns nil before any
// scrape has run.
func TestNew_NoLatestBeforeScrape(t *testing.T) {
	c := New(10 * time.Second)
	if got := c.Latest(); got != nil {
		t.Fatalf("expected nil before first scrape, got %+v", got)
	}
}

// TestScrape_PopulatesFields calls scrape() directly and verifies that every
// field has a plausible value. We do bounds checks only — no exact values,
// because this reads real OS state.
func TestScrape_PopulatesFields(t *testing.T) {
	c := New(10 * time.Second)
	c.scrape()

	s := c.Latest()
	if s == nil {
		t.Fatal("expected non-nil snapshot after scrape()")
	}

	if s.CPUPercent < 0 || s.CPUPercent > 100 {
		t.Errorf("CPUPercent out of range: %v", s.CPUPercent)
	}
	if s.MemPercent < 0 || s.MemPercent > 100 {
		t.Errorf("MemPercent out of range: %v", s.MemPercent)
	}
	if s.LoadAvg1 < 0 {
		t.Errorf("LoadAvg1 negative: %v", s.LoadAvg1)
	}
	if s.LoadAvg5 < 0 {
		t.Errorf("LoadAvg5 negative: %v", s.LoadAvg5)
	}
	if s.LoadAvg15 < 0 {
		t.Errorf("LoadAvg15 negative: %v", s.LoadAvg15)
	}
	if s.ProcessCount <= 0 {
		t.Errorf("ProcessCount should be > 0, got %d", s.ProcessCount)
	}
	if s.UptimeSeconds < 0 {
		t.Errorf("UptimeSeconds negative: %v", s.UptimeSeconds)
	}
}

// TestConcurrentLatest hammers Latest() from many goroutines while scrape()
// is running concurrently. Run with -race to detect data races.
func TestConcurrentLatest(t *testing.T) {
	c := New(10 * time.Second)
	c.scrape() // ensure there is an initial snapshot

	var wg sync.WaitGroup
	const readers = 20

	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = c.Latest()
			}
		}()
	}

	// Concurrently run scrapes while readers are active.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for k := 0; k < 5; k++ {
			c.scrape()
			time.Sleep(10 * time.Millisecond)
		}
	}()

	wg.Wait()
}
