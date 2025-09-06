package logging

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCounter(t *testing.T) {
	labels := map[string]string{"method": "GET", "status": "200"}
	counter := NewCounter("http_requests", labels)

	assert.Equal(t, CounterType, counter.Type())
	assert.Equal(t, "http_requests", counter.Name())
	assert.Equal(t, labels, counter.Labels())
	assert.Equal(t, int64(0), counter.Get())

	// Test increment
	counter.Inc()
	assert.Equal(t, int64(1), counter.Get())
	assert.Equal(t, int64(1), counter.Value().(int64))

	// Test add
	counter.Add(5)
	assert.Equal(t, int64(6), counter.Get())

	// Test concurrent access
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counter.Inc()
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(106), counter.Get())
}

func TestGauge(t *testing.T) {
	labels := map[string]string{"instance": "server1"}
	gauge := NewGauge("active_connections", labels)

	assert.Equal(t, GaugeType, gauge.Type())
	assert.Equal(t, "active_connections", gauge.Name())
	assert.Equal(t, labels, gauge.Labels())
	assert.Equal(t, int64(0), gauge.Get())

	// Test set
	gauge.Set(10)
	assert.Equal(t, int64(10), gauge.Get())

	// Test increment/decrement
	gauge.Inc()
	assert.Equal(t, int64(11), gauge.Get())
	
	gauge.Dec()
	assert.Equal(t, int64(10), gauge.Get())

	// Test add/subtract
	gauge.Add(5)
	assert.Equal(t, int64(15), gauge.Get())
	
	gauge.Sub(3)
	assert.Equal(t, int64(12), gauge.Get())
}

func TestHistogram(t *testing.T) {
	labels := map[string]string{"endpoint": "/api/users"}
	histogram := NewHistogram("request_duration", labels)

	assert.Equal(t, HistogramType, histogram.Type())
	assert.Equal(t, "request_duration", histogram.Name())
	assert.Equal(t, labels, histogram.Labels())

	// Test observations
	histogram.Observe(0.1)   // 100ms
	histogram.Observe(0.05)  // 50ms
	histogram.Observe(0.2)   // 200ms
	histogram.Observe(1.5)   // 1500ms

	value := histogram.Value().(map[string]interface{})
	
	assert.Equal(t, int64(4), value["count"])
	assert.Equal(t, int64(1850), value["sum"]) // Sum in milliseconds

	buckets := value["buckets"].(map[string]int64)
	
	// Check bucket counts (cumulative)
	assert.Equal(t, int64(0), buckets["0.025"]) // 0 observations <= 0.025
	assert.Equal(t, int64(2), buckets["0.100"]) // 2 observations <= 0.100 (0.05, 0.1)
	assert.Equal(t, int64(3), buckets["0.250"]) // 3 observations <= 0.250 (0.05, 0.1, 0.2)
	assert.Equal(t, int64(4), buckets["2.500"]) // 4 observations <= 2.500 (all)
}

func TestHistogramWithCustomBuckets(t *testing.T) {
	labels := map[string]string{"service": "api"}
	buckets := []float64{0.1, 0.5, 1.0, 2.0}
	histogram := NewHistogramWithBuckets("custom_duration", labels, buckets)

	histogram.Observe(0.3)
	histogram.Observe(1.5)

	value := histogram.Value().(map[string]interface{})
	bucketCounts := value["buckets"].(map[string]int64)

	assert.Equal(t, int64(0), bucketCounts["0.100"]) // 0 <= 0.1
	assert.Equal(t, int64(1), bucketCounts["0.500"]) // 1 <= 0.5 (0.3)
	assert.Equal(t, int64(1), bucketCounts["1.000"]) // 1 <= 1.0 (0.3)
	assert.Equal(t, int64(2), bucketCounts["2.000"]) // 2 <= 2.0 (0.3, 1.5)
}

