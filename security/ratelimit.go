package security

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	RequestsPerMinute int           // Number of requests per minute
	BurstSize         int           // Burst allowance
	WindowSize        time.Duration // Time window for rate limiting
	SkipSuccessful    bool          // Only count failed requests (4xx, 5xx)
	SkipPaths         []string      // Paths to skip rate limiting
}

// DefaultRateLimitConfig returns sensible defaults
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         10,
		WindowSize:        time.Minute,
		SkipSuccessful:    false,
		SkipPaths:         []string{"/health", "/metrics", "/health/ready", "/health/live"},
	}
}

// RateLimiter implements token bucket algorithm for rate limiting
type RateLimiter struct {
	config  RateLimitConfig
	clients map[string]*clientBucket
	mu      sync.RWMutex
}

// clientBucket represents rate limiting state for a single client
type clientBucket struct {
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		config:  config,
		clients: make(map[string]*clientBucket),
	}
	
	// Start cleanup goroutine
	go rl.cleanup()
	
	return rl
}

// Allow checks if a request should be allowed
func (rl *RateLimiter) Allow(clientIP string) bool {
	rl.mu.RLock()
	bucket, exists := rl.clients[clientIP]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		// Double-check after acquiring write lock
		if bucket, exists = rl.clients[clientIP]; !exists {
			bucket = &clientBucket{
				tokens:     float64(rl.config.BurstSize),
				lastUpdate: time.Now(),
			}
			rl.clients[clientIP] = bucket
		}
		rl.mu.Unlock()
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	now := time.Now()
	timePassed := now.Sub(bucket.lastUpdate)
	bucket.lastUpdate = now

	// Add tokens based on time passed
	tokensToAdd := timePassed.Seconds() * (float64(rl.config.RequestsPerMinute) / 60.0)
	bucket.tokens += tokensToAdd

	// Cap at burst size
	if bucket.tokens > float64(rl.config.BurstSize) {
		bucket.tokens = float64(rl.config.BurstSize)
	}

	// Check if request can be allowed
	if bucket.tokens >= 1.0 {
		bucket.tokens -= 1.0
		return true
	}

	return false
}

// cleanup removes old client entries
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for clientIP, bucket := range rl.clients {
			bucket.mu.Lock()
			if now.Sub(bucket.lastUpdate) > 10*time.Minute {
				delete(rl.clients, clientIP)
			}
			bucket.mu.Unlock()
		}
		rl.mu.Unlock()
	}
}

// RateLimitMiddleware creates middleware for rate limiting
func RateLimitMiddleware(config RateLimitConfig) func(next http.Handler) http.Handler {
	limiter := NewRateLimiter(config)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip rate limiting for certain paths
			for _, skipPath := range config.SkipPaths {
				if r.URL.Path == skipPath {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Get client IP (prefer X-Forwarded-For, fallback to RemoteAddr)
			clientIP := r.Header.Get("X-Forwarded-For")
			if clientIP == "" {
				clientIP = r.Header.Get("X-Real-IP")
			}
			if clientIP == "" {
				clientIP = r.RemoteAddr
			}

			// Handle case where we only count failed requests
			if config.SkipSuccessful {
				// Wrap response writer to capture status code
				ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
				next.ServeHTTP(ww, r)

				// Only apply rate limiting if response was an error
				if ww.Status() >= 400 {
					if !limiter.Allow(clientIP) {
						http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
						return
					}
				}
				return
			}

			// Standard rate limiting - check before processing request
			if !limiter.Allow(clientIP) {
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(config.RequestsPerMinute))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("Retry-After", "60")
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// Add rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(config.RequestsPerMinute))

			next.ServeHTTP(w, r)
		})
	}
}

// AdaptiveRateLimiter implements adaptive rate limiting based on response patterns
type AdaptiveRateLimiter struct {
	baseLimiter *RateLimiter
	config      RateLimitConfig
	stats       map[string]*clientStats
	mu          sync.RWMutex
}

// clientStats tracks client behavior patterns
type clientStats struct {
	totalRequests   int64
	failedRequests  int64
	lastFailureTime time.Time
	currentLimit    int
	mu              sync.Mutex
}

// NewAdaptiveRateLimiter creates an adaptive rate limiter
func NewAdaptiveRateLimiter(baseConfig RateLimitConfig) *AdaptiveRateLimiter {
	return &AdaptiveRateLimiter{
		baseLimiter: NewRateLimiter(baseConfig),
		config:      baseConfig,
		stats:       make(map[string]*clientStats),
	}
}

