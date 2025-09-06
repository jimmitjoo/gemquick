package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RateLimiter interface for different rate limiting strategies
type RateLimiter interface {
	Allow(key string) (bool, *RateLimitInfo)
	Reset(key string)
}

// RateLimitInfo contains rate limit information
type RateLimitInfo struct {
	Limit     int
	Remaining int
	Reset     time.Time
	RetryAfter int // seconds
}

// TokenBucket implements token bucket rate limiting
type TokenBucket struct {
	mu       sync.RWMutex
	buckets  map[string]*bucket
	rate     int           // tokens per interval
	capacity int           // max tokens
	interval time.Duration // refill interval
	cleanup  time.Duration // cleanup interval for old buckets
	stop     chan struct{} // channel to stop cleanup routine
	stopped  bool          // flag to track if stopped
}

type bucket struct {
	tokens    int
	lastRefill time.Time
}

// NewTokenBucket creates a new token bucket rate limiter
func NewTokenBucket(rate, capacity int, interval time.Duration) *TokenBucket {
	tb := &TokenBucket{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		capacity: capacity,
		interval: interval,
		cleanup:  interval * 10, // cleanup buckets older than 10 intervals
		stop:     make(chan struct{}),
		stopped:  false,
	}
	
	// Start cleanup goroutine
	go tb.cleanupRoutine()
	
	return tb
}

// Allow checks if request is allowed
func (tb *TokenBucket) Allow(key string) (bool, *RateLimitInfo) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	
	now := time.Now()
	b, exists := tb.buckets[key]
	
	if !exists {
		// Create new bucket
		b = &bucket{
			tokens:     tb.capacity,
			lastRefill: now,
		}
		tb.buckets[key] = b
	}
	
	// Refill tokens
	elapsed := now.Sub(b.lastRefill)
	tokensToAdd := int(elapsed / tb.interval) * tb.rate
	if tokensToAdd > 0 {
		b.tokens = min(b.tokens+tokensToAdd, tb.capacity)
		b.lastRefill = now
	}
	
	// Check if request is allowed
	allowed := b.tokens > 0
	if allowed {
		b.tokens--
	}
	
	// Calculate reset time
	resetTime := b.lastRefill.Add(tb.interval)
	retryAfter := 0
	if !allowed {
		retryAfter = int(resetTime.Sub(now).Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}
	}
	
	info := &RateLimitInfo{
		Limit:     tb.capacity,
		Remaining: b.tokens,
		Reset:     resetTime,
		RetryAfter: retryAfter,
	}
	
	return allowed, info
}

// Reset resets the bucket for a key
func (tb *TokenBucket) Reset(key string) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	delete(tb.buckets, key)
}

// cleanupRoutine removes old buckets
func (tb *TokenBucket) cleanupRoutine() {
	ticker := time.NewTicker(tb.cleanup)
	defer ticker.Stop()
	
	for {
		select {
		case <-tb.stop:
			return
		case <-ticker.C:
			tb.mu.Lock()
			if tb.stopped {
				tb.mu.Unlock()
				return
			}
			now := time.Now()
			for key, b := range tb.buckets {
				if now.Sub(b.lastRefill) > tb.cleanup {
					delete(tb.buckets, key)
				}
			}
			tb.mu.Unlock()
		}
	}
}

// Stop gracefully stops the token bucket and cleans up resources
func (tb *TokenBucket) Stop() {
	tb.mu.Lock()
	if !tb.stopped {
		tb.stopped = true
		close(tb.stop)
	}
	tb.mu.Unlock()
}

// SlidingWindow implements sliding window rate limiting
type SlidingWindow struct {
	mu       sync.RWMutex
	windows  map[string]*window
	limit    int
	duration time.Duration
	cleanup  time.Duration
	stop     chan struct{} // channel to stop cleanup routine
	stopped  bool          // flag to track if stopped
}

type window struct {
	requests []time.Time
}

// NewSlidingWindow creates a new sliding window rate limiter
func NewSlidingWindow(limit int, duration time.Duration) *SlidingWindow {
	sw := &SlidingWindow{
		windows:  make(map[string]*window),
		limit:    limit,
		duration: duration,
		cleanup:  duration * 2,
		stop:     make(chan struct{}),
		stopped:  false,
	}
	
	// Start cleanup goroutine
	go sw.cleanupRoutine()
	
	return sw
}

// Allow checks if request is allowed
func (sw *SlidingWindow) Allow(key string) (bool, *RateLimitInfo) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	
	now := time.Now()
	w, exists := sw.windows[key]
	
	if !exists {
		w = &window{
			requests: make([]time.Time, 0, sw.limit),
		}
		sw.windows[key] = w
	}
	
	// Remove old requests outside the window
	cutoff := now.Add(-sw.duration)
	validRequests := make([]time.Time, 0, len(w.requests))
	for _, t := range w.requests {
		if t.After(cutoff) {
			validRequests = append(validRequests, t)
		}
	}
	w.requests = validRequests
	
	// Check if request is allowed
	allowed := len(w.requests) < sw.limit
	if allowed {
		w.requests = append(w.requests, now)
	}
	
	// Calculate reset time and retry after
	resetTime := now.Add(sw.duration)
	retryAfter := 0
	if !allowed && len(w.requests) > 0 {
		oldestRequest := w.requests[0]
		retryAfter = int(oldestRequest.Add(sw.duration).Sub(now).Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}
		resetTime = oldestRequest.Add(sw.duration)
	}
	
	info := &RateLimitInfo{
		Limit:     sw.limit,
		Remaining: sw.limit - len(w.requests),
		Reset:     resetTime,
		RetryAfter: retryAfter,
	}
	
	return allowed, info
}

// Reset resets the window for a key
func (sw *SlidingWindow) Reset(key string) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	delete(sw.windows, key)
}

