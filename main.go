package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
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

	temperatureGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "weather_temperature_fahrenheit",
			Help: "Current hour temperature in Fahrenheit from Open-Meteo",
		},
		[]string{"latitude", "longitude"},
	)

	httpClient = &http.Client{Timeout: 10 * time.Second}
)

type openMeteoResponse struct {
	Hourly struct {
		Time          []string  `json:"time"`
		Precipitation []float64 `json:"precipitation"`
		Temperature   []float64 `json:"temperature_2m"`
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

func (e *exporter) fetch() (precip, temp float64, err error) {
	url := fmt.Sprintf(
		"%s?latitude=%s&longitude=%s&hourly=precipitation,temperature_2m&temperature_unit=fahrenheit&timezone=%s&forecast_days=1&past_days=1",
		openMeteoURL, e.lat, e.lon, e.timezone,
	)

	resp, err := httpClient.Get(url)
	if err != nil {
		return 0, 0, fmt.Errorf("http get: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var data openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, 0, fmt.Errorf("decode response: %w", err)
	}

	loc, err := time.LoadLocation(e.timezone)
	if err != nil {
		return 0, 0, fmt.Errorf("load location %q: %w", e.timezone, err)
	}

	return currentReading(data, time.Now().In(loc))
}

// currentReading picks the last completed hour from the hourly series.
func currentReading(data openMeteoResponse, now time.Time) (precip, temp float64, err error) {
	currentHour := now.Add(-1 * time.Hour).Truncate(time.Hour).Format("2006-01-02T15:00")
	for i, t := range data.Hourly.Time {
		if t == currentHour {
			return data.Hourly.Precipitation[i], data.Hourly.Temperature[i], nil
		}
	}

	return 0, 0, fmt.Errorf("current hour %q not found in response", currentHour)
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (e *exporter) update() {
	precip, temp, err := e.fetch()
	if err != nil {
		log.Printf("ERROR fetching weather: %v", err)
		return
	}

	e.mu.Lock()
	e.cachedValue = precip
	e.lastFetch = time.Now()
	e.mu.Unlock()

	precipitationGauge.WithLabelValues(e.lat, e.lon).Set(precip)
	temperatureGauge.WithLabelValues(e.lat, e.lon).Set(temp)
	log.Printf("weather updated: %.2f mm precip, %.1f°F (lat=%s lon=%s)", precip, temp, e.lat, e.lon)
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

	prometheus.MustRegister(precipitationGauge, temperatureGauge)

	go exp.run()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", healthHandler)
	srv := &http.Server{Addr: ":" + port, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("starting weather-exporter on :%s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")
	// Kubernetes sends SIGKILL after 30s, so finish draining well before that.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