func TestMetricRegistry(t *testing.T) {
	registry := NewMetricRegistry()
	
	counter := NewCounter("test_counter", nil)
	gauge := NewGauge("test_gauge", nil)

	// Test registration
	registry.Register(counter)
	registry.Register(gauge)

	// Test retrieval
	retrievedCounter, ok := registry.Get("test_counter")
	assert.True(t, ok)
	assert.Equal(t, counter, retrievedCounter)

	retrievedGauge, ok := registry.Get("test_gauge")
	assert.True(t, ok)
	assert.Equal(t, gauge, retrievedGauge)

	// Test non-existent metric
	_, ok = registry.Get("non_existent")
	assert.False(t, ok)

	// Test get all
	all := registry.GetAll()
	assert.Len(t, all, 2)
	assert.Contains(t, all, "test_counter")
	assert.Contains(t, all, "test_gauge")

	// Test unregister
	registry.Unregister("test_counter")
	_, ok = registry.Get("test_counter")
	assert.False(t, ok)

	all = registry.GetAll()
	assert.Len(t, all, 1)
}

func TestDefaultRegistry(t *testing.T) {
	registry := GetDefaultRegistry()
	assert.NotNil(t, registry)
	
	// Should return the same instance
	registry2 := GetDefaultRegistry()
	assert.Equal(t, registry, registry2)
}

func TestApplicationMetrics(t *testing.T) {
	appMetrics := NewApplicationMetrics()
	
	assert.NotNil(t, appMetrics.RequestsTotal)
	assert.NotNil(t, appMetrics.RequestDuration)
	assert.NotNil(t, appMetrics.ResponseStatusCodes)
	assert.NotNil(t, appMetrics.ActiveConnections)
	assert.NotNil(t, appMetrics.ErrorsTotal)

	// Test registration
	registry := NewMetricRegistry()
	appMetrics.Register(registry)

	_, ok := registry.Get("http_requests_total")
	assert.True(t, ok)
	
	_, ok = registry.Get("http_request_duration_seconds")
	assert.True(t, ok)
	
	_, ok = registry.Get("http_response_status_codes")
	assert.True(t, ok)
	
	_, ok = registry.Get("http_active_connections")
	assert.True(t, ok)
	
	_, ok = registry.Get("application_errors_total")
	assert.True(t, ok)
}

func TestHealthStatus(t *testing.T) {
	monitor := NewHealthMonitor("1.0.0")
	
	// Test healthy status with no checks
	status := monitor.CheckHealth()
	assert.Equal(t, "healthy", status.Status)
	assert.Equal(t, "1.0.0", status.Version)
	assert.NotZero(t, status.Timestamp)
	assert.NotEmpty(t, status.Uptime)
	assert.Empty(t, status.Checks)

	// Add healthy check
	monitor.AddCheck("test", func() HealthCheck {
		return HealthCheck{
			Status:  "healthy",
			Message: "All good",
		}
	})

	status = monitor.CheckHealth()
	assert.Equal(t, "healthy", status.Status)
	assert.Len(t, status.Checks, 1)
	assert.Equal(t, "healthy", status.Checks["test"].Status)
	assert.Equal(t, "All good", status.Checks["test"].Message)

	// Add unhealthy check
	monitor.AddCheck("failing", func() HealthCheck {
		return HealthCheck{
			Status:  "unhealthy",
			Message: "Something is wrong",
			Details: map[string]interface{}{"error": "connection failed"},
		}
	})

	status = monitor.CheckHealth()
	assert.Equal(t, "unhealthy", status.Status)
	assert.Len(t, status.Checks, 2)
	assert.Equal(t, "unhealthy", status.Checks["failing"].Status)
	
	// Remove check
	monitor.RemoveCheck("failing")
	status = monitor.CheckHealth()
	assert.Equal(t, "healthy", status.Status)
	assert.Len(t, status.Checks, 1)
}

func TestDefaultHealthMonitor(t *testing.T) {
	monitor := GetDefaultHealthMonitor()
	assert.NotNil(t, monitor)
	
	// Should return the same instance
	monitor2 := GetDefaultHealthMonitor()
	assert.Equal(t, monitor, monitor2)
}