// cleanupRoutine removes old windows
func (sw *SlidingWindow) cleanupRoutine() {
	ticker := time.NewTicker(sw.cleanup)
	defer ticker.Stop()
	
	for {
		select {
		case <-sw.stop:
			return
		case <-ticker.C:
			sw.mu.Lock()
			if sw.stopped {
				sw.mu.Unlock()
				return
			}
			now := time.Now()
			cutoff := now.Add(-sw.cleanup)
			for key, w := range sw.windows {
				if len(w.requests) == 0 || w.requests[len(w.requests)-1].Before(cutoff) {
					delete(sw.windows, key)
				}
			}
			sw.mu.Unlock()
		}
	}
}

// Stop gracefully stops the sliding window and cleans up resources
func (sw *SlidingWindow) Stop() {
	sw.mu.Lock()
	if !sw.stopped {
		sw.stopped = true
		close(sw.stop)
	}
	sw.mu.Unlock()
}

// RateLimitMiddleware creates rate limiting middleware
func RateLimitMiddleware(limiter RateLimiter, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			allowed, info := limiter.Allow(key)
			
			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(info.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(info.Remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(info.Reset.Unix(), 10))
			
			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(info.RetryAfter))
				
				// Log the rate limit violation
				if info.RetryAfter > 0 {
					// This would require importing the security package
					// For now, we'll add a comment indicating where security logging should go
					// security.DefaultSecurityLogger.LogRateLimitExceeded(r, info.Limit, time.Duration(info.RetryAfter) * time.Second)
				}
				
				Error(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED",
					"Rate limit exceeded. Please try again later.",
					map[string]interface{}{
						"limit":      info.Limit,
						"retry_after": info.RetryAfter,
						"reset":      info.Reset.Unix(),
					})
				return
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// IPKeyFunc returns client IP as rate limit key
func IPKeyFunc(r *http.Request) string {
	return GetClientIP(r, nil)
}

// GetClientIP extracts the real client IP with optional trusted proxy validation
func GetClientIP(r *http.Request, trustedProxies []string) string {
	// If no trusted proxies defined, only use RemoteAddr for security
	if len(trustedProxies) == 0 {
		return extractIPFromRemoteAddr(r.RemoteAddr)
	}
	
	// First check if the immediate connection is from a trusted proxy
	immediateIP := extractIPFromRemoteAddr(r.RemoteAddr)
	if !isIPTrusted(immediateIP, trustedProxies) {
		// If not from trusted proxy, use the immediate IP
		return immediateIP
	}
	
	// Only if from trusted proxy, then check forwarded headers
	// X-Real-IP takes precedence over X-Forwarded-For
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		if cleanIP := validateAndCleanIP(ip); cleanIP != "" {
			return cleanIP
		}
	}
	
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		// X-Forwarded-For can contain multiple IPs: "client, proxy1, proxy2"
		// We want the leftmost (original client) IP
		ips := strings.Split(forwardedFor, ",")
		if len(ips) > 0 {
			if cleanIP := validateAndCleanIP(strings.TrimSpace(ips[0])); cleanIP != "" {
				return cleanIP
			}
		}
	}
	
	// Fall back to remote address
	return immediateIP
}

// extractIPFromRemoteAddr extracts IP from RemoteAddr (which includes port)
func extractIPFromRemoteAddr(remoteAddr string) string {
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		return remoteAddr[:idx]
	}
	return remoteAddr
}

// validateAndCleanIP validates and cleans an IP address
func validateAndCleanIP(ip string) string {
	// Remove any whitespace
	ip = strings.TrimSpace(ip)
	
	// Basic validation - reject obvious malicious patterns
	if ip == "" || strings.Contains(ip, " ") || strings.Contains(ip, "\n") || strings.Contains(ip, "\r") {
		return ""
	}
	
	// Reject private/internal addresses that shouldn't be forwarded
	if isPrivateOrLoopback(ip) {
		return ""
	}
	
	return ip
}

// isPrivateOrLoopback checks if IP is private, loopback, or other non-routable
func isPrivateOrLoopback(ip string) bool {
	// Common private/internal ranges (basic check)
	if strings.HasPrefix(ip, "127.") ||           // Loopback
		strings.HasPrefix(ip, "10.") ||           // Private
		strings.HasPrefix(ip, "192.168.") ||      // Private
		strings.HasPrefix(ip, "172.16.") ||       // Private (partial check)
		strings.HasPrefix(ip, "169.254.") ||      // Link-local
		ip == "localhost" ||
		ip == "::1" {                             // IPv6 loopback
		return true
	}
	return false
}

// isIPTrusted checks if an IP is in the trusted proxy list
func isIPTrusted(ip string, trustedProxies []string) bool {
	for _, trusted := range trustedProxies {
		if ip == trusted {
			return true
		}
	}
	return false
}

// TrustedProxyIPKeyFunc returns a key function that validates proxy headers
func TrustedProxyIPKeyFunc(trustedProxies []string) func(*http.Request) string {
	return func(r *http.Request) string {
		return GetClientIP(r, trustedProxies)
	}
}

// UserKeyFunc returns user ID as rate limit key (requires authentication)
func UserKeyFunc(userIDFunc func(*http.Request) string) func(*http.Request) string {
	return func(r *http.Request) string {
		if userID := userIDFunc(r); userID != "" {
			return fmt.Sprintf("user:%s", userID)
		}
		// Fall back to IP if no user ID
		return IPKeyFunc(r)
	}
}

// APIKeyFunc returns API key as rate limit key
func APIKeyFunc(r *http.Request) string {
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return fmt.Sprintf("api:%s", apiKey)
	}
	// Fall back to IP if no API key
	return IPKeyFunc(r)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}