// AdaptiveRateLimitMiddleware creates adaptive rate limiting middleware
func AdaptiveRateLimitMiddleware(config RateLimitConfig) func(next http.Handler) http.Handler {
	limiter := NewAdaptiveRateLimiter(config)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip rate limiting for certain paths
			for _, skipPath := range config.SkipPaths {
				if r.URL.Path == skipPath {
					next.ServeHTTP(w, r)
					return
				}
			}

			clientIP := getClientIP(r)
			
			// Get or create client stats
			limiter.mu.RLock()
			stats, exists := limiter.stats[clientIP]
			limiter.mu.RUnlock()

			if !exists {
				limiter.mu.Lock()
				if stats, exists = limiter.stats[clientIP]; !exists {
					stats = &clientStats{
						currentLimit: config.RequestsPerMinute,
					}
					limiter.stats[clientIP] = stats
				}
				limiter.mu.Unlock()
			}

			// Check if request should be allowed
			if !limiter.baseLimiter.Allow(clientIP) {
				stats.mu.Lock()
				stats.failedRequests++
				stats.lastFailureTime = time.Now()
				
				// Reduce limit for problematic clients
				if stats.failedRequests > 5 {
					stats.currentLimit = config.RequestsPerMinute / 2
				}
				stats.mu.Unlock()

				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(stats.currentLimit))
				w.Header().Set("X-RateLimit-Remaining", "0")
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// Wrap response writer to track success/failure
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			// Update client stats
			stats.mu.Lock()
			stats.totalRequests++
			if ww.Status() >= 400 {
				stats.failedRequests++
				stats.lastFailureTime = time.Now()
			}

			// Adjust rate limit based on behavior
			failureRate := float64(stats.failedRequests) / float64(stats.totalRequests)
			if failureRate > 0.5 {
				// High failure rate - reduce limit
				stats.currentLimit = config.RequestsPerMinute / 2
			} else if failureRate < 0.1 && time.Since(stats.lastFailureTime) > 5*time.Minute {
				// Low failure rate and no recent failures - increase limit
				stats.currentLimit = config.RequestsPerMinute
			}
			stats.mu.Unlock()

			// Add rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(stats.currentLimit))
		})
	}
}

// getClientIP extracts client IP from request (secure version)
func getClientIP(r *http.Request) string {
	// For security, only use RemoteAddr unless trusted proxies are configured
	// This prevents IP spoofing attacks via headers
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx != -1 {
		return r.RemoteAddr[:idx]
	}
	return r.RemoteAddr
}

// getClientIPWithTrustedProxies extracts client IP with proxy validation
func getClientIPWithTrustedProxies(r *http.Request, trustedProxies []string) string {
	// If no trusted proxies defined, only use RemoteAddr for security
	if len(trustedProxies) == 0 {
		return getClientIP(r)
	}
	
	// Check if immediate connection is from trusted proxy
	immediateIP := getClientIP(r)
	trusted := false
	for _, proxy := range trustedProxies {
		if immediateIP == proxy {
			trusted = true
			break
		}
	}
	
	if !trusted {
		return immediateIP
	}
	
	// Only trust headers if from verified proxy
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx != -1 {
			ip := strings.TrimSpace(xff[:idx])
			if ip != "" && !isPrivateIP(ip) {
				return ip
			}
		} else if xff != "" && !isPrivateIP(xff) {
			return strings.TrimSpace(xff)
		}
	}
	
	if xri := r.Header.Get("X-Real-IP"); xri != "" && !isPrivateIP(xri) {
		return strings.TrimSpace(xri)
	}
	
	return immediateIP
}

// isPrivateIP checks if IP is in private ranges (basic check)
func isPrivateIP(ip string) bool {
	return strings.HasPrefix(ip, "127.") ||
		strings.HasPrefix(ip, "10.") ||
		strings.HasPrefix(ip, "192.168.") ||
		strings.HasPrefix(ip, "172.16.") ||
		ip == "localhost"
}

// APIRateLimitMiddleware provides stricter rate limiting for API endpoints
func APIRateLimitMiddleware(requestsPerMinute int) func(next http.Handler) http.Handler {
	burstSize := requestsPerMinute / 6 // Allow burst of ~10 seconds worth
	if burstSize < 2 {
		burstSize = 2 // Minimum burst size
	}
	
	config := RateLimitConfig{
		RequestsPerMinute: requestsPerMinute,
		BurstSize:         burstSize,
		WindowSize:        time.Minute,
		SkipSuccessful:    false,
		SkipPaths:         []string{}, // Don't skip any paths for API
	}

	return RateLimitMiddleware(config)
}

// GlobalRateLimitMiddleware provides application-wide rate limiting
func GlobalRateLimitMiddleware() func(next http.Handler) http.Handler {
	config := DefaultRateLimitConfig()
	return RateLimitMiddleware(config)
}