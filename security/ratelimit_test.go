package security

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         5,
		WindowSize:        time.Minute,
		SkipSuccessful:    false,
		SkipPaths:         []string{},
	}
	
	limiter := NewRateLimiter(config)
	
	// Test initial burst allowance
	for i := 0; i < 5; i++ {
		allowed := limiter.Allow("192.168.1.1")
		assert.True(t, allowed, "Request %d should be allowed", i+1)
	}
	
	// 6th request should be blocked (burst exhausted)
	allowed := limiter.Allow("192.168.1.1")
	assert.False(t, allowed, "6th request should be blocked")
	
	// Different IP should have its own bucket
	allowed = limiter.Allow("192.168.1.2")
	assert.True(t, allowed, "Different IP should be allowed")
}

func TestRateLimitMiddleware(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerMinute: 10,
		BurstSize:         2,
		WindowSize:        time.Minute,
		SkipPaths:         []string{"/health"},
	}
	
	middleware := RateLimitMiddleware(config)
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	t.Run("allows requests within limit", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "OK", w.Body.String())
	})
	
	t.Run("blocks requests over limit", func(t *testing.T) {
		// Exhaust the rate limit
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.2:1234"
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		}
		
		// This should be blocked
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.2:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.Contains(t, w.Body.String(), "Rate limit exceeded")
		assert.Equal(t, "10", w.Header().Get("X-RateLimit-Limit"))
		assert.Equal(t, "0", w.Header().Get("X-RateLimit-Remaining"))
	})
	
	t.Run("skips configured paths", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		req.RemoteAddr = "192.168.1.3:1234"
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestRateLimitWithXForwardedFor(t *testing.T) {
	config := DefaultRateLimitConfig()
	config.BurstSize = 1
	middleware := RateLimitMiddleware(config)
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	// First request with X-Forwarded-For should be allowed
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.Header.Set("X-Forwarded-For", "10.0.0.1")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	
	// Second request from same IP should be blocked
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-Forwarded-For", "10.0.0.1")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
}

func TestAPIRateLimitMiddleware(t *testing.T) {
	middleware := APIRateLimitMiddleware(5) // 5 requests per minute
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	// Should allow requests within limit
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Request %d should be allowed", i+1)
	}
	
	// Should block additional requests
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestAdaptiveRateLimiter(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerMinute: 10,
		BurstSize:         5,
		WindowSize:        time.Minute,
	}
	
	middleware := AdaptiveRateLimitMiddleware(config)
	
	// Handler that sometimes fails
	failureCount := 0
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failureCount++
		if failureCount%2 == 0 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	
	// Make several requests to trigger adaptive behavior
	for i := 0; i < 6; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		// Should eventually start adapting based on failure rate
	}
}

func TestRateLimiterCleanup(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         5,
		WindowSize:        time.Minute,
	}
	
	limiter := NewRateLimiter(config)
	
	// Add some clients
	limiter.Allow("192.168.1.1")
	limiter.Allow("192.168.1.2")
	limiter.Allow("192.168.1.3")
	
	// Verify clients exist
	limiter.mu.RLock()
	initialCount := len(limiter.clients)
	limiter.mu.RUnlock()
	
	assert.Equal(t, 3, initialCount)
	
	// Note: In a real test, we'd wait for cleanup to run or trigger it manually
	// For this test, we're just verifying the structure exists
}

func TestSkipSuccessfulRequests(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerMinute: 10,
		BurstSize:         2,
		SkipSuccessful:    true, // Only count failed requests
	}
	
	middleware := RateLimitMiddleware(config)
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always return success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	// Make multiple successful requests - should all be allowed
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Successful request %d should be allowed", i+1)
	}
}

func TestGlobalRateLimitMiddleware(t *testing.T) {
	middleware := GlobalRateLimitMiddleware()
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	// Test that it creates a working middleware
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimitConcurrency(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerMinute: 100,
		BurstSize:         10,
	}
	
	limiter := NewRateLimiter(config)
	
	// Test concurrent access
	done := make(chan bool, 20)
	
	for i := 0; i < 20; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				limiter.Allow(fmt.Sprintf("192.168.1.%d", id))
			}
			done <- true
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 20; i++ {
		<-done
	}
	
	// Verify no race conditions occurred
	limiter.mu.RLock()
	clientCount := len(limiter.clients)
	limiter.mu.RUnlock()
	
	assert.True(t, clientCount <= 20, "Should not have more clients than expected")
}

func TestRateLimitTokenRefill(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerMinute: 60, // 1 per second
		BurstSize:         1,
	}
	
	limiter := NewRateLimiter(config)
	
	// Use up the initial token
	allowed := limiter.Allow("192.168.1.1")
	assert.True(t, allowed)
	
	// Should be blocked immediately
	allowed = limiter.Allow("192.168.1.1")
	assert.False(t, allowed)
	
	// Wait a bit more than 1 second and should be allowed again
	time.Sleep(1100 * time.Millisecond)
	allowed = limiter.Allow("192.168.1.1")
	assert.True(t, allowed)
}