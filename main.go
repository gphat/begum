package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	interval        = kingpin.Arg("interval", "Interval to update metrics").Default("1s").Duration()
	address         = kingpin.Arg("address", "Address on which to listen for API calls").Default("127.0.0.1:8080").TCP()
	requestRate     = kingpin.Arg("request_rate", "Rate of requests to simulate, in requests per second").Default("100").Int()
	errorPct        = kingpin.Arg("error_pct", "Percentage of errors to simulate, as percentage").Default("1").Int()
	latencyMinMs    = kingpin.Arg("latency_min_ms", "Minimum latency value in milliseconds").Default("100").Int()
	latencyMaxMs    = kingpin.Arg("latency_max_ms", "Minimum latency value in milliseconds").Default("300").Int()
	latencyOffsetMs = kingpin.Arg("latency_offset_ms", "Offset to add to latency, simulating slowness").Default("0").Int()

	requestProcessMs = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Name:       "request_duration_millis",
		Help:       "The duration of requuests",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"code"})
	errorsEncountered = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "errors_encountered_total",
		Help: "The total number of errors encountered",
	}, []string{"code"})

	mutex = &sync.RWMutex{}
)

func errorRateHandler(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	defer mutex.Unlock()
	newErrorRate, err := strconv.Atoi(r.URL.Path[len("/error_rate/"):])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		errorPct = &newErrorRate
	}
}

func latencyHandler(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	defer mutex.Unlock()
	newLatencyOffset, err := strconv.Atoi(r.URL.Path[len("/latency_offset/"):])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		latencyOffsetMs = &newLatencyOffset
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello!")
}

func main() {
	rand.Seed(time.Now().UnixNano())
	kingpin.Parse()

	http.HandleFunc("/", handler)
	http.HandleFunc("/error_rate/", errorRateHandler)
	http.HandleFunc("/latency_offset/", latencyHandler)
	http.Handle("/metrics", promhttp.Handler())

	beginTicker(*interval)

	stringAddr := fmt.Sprintf("%s:%d", (*address).IP.String(), (*address).Port)
	log.Fatal(http.ListenAndServe(stringAddr, nil))
}

func beginTicker(d time.Duration) {
	ticker := time.NewTicker(d)
	go func() {
		for t := range ticker.C {
			mutex.RLock()
			fmt.Println("Tick at", t)
			for i := 0; i < *requestRate; i++ {
				var status = "200"
				if rand.Intn(100) <= *errorPct {
					status = "500"
					errorsEncountered.WithLabelValues(status).Inc()
				}
				duration := *latencyMinMs + rand.Intn(*latencyMaxMs) + *latencyOffsetMs
				requestProcessMs.WithLabelValues(status).Observe(float64(duration))
			}
			mutex.RUnlock()
		}
	}()
}
