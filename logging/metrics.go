package logging

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// MetricType represents the type of metric
type MetricType string

const (
	CounterType   MetricType = "counter"
	GaugeType     MetricType = "gauge"
	HistogramType MetricType = "histogram"
)

// Metric represents a metric
type Metric interface {
	Type() MetricType
	Name() string
	Value() interface{}
	Labels() map[string]string
}

// Counter represents a counter metric
type Counter struct {
	name   string
	labels map[string]string
	value  int64
}

// NewCounter creates a new counter
func NewCounter(name string, labels map[string]string) *Counter {
	if labels == nil {
		labels = make(map[string]string)
	}
	return &Counter{
		name:   name,
		labels: labels,
	}
}

func (c *Counter) Type() MetricType                { return CounterType }
func (c *Counter) Name() string                    { return c.name }
func (c *Counter) Value() interface{}              { return atomic.LoadInt64(&c.value) }
func (c *Counter) Labels() map[string]string       { return c.labels }
func (c *Counter) Inc()                            { atomic.AddInt64(&c.value, 1) }
func (c *Counter) Add(delta int64)                 { atomic.AddInt64(&c.value, delta) }
func (c *Counter) Get() int64                      { return atomic.LoadInt64(&c.value) }

// Gauge represents a gauge metric
type Gauge struct {
	name   string
	labels map[string]string
	value  int64
}

// NewGauge creates a new gauge
func NewGauge(name string, labels map[string]string) *Gauge {
	if labels == nil {
		labels = make(map[string]string)
	}
	return &Gauge{
		name:   name,
		labels: labels,
	}
}

func (g *Gauge) Type() MetricType          { return GaugeType }
func (g *Gauge) Name() string              { return g.name }
func (g *Gauge) Value() interface{}        { return atomic.LoadInt64(&g.value) }
func (g *Gauge) Labels() map[string]string { return g.labels }
func (g *Gauge) Set(value int64)           { atomic.StoreInt64(&g.value, value) }
func (g *Gauge) Inc()                      { atomic.AddInt64(&g.value, 1) }
func (g *Gauge) Dec()                      { atomic.AddInt64(&g.value, -1) }
func (g *Gauge) Add(delta int64)           { atomic.AddInt64(&g.value, delta) }
func (g *Gauge) Sub(delta int64)           { atomic.AddInt64(&g.value, -delta) }
func (g *Gauge) Get() int64                { return atomic.LoadInt64(&g.value) }

// Histogram represents a histogram metric
type Histogram struct {
	name    string
	labels  map[string]string
	buckets map[float64]*Counter
	sum     int64
	count   int64
	mu      sync.RWMutex
}

// NewHistogram creates a new histogram with default buckets
func NewHistogram(name string, labels map[string]string) *Histogram {
	return NewHistogramWithBuckets(name, labels, []float64{
		0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
	})
}

// NewHistogramWithBuckets creates a new histogram with custom buckets
func NewHistogramWithBuckets(name string, labels map[string]string, buckets []float64) *Histogram {
	if labels == nil {
		labels = make(map[string]string)
	}

	h := &Histogram{
		name:    name,
		labels:  labels,
		buckets: make(map[float64]*Counter),
	}

	for _, bucket := range buckets {
		bucketLabels := make(map[string]string)
		for k, v := range labels {
			bucketLabels[k] = v
		}
		h.buckets[bucket] = NewCounter(name+"_bucket", bucketLabels)
	}

	return h
}

func (h *Histogram) Type() MetricType          { return HistogramType }
func (h *Histogram) Name() string              { return h.name }
func (h *Histogram) Labels() map[string]string { return h.labels }

func (h *Histogram) Value() interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	buckets := make(map[string]int64)
	for bucket, counter := range h.buckets {
		buckets[formatFloat(bucket)] = counter.Get()
	}

	return map[string]interface{}{
		"count":   atomic.LoadInt64(&h.count),
		"sum":     atomic.LoadInt64(&h.sum),
		"buckets": buckets,
	}
}