func TestDatabaseHealthChecker(t *testing.T) {
	// Test successful ping
	healthyChecker := DatabaseHealthChecker(func() error {
		return nil
	})
	
	check := healthyChecker()
	assert.Equal(t, "healthy", check.Status)
	assert.Equal(t, "Database connection successful", check.Message)

	// Test failed ping
	unhealthyChecker := DatabaseHealthChecker(func() error {
		return assert.AnError
	})
	
	check = unhealthyChecker()
	assert.Equal(t, "unhealthy", check.Status)
	assert.Equal(t, "Database connection failed", check.Message)
	assert.NotNil(t, check.Details)
}

func TestRedisHealthChecker(t *testing.T) {
	// Test successful ping
	healthyChecker := RedisHealthChecker(func() error {
		return nil
	})
	
	check := healthyChecker()
	assert.Equal(t, "healthy", check.Status)
	assert.Equal(t, "Redis connection successful", check.Message)

	// Test failed ping
	unhealthyChecker := RedisHealthChecker(func() error {
		return assert.AnError
	})
	
	check = unhealthyChecker()
	assert.Equal(t, "unhealthy", check.Status)
	assert.Equal(t, "Redis connection failed", check.Message)
}

func TestMetricsHandler(t *testing.T) {
	registry := NewMetricRegistry()
	counter := NewCounter("test_requests", map[string]string{"method": "GET"})
	gauge := NewGauge("active_users", nil)
	
	counter.Add(10)
	gauge.Set(5)
	
	registry.Register(counter)
	registry.Register(gauge)

	handler := MetricsHandler(registry)
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var result map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)

	// Check metrics
	assert.Contains(t, result, "test_requests")
	assert.Contains(t, result, "active_users")
	assert.Contains(t, result, "runtime")

	// Check runtime metrics
	runtime := result["runtime"].(map[string]interface{})
	assert.Contains(t, runtime, "goroutines")
	assert.Contains(t, runtime, "memory_alloc")
	assert.Contains(t, runtime, "gc_runs")
}

func TestHealthHandler(t *testing.T) {
	monitor := NewHealthMonitor("1.0.0")
	monitor.AddCheck("test", func() HealthCheck {
		return HealthCheck{Status: "healthy", Message: "OK"}
	})

	handler := HealthHandler(monitor)
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var status HealthStatus
	err := json.Unmarshal(w.Body.Bytes(), &status)
	require.NoError(t, err)

	assert.Equal(t, "healthy", status.Status)
	assert.Equal(t, "1.0.0", status.Version)

	// Test unhealthy response
	monitor.AddCheck("failing", func() HealthCheck {
		return HealthCheck{Status: "unhealthy", Message: "Failed"}
	})

	w = httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestReadinessHandler(t *testing.T) {
	handler := ReadinessHandler()
	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "ready", response["status"])
	assert.Contains(t, response, "timestamp")
}

func TestLivenessHandler(t *testing.T) {
	handler := LivenessHandler()
	req := httptest.NewRequest("GET", "/live", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "alive", response["status"])
	assert.Contains(t, response, "timestamp")
}

func TestConcurrentMetrics(t *testing.T) {
	counter := NewCounter("concurrent_test", nil)
	gauge := NewGauge("concurrent_gauge", nil)
	
	var wg sync.WaitGroup
	
	// Concurrent counter increments
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counter.Inc()
		}()
	}
	
	// Concurrent gauge operations
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			gauge.Inc()
			gauge.Dec()
		}()
	}
	
	wg.Wait()
	
	assert.Equal(t, int64(100), counter.Get())
	assert.Equal(t, int64(0), gauge.Get()) // Should be back to 0
}

func TestHistogramConcurrency(t *testing.T) {
	histogram := NewHistogram("concurrent_histogram", nil)
	
	var wg sync.WaitGroup
	
	// Concurrent observations
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			histogram.Observe(float64(val) / 1000.0) // Values 0.000 to 0.999
		}(i)
	}
	
	wg.Wait()
	
	value := histogram.Value().(map[string]interface{})
	assert.Equal(t, int64(1000), value["count"])
	
	// Sum should be approximately 499.5 (sum of 0 to 999 / 1000)
	sum := value["sum"].(int64)
	assert.InDelta(t, 499500, sum, 100) // Allow some tolerance
}