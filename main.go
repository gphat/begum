package main

import (
	"encoding/json"
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

type Instance struct {
	ErrorPct        int `json:"error_pct"`
	LatencyMinMs    int `json:"latency_min_ms"`
	LatencyMaxMs    int `json:"latency_max_ms"`
	LatencyOffsetMs int `json:"latency_offset_ms"`
}

var (
	instances = map[string]Instance{}

	interval        = kingpin.Arg("interval", "Interval to update metrics").Default("1s").Duration()
	address         = kingpin.Arg("address", "Address on which to listen for API calls").Default("127.0.0.1:8080").TCP()
	instanceCount   = kingpin.Arg("instance_count", "Number of instances").Default("1").Int()
	requestRate     = kingpin.Arg("request_rate", "Rate of requests to simulate, in requests per second").Default("100").Int()
	errorPct        = kingpin.Arg("error_pct", "Percentage of errors to simulate, as percentage").Default("1").Int()
	latencyMinMs    = kingpin.Arg("latency_min_ms", "Minimum latency value in milliseconds").Default("100").Int()
	latencyMaxMs    = kingpin.Arg("latency_max_ms", "Minimum latency value in milliseconds").Default("300").Int()
	latencyOffsetMs = kingpin.Arg("latency_offset_ms", "Offset to add to latency, simulating slowness").Default("0").Int()

	requestProcessMs = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Name:       "request_duration_millis",
		Help:       "The duration of requuests",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"code", "instance"})
	errorsEncountered = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "errors_encountered_total",
		Help: "The total number of errors encountered",
	}, []string{"code", "instance"})

	mutex = &sync.RWMutex{}
)

func addInstanceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		http.Error(w, "Please send a request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	name := r.URL.Path[len("/instance/"):]
	if name == "" {
		http.Error(w, "Missing name", http.StatusBadRequest)
	}
	mutex.Lock()
	defer mutex.Unlock()
	if _, ok := instances[name]; ok {
		http.Error(w, "Existing instance", http.StatusUnauthorized)
		return
	}

	var instance Instance
	err := json.NewDecoder(r.Body).Decode(&instance)
	if err != nil {
		log.Println(err)
		http.Error(w, "Malformed instance", http.StatusBadRequest)
		return
	}

	log.Printf("Adding instance '%s': %v", name, instance)
	instances[name] = instance
}

func updateInstanceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		http.Error(w, "Please send a request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	name := r.URL.Path[len("/instance/"):]
	mutex.Lock()
	defer mutex.Unlock()
	if _, ok := instances[name]; !ok {
		http.Error(w, "Unknown instance", http.StatusNotFound)
		return
	}
	var instance Instance
	err := json.NewDecoder(r.Body).Decode(&instance)
	if err != nil {
		http.Error(w, "Malformed instance", http.StatusBadRequest)
	}
	log.Printf("Updating instance '%s'", name)
	instances[name] = instance
}

func deleteInstanceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		http.Error(w, "Please send a request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	name := r.URL.Path[len("/instance/"):]
	mutex.Lock()
	defer mutex.Unlock()
	if _, ok := instances[name]; !ok {
		http.Error(w, "Unknown instance", http.StatusNotFound)
		return
	}
	log.Printf("Deleting instance '%s'", name)
	delete(instances, name)
}

func handleInstance(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "DELETE":
		deleteInstanceHandler(w, r)
	case "POST":
		addInstanceHandler(w, r)
	case "PUT":
		updateInstanceHandler(w, r)
	default:
		http.Error(w, "Invalid operation", http.StatusBadRequest)
	}
}

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
	kingpin.Parse()
	log.Printf("Instance count is %d", *instanceCount)
	for i := 0; i < *instanceCount; i++ {
		name := string(rune(65 + i))
		log.Printf("Made instances %s", name)
		instances[name] = Instance{
			ErrorPct:        *errorPct,
			LatencyMinMs:    *latencyMinMs,
			LatencyMaxMs:    *latencyMaxMs,
			LatencyOffsetMs: *latencyOffsetMs,
		}
	}
	log.Printf("Made %d instances", len(instances))

	rand.Seed(time.Now().UnixNano())

	http.HandleFunc("/", handler)
	http.HandleFunc("/error_rate/", errorRateHandler)
	http.HandleFunc("/latency_offset/", latencyHandler)
	http.HandleFunc("/instance/", handleInstance)
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

			perInstanceRate := int(*requestRate / len(instances))

			fmt.Println("Tick at", t)
			for name, instance := range instances {
				log.Printf("Instance '%s', %v", name, instance)
				for i := 0; i < perInstanceRate; i++ {
					var status = "200"
					if rand.Intn(100) <= instance.ErrorPct {
						status = "500"
						errorsEncountered.WithLabelValues(status, name).Inc()
					}
					duration := instance.LatencyMinMs + rand.Intn(instance.LatencyMaxMs) + instance.LatencyOffsetMs
					requestProcessMs.WithLabelValues(status, name).Observe(float64(duration))
				}
			}
			mutex.RUnlock()
		}
	}()
}
