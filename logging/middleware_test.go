package logging

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:      InfoLevel,
		Writer:     &buf,
		EnableJSON: true,
	})
	
	metrics := NewApplicationMetrics()
	
	// Create middleware
	middlewareFunc := MetricsMiddleware(metrics, logger)
	
	// Create test handler
	handler := middlewareFunc(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // Simulate some processing time
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	// Initial metrics should be zero
	assert.Equal(t, int64(0), metrics.RequestsTotal.Get())
	assert.Equal(t, int64(0), metrics.ActiveConnections.Get())
	assert.Equal(t, int64(0), metrics.ErrorsTotal.Get())
	
	// Make request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	// Check metrics were updated
	assert.Equal(t, int64(1), metrics.RequestsTotal.Get())
	assert.Equal(t, int64(0), metrics.ActiveConnections.Get()) // Should be back to 0 after request
	assert.Equal(t, int64(0), metrics.ErrorsTotal.Get()) // No errors
	
	// Check that duration was recorded
	durationValue := metrics.RequestDuration.Value().(map[string]interface{})
	assert.Equal(t, int64(1), durationValue["count"])
	assert.True(t, durationValue["sum"].(int64) > 0)
	
	// Check log output
	output := buf.String()
	var logEntry LogEntry
	err := json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)
	
	assert.Equal(t, "INFO", logEntry.Level)
	assert.Equal(t, "HTTP request completed", logEntry.Message)
	assert.Equal(t, "GET", logEntry.Fields["method"])
	assert.Equal(t, "/test", logEntry.Fields["path"])
	assert.Equal(t, float64(200), logEntry.Fields["status"])
	assert.True(t, logEntry.Fields["duration"].(float64) > 0)
}

func TestMetricsMiddlewareWithErrors(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:      InfoLevel,
		Writer:     &buf,
		EnableJSON: true,
	})
	
	metrics := NewApplicationMetrics()
	middlewareFunc := MetricsMiddleware(metrics, logger)
	
	tests := []struct {
		name           string
		statusCode     int
		expectedErrors int64
		expectedLevel  string
	}{
		{"success", http.StatusOK, 0, "INFO"},
		{"client error", http.StatusBadRequest, 1, "WARN"},
		{"server error", http.StatusInternalServerError, 1, "ERROR"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset metrics
			metrics.ErrorsTotal = NewCounter("application_errors_total", nil)
			buf.Reset()
			
			handler := middlewareFunc(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			
			req := httptest.NewRequest("POST", "/api/test", nil)
			w := httptest.NewRecorder()
			
			handler.ServeHTTP(w, req)
			
			assert.Equal(t, tt.expectedErrors, metrics.ErrorsTotal.Get())
			
			// Check log level
			output := buf.String()
			var logEntry LogEntry
			err := json.Unmarshal([]byte(output), &logEntry)
			require.NoError(t, err)
			
			assert.Equal(t, tt.expectedLevel, logEntry.Level)
			assert.Equal(t, float64(tt.statusCode), logEntry.Fields["status"])
			assert.Equal(t, "POST", logEntry.Fields["method"])
		})
	}
}

func TestMetricsMiddlewareWithRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:      InfoLevel,
		Writer:     &buf,
		EnableJSON: true,
	})
	
	metrics := NewApplicationMetrics()
	
	// Create router with request ID middleware
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(MetricsMiddleware(metrics, logger))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	
	r.ServeHTTP(w, req)
	
	// Check that request ID was logged
	output := buf.String()
	var logEntry LogEntry
	err := json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)
	
	// The request_id should be in the RequestID field, not Fields
	assert.NotEmpty(t, logEntry.RequestID)
}

func TestStructuredLoggingMiddleware(t *testing.T) {
	logger := NewDefault()
	
	middlewareFunc := StructuredLoggingMiddleware(logger)
	
	var contextLogger *Logger
	handler := middlewareFunc(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contextLogger = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	
	// Create router with request ID middleware to test integration
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middlewareFunc)
	r.Get("/test", handler.ServeHTTP)
	
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	
	r.ServeHTTP(w, req)
	
	// Should have logger in context with request ID
	assert.NotNil(t, contextLogger)
	assert.Contains(t, contextLogger.fields, "request_id")
	assert.NotEmpty(t, contextLogger.fields["request_id"])
}

func TestRecoveryMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:      InfoLevel,
		Writer:     &buf,
		EnableJSON: true,
	})
	
	middlewareFunc := RecoveryMiddleware(logger)
	
	// Handler that panics
	handler := middlewareFunc(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))
	
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	
	// Should not panic
	assert.NotPanics(t, func() {
		handler.ServeHTTP(w, req)
	})
	
	// Should return 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Internal Server Error")
	
	// Should log the panic
	output := buf.String()
	assert.NotEmpty(t, output)
	
	var logEntry LogEntry
	err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry)
	require.NoError(t, err)
	
	assert.Equal(t, "ERROR", logEntry.Level)
	assert.Equal(t, "Panic recovered", logEntry.Message)
	assert.Contains(t, logEntry.Fields["panic"], "test panic")
	assert.Equal(t, "GET", logEntry.Fields["method"])
	assert.Equal(t, "/test", logEntry.Fields["path"])
}