// Observe records an observation
func (h *Histogram) Observe(value float64) {
	atomic.AddInt64(&h.count, 1)
	atomic.AddInt64(&h.sum, int64(value*1000)) // Store as milliseconds

	h.mu.RLock()
	for bucket, counter := range h.buckets {
		if value <= bucket {
			counter.Inc()
		}
	}
	h.mu.RUnlock()
}

// formatFloat formats a float64 as a string
func formatFloat(f float64) string {
	return fmt.Sprintf("%.3f", f)
}

// MetricRegistry manages metrics
type MetricRegistry struct {
	metrics map[string]Metric
	mu      sync.RWMutex
}

// NewMetricRegistry creates a new metric registry
func NewMetricRegistry() *MetricRegistry {
	return &MetricRegistry{
		metrics: make(map[string]Metric),
	}
}

// Register registers a metric
func (r *MetricRegistry) Register(metric Metric) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metrics[metric.Name()] = metric
}

// Get retrieves a metric by name
func (r *MetricRegistry) Get(name string) (Metric, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	metric, ok := r.metrics[name]
	return metric, ok
}

// GetAll returns all metrics
func (r *MetricRegistry) GetAll() map[string]Metric {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]Metric)
	for name, metric := range r.metrics {
		result[name] = metric
	}
	return result
}

// Unregister removes a metric
func (r *MetricRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.metrics, name)
}

// Default registry
var defaultRegistry = NewMetricRegistry()

// GetDefaultRegistry returns the default metric registry
func GetDefaultRegistry() *MetricRegistry {
	return defaultRegistry
}

// ApplicationMetrics holds commonly used application metrics
type ApplicationMetrics struct {
	RequestsTotal       *Counter
	RequestDuration     *Histogram
	ResponseStatusCodes *Counter
	ActiveConnections   *Gauge
	ErrorsTotal         *Counter
}

// NewApplicationMetrics creates application metrics
func NewApplicationMetrics() *ApplicationMetrics {
	return &ApplicationMetrics{
		RequestsTotal:       NewCounter("http_requests_total", nil),
		RequestDuration:     NewHistogram("http_request_duration_seconds", nil),
		ResponseStatusCodes: NewCounter("http_response_status_codes", nil),
		ActiveConnections:   NewGauge("http_active_connections", nil),
		ErrorsTotal:         NewCounter("application_errors_total", nil),
	}
}

// Register registers all application metrics
func (am *ApplicationMetrics) Register(registry *MetricRegistry) {
	registry.Register(am.RequestsTotal)
	registry.Register(am.RequestDuration)
	registry.Register(am.ResponseStatusCodes)
	registry.Register(am.ActiveConnections)
	registry.Register(am.ErrorsTotal)
}

// HealthStatus represents the health status of the application
type HealthStatus struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version,omitempty"`
	Uptime    string                 `json:"uptime"`
	Checks    map[string]HealthCheck `json:"checks,omitempty"`
}

// HealthCheck represents an individual health check
type HealthCheck struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Details interface{} `json:"details,omitempty"`
}

// HealthChecker function type for health checks
type HealthChecker func() HealthCheck

// HealthMonitor manages health checks
type HealthMonitor struct {
	checkers  map[string]HealthChecker
	startTime time.Time
	version   string
	mu        sync.RWMutex
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(version string) *HealthMonitor {
	return &HealthMonitor{
		checkers:  make(map[string]HealthChecker),
		startTime: time.Now(),
		version:   version,
	}
}

// AddCheck adds a health check
func (hm *HealthMonitor) AddCheck(name string, checker HealthChecker) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.checkers[name] = checker
}

// RemoveCheck removes a health check
func (hm *HealthMonitor) RemoveCheck(name string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	delete(hm.checkers, name)
}

