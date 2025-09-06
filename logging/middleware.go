package logging

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// MetricsMiddleware creates middleware that automatically tracks HTTP metrics
func MetricsMiddleware(metrics *ApplicationMetrics, logger *Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Track active connections
			metrics.ActiveConnections.Inc()
			defer metrics.ActiveConnections.Dec()

			// Create response writer wrapper to capture status code
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Process request
			next.ServeHTTP(ww, r)

			// Calculate metrics
			duration := time.Since(start)
			statusCode := ww.Status()

			// Update metrics
			metrics.RequestsTotal.Inc()
			metrics.RequestDuration.Observe(duration.Seconds())

			// Track status codes - update the existing counter
			if statusCode == 200 {
				metrics.ResponseStatusCodes.Add(1)
			}

			// Track errors (4xx and 5xx status codes)
			if statusCode >= 400 {
				metrics.ErrorsTotal.Inc()
			}

			// Log request with structured data
			requestFields := map[string]interface{}{
				"method":     r.Method,
				"path":       r.URL.Path,
				"status":     statusCode,
				"duration":   duration.Milliseconds(),
				"user_agent": r.UserAgent(),
				"remote_ip":  r.RemoteAddr,
			}

			// Add request ID if available
			if reqID := middleware.GetReqID(r.Context()); reqID != "" {
				requestFields["request_id"] = reqID
			}

			// Log based on status code
			if statusCode >= 500 {
				logger.Error("HTTP request completed with server error", requestFields)
			} else if statusCode >= 400 {
				logger.Warn("HTTP request completed with client error", requestFields)
			} else {
				logger.Info("HTTP request completed", requestFields)
			}
		})
	}
}

// StructuredLoggingMiddleware creates middleware that adds structured logger to request context
func StructuredLoggingMiddleware(logger *Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get request ID from chi middleware
			requestID := middleware.GetReqID(r.Context())
			
			// Create logger with request context
			contextLogger := logger.WithRequestID(requestID)
			
			// Add logger to request context
			ctx := ToContext(r.Context(), contextLogger)
			
			// Continue with updated context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RecoveryMiddleware creates structured logging recovery middleware
func RecoveryMiddleware(logger *Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					// Log panic with request context, fallback to provided logger if context doesn't have one
					contextLogger := FromContext(r.Context())
					if contextLogger == GetDefaultLogger() && logger != nil {
						contextLogger = logger
					}
					
					contextLogger.Error("Panic recovered", map[string]interface{}{
						"panic":      fmt.Sprintf("%v", rvr),
						"method":     r.Method,
						"path":       r.URL.Path,
						"user_agent": r.UserAgent(),
						"remote_ip":  r.RemoteAddr,
					})
					
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			
			next.ServeHTTP(w, r)
		})
	}
}