func TestRecoveryMiddlewareWithContext(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:      InfoLevel,
		Writer:     &buf,
		EnableJSON: true,
	})
	
	// Create router with both middlewares
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(StructuredLoggingMiddleware(logger))
	r.Use(RecoveryMiddleware(logger))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		panic("context panic")
	})
	
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	
	r.ServeHTTP(w, req)
	
	// Check that panic was logged with request context
	output := buf.String()
	var logEntry LogEntry
	err := json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)
	
	assert.Equal(t, "ERROR", logEntry.Level)
	assert.Contains(t, logEntry.Fields["panic"], "context panic")
	assert.NotEmpty(t, logEntry.RequestID) // Should have request ID from context
}

func TestMiddlewareIntegration(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:      InfoLevel,
		Writer:     &buf,
		EnableJSON: true,
	})
	
	metrics := NewApplicationMetrics()
	
	// Create router with all middlewares
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(StructuredLoggingMiddleware(logger))
	r.Use(RecoveryMiddleware(logger))
	r.Use(MetricsMiddleware(metrics, logger))
	
	r.Get("/success", func(w http.ResponseWriter, r *http.Request) {
		// Get logger from context and use it
		contextLogger := FromContext(r.Context())
		contextLogger.Info("Handler executed", map[string]interface{}{"endpoint": "/success"})
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})
	
	r.Get("/error", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	})
	
	r.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("integration test panic")
	})
	
	// Test success case
	req := httptest.NewRequest("GET", "/success", nil)
	w := httptest.NewRecorder()
	
	r.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(1), metrics.RequestsTotal.Get())
	assert.Equal(t, int64(0), metrics.ErrorsTotal.Get())
	
	// Should have two log entries: handler log and metrics log
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Len(t, lines, 2)
	
	// Test error case
	buf.Reset()
	req = httptest.NewRequest("GET", "/error", nil)
	w = httptest.NewRecorder()
	
	r.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, int64(2), metrics.RequestsTotal.Get())
	assert.Equal(t, int64(1), metrics.ErrorsTotal.Get())
	
	// Test panic case
	buf.Reset()
	req = httptest.NewRequest("GET", "/panic", nil)
	w = httptest.NewRecorder()
	
	r.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	// Note: Panic happens before metrics are recorded, so counts don't increment
	assert.Equal(t, int64(2), metrics.RequestsTotal.Get()) // Still 2, not 3
	assert.Equal(t, int64(1), metrics.ErrorsTotal.Get())   // Still 1, not 2
	
	// Should have at least one log entry for panic recovery
	output = buf.String()
	lines = strings.Split(strings.TrimSpace(output), "\n")
	assert.True(t, len(lines) >= 1, "Should have at least one log entry")
	
	// Check panic log (should be first entry)
	var panicEntry LogEntry
	err := json.Unmarshal([]byte(lines[0]), &panicEntry)
	require.NoError(t, err)
	assert.Equal(t, "ERROR", panicEntry.Level)
	assert.Equal(t, "Panic recovered", panicEntry.Message)
}

func TestMiddlewareConcurrency(t *testing.T) {
	logger := NewDefault()
	metrics := NewApplicationMetrics()
	
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(StructuredLoggingMiddleware(logger))
	r.Use(MetricsMiddleware(metrics, logger))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Millisecond) // Small delay
		w.WriteHeader(http.StatusOK)
	})
	
	// Make concurrent requests
	const numRequests = 100
	done := make(chan bool, numRequests)
	
	for i := 0; i < numRequests; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			
			assert.Equal(t, http.StatusOK, w.Code)
			done <- true
		}()
	}
	
	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		<-done
	}
	
	// Check that all requests were counted
	assert.Equal(t, int64(numRequests), metrics.RequestsTotal.Get())
	assert.Equal(t, int64(0), metrics.ActiveConnections.Get()) // Should be back to 0
	
	// Check that duration histogram recorded all requests
	durationValue := metrics.RequestDuration.Value().(map[string]interface{})
	assert.Equal(t, int64(numRequests), durationValue["count"])
}

func TestMiddlewareWithNilComponents(t *testing.T) {
	// Test that middleware handles nil components gracefully
	assert.NotPanics(t, func() {
		MetricsMiddleware(nil, nil)
	})
	
	assert.NotPanics(t, func() {
		StructuredLoggingMiddleware(nil)
	})
	
	assert.NotPanics(t, func() {
		RecoveryMiddleware(nil)
	})
}