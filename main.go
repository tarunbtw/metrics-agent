package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/tarunbtw/metrics-agent/internal/collector"
	"github.com/tarunbtw/metrics-agent/internal/exporter"
)

func main() {
	log.SetOutput(os.Stdout)

	interval := scrapeInterval()
	port := os.Getenv("PORT")
	if port == "" {
		port = "9100"
	}

	log.Printf("starting metrics agent on :%s, scrape interval: %v\n", port, interval)

	c := collector.New(interval)
	c.Start()

	exp := exporter.New(c)

	http.HandleFunc("/metrics", exp.HandleMetrics)
	http.HandleFunc("/snapshot", exp.HandleSnapshot)
	http.HandleFunc("/health", exp.HandleHealth)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func scrapeInterval() time.Duration {
	val := os.Getenv("SCRAPE_INTERVAL_SECONDS")
	if val == "" {
		return 10 * time.Second
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 1 {
		return 10 * time.Second
	}
	return time.Duration(n) * time.Second
}