// CheckHealth performs all health checks
func (hm *HealthMonitor) CheckHealth() HealthStatus {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	status := HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
		Version:   hm.version,
		Uptime:    time.Since(hm.startTime).String(),
		Checks:    make(map[string]HealthCheck),
	}

	for name, checker := range hm.checkers {
		check := checker()
		status.Checks[name] = check

		// If any check fails, mark overall status as unhealthy
		if check.Status != "healthy" {
			status.Status = "unhealthy"
		}
	}

	return status
}

// Default health monitor
var defaultHealthMonitor = NewHealthMonitor("1.0.0")

// GetDefaultHealthMonitor returns the default health monitor
func GetDefaultHealthMonitor() *HealthMonitor {
	return defaultHealthMonitor
}

// MetricsHandler returns an HTTP handler for metrics endpoint
func MetricsHandler(registry *MetricRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		metrics := registry.GetAll()
		result := make(map[string]interface{})

		for name, metric := range metrics {
			result[name] = map[string]interface{}{
				"type":   string(metric.Type()),
				"value":  metric.Value(),
				"labels": metric.Labels(),
			}
		}

		// Add runtime metrics
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		result["runtime"] = map[string]interface{}{
			"goroutines":     runtime.NumGoroutine(),
			"memory_alloc":   m.Alloc,
			"memory_total":   m.TotalAlloc,
			"memory_sys":     m.Sys,
			"gc_runs":        m.NumGC,
			"gc_pause_total": m.PauseTotalNs,
		}

		json.NewEncoder(w).Encode(result)
	}
}

// HealthHandler returns an HTTP handler for health endpoint
func HealthHandler(monitor *HealthMonitor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := monitor.CheckHealth()

		w.Header().Set("Content-Type", "application/json")

		if status.Status == "healthy" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		json.NewEncoder(w).Encode(status)
	}
}

// ReadinessHandler returns an HTTP handler for readiness endpoint
func ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := map[string]interface{}{
			"status":    "ready",
			"timestamp": time.Now().UTC(),
		}

		json.NewEncoder(w).Encode(response)
	}
}

// LivenessHandler returns an HTTP handler for liveness endpoint
func LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := map[string]interface{}{
			"status":    "alive",
			"timestamp": time.Now().UTC(),
		}

		json.NewEncoder(w).Encode(response)
	}
}

// Common health checkers

// DatabaseHealthChecker creates a health checker for database connectivity
func DatabaseHealthChecker(pingFunc func() error) HealthChecker {
	return func() HealthCheck {
		if err := pingFunc(); err != nil {
			return HealthCheck{
				Status:  "unhealthy",
				Message: "Database connection failed",
				Details: map[string]interface{}{
					"error": err.Error(),
				},
			}
		}

		return HealthCheck{
			Status:  "healthy",
			Message: "Database connection successful",
		}
	}
}

// RedisHealthChecker creates a health checker for Redis connectivity
func RedisHealthChecker(pingFunc func() error) HealthChecker {
	return func() HealthCheck {
		if err := pingFunc(); err != nil {
			return HealthCheck{
				Status:  "unhealthy",
				Message: "Redis connection failed",
				Details: map[string]interface{}{
					"error": err.Error(),
				},
			}
		}

		return HealthCheck{
			Status:  "healthy",
			Message: "Redis connection successful",
		}
	}
}

// FileSystemHealthChecker creates a health checker for file system access
func FileSystemHealthChecker(path string) HealthChecker {
	return func() HealthCheck {
		if _, err := os.Stat(path); err != nil {
			return HealthCheck{
				Status:  "unhealthy",
				Message: "File system check failed",
				Details: map[string]interface{}{
					"path":  path,
					"error": err.Error(),
				},
			}
		}

		return HealthCheck{
			Status:  "healthy",
			Message: "File system accessible",
			Details: map[string]interface{}{
				"path": path,
			},
		}
	}
}