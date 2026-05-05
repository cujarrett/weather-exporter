package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
	_ "time/tzdata"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const openMeteoURL = "https://api.open-meteo.com/v1/forecast"

var (
	precipitationGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "weather_precipitation_mm",
			Help: "Current hour precipitation in millimeters from Open-Meteo",
		},
		[]string{"latitude", "longitude"},
	)

	httpClient = &http.Client{Timeout: 10 * time.Second}
)

type openMeteoResponse struct {
	Hourly struct {
		Time          []string  `json:"time"`
		Precipitation []float64 `json:"precipitation"`
	} `json:"hourly"`
}

type exporter struct {
	lat      string
	lon      string
	timezone string
	interval time.Duration

	mu          sync.RWMutex
	lastFetch   time.Time
	cachedValue float64
}

func (e *exporter) fetchPrecipitation() (float64, error) {
	url := fmt.Sprintf(
		"%s?latitude=%s&longitude=%s&hourly=precipitation&timezone=%s&forecast_days=1&past_days=1",
		openMeteoURL, e.lat, e.lon, e.timezone,
	)

	resp, err := httpClient.Get(url)
	if err != nil {
		return 0, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var data openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}

	loc, err := time.LoadLocation(e.timezone)
	if err != nil {
		return 0, fmt.Errorf("load location %q: %w", e.timezone, err)
	}

	currentHour := time.Now().In(loc).Add(-1 * time.Hour).Truncate(time.Hour).Format("2006-01-02T15:00")
	for i, t := range data.Hourly.Time {
		if t == currentHour {
			return data.Hourly.Precipitation[i], nil
		}
	}

	return 0, fmt.Errorf("current hour %q not found in response", currentHour)
}

func (e *exporter) update() {
	val, err := e.fetchPrecipitation()
	if err != nil {
		log.Printf("ERROR fetching precipitation: %v", err)
		return
	}

	e.mu.Lock()
	e.cachedValue = val
	e.lastFetch = time.Now()
	e.mu.Unlock()

	precipitationGauge.WithLabelValues(e.lat, e.lon).Set(val)
	log.Printf("precipitation updated: %.2f mm (lat=%s lon=%s)", val, e.lat, e.lon)
}

func (e *exporter) run() {
	e.update()
	ticker := time.NewTicker(e.interval)
	for range ticker.C {
		e.update()
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	exp := &exporter{
		lat:      getEnv("LATITUDE", "40.5142"),
		lon:      getEnv("LONGITUDE", "-88.9906"),
		timezone: getEnv("TIMEZONE", "America/Chicago"),
		interval: 10 * time.Minute,
	}

	port := getEnv("PORT", "8080")

	prometheus.MustRegister(precipitationGauge)

	go exp.run()

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("starting weather-exporter on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
