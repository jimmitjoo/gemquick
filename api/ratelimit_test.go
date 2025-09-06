package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTokenBucket(t *testing.T) {
	// Create a token bucket with 5 tokens, capacity 5, refilling every 100ms
	tb := NewTokenBucket(5, 5, 100*time.Millisecond)
	defer tb.Reset("test-key") // Clean up

	// Test initial state - should have full capacity
	for i := 0; i < 5; i++ {
		allowed, info := tb.Allow("test-key")
		if !allowed {
			t.Errorf("TokenBucket request %d should be allowed", i+1)
		}
		if info.Remaining != 4-i {
			t.Errorf("TokenBucket remaining = %d, want %d", info.Remaining, 4-i)
		}
	}

	// 6th request should be denied
	allowed, info := tb.Allow("test-key")
	if allowed {
		t.Error("TokenBucket 6th request should be denied")
	}
	if info.RetryAfter <= 0 {
		t.Error("TokenBucket RetryAfter should be positive when rate limited")
	}

	// Wait for refill
	time.Sleep(150 * time.Millisecond)

	// Should have tokens again
	allowed, info = tb.Allow("test-key")
	if !allowed {
		t.Error("TokenBucket should allow request after refill")
	}
	if info.Remaining <= 0 {
		t.Error("TokenBucket should have remaining tokens after refill")
	}
}

func TestTokenBucketReset(t *testing.T) {
	tb := NewTokenBucket(1, 1, time.Hour)

	// Use up the token
	tb.Allow("test-key")
	
	// Should be denied
	allowed, _ := tb.Allow("test-key")
	if allowed {
		t.Error("TokenBucket should deny after using all tokens")
	}

	// Reset the bucket
	tb.Reset("test-key")

	// Should be allowed again
	allowed, _ = tb.Allow("test-key")
	if !allowed {
		t.Error("TokenBucket should allow after reset")
	}
}

func TestTokenBucketMultipleKeys(t *testing.T) {
	tb := NewTokenBucket(2, 2, time.Hour)

	// Use tokens for key1
	tb.Allow("key1")
	tb.Allow("key1")

	// key1 should be denied
	allowed, _ := tb.Allow("key1")
	if allowed {
		t.Error("TokenBucket key1 should be denied")
	}

	// key2 should still have tokens
	allowed, _ = tb.Allow("key2")
	if !allowed {
		t.Error("TokenBucket key2 should be allowed")
	}
}

func TestSlidingWindow(t *testing.T) {
	// Create a sliding window with limit 3 per 100ms
	sw := NewSlidingWindow(3, 100*time.Millisecond)
	defer sw.Reset("test-key")

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		allowed, info := sw.Allow("test-key")
		if !allowed {
			t.Errorf("SlidingWindow request %d should be allowed", i+1)
		}
		if info.Remaining != 2-i {
			t.Errorf("SlidingWindow remaining = %d, want %d", info.Remaining, 2-i)
		}
	}

	// 4th request should be denied
	allowed, info := sw.Allow("test-key")
	if allowed {
		t.Error("SlidingWindow 4th request should be denied")
	}
	if info.RetryAfter <= 0 {
		t.Error("SlidingWindow RetryAfter should be positive when rate limited")
	}

	// Wait for window to slide
	time.Sleep(150 * time.Millisecond)

	// Should allow requests again
	allowed, _ = sw.Allow("test-key")
	if !allowed {
		t.Error("SlidingWindow should allow request after window slides")
	}
}

func TestSlidingWindowReset(t *testing.T) {
	sw := NewSlidingWindow(1, time.Hour)

	// Use up the limit
	sw.Allow("test-key")
	
	// Should be denied
	allowed, _ := sw.Allow("test-key")
	if allowed {
		t.Error("SlidingWindow should deny after reaching limit")
	}

	// Reset the window
	sw.Reset("test-key")

	// Should be allowed again
	allowed, _ = sw.Allow("test-key")
	if !allowed {
		t.Error("SlidingWindow should allow after reset")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	limiter := NewTokenBucket(2, 2, time.Hour)
	
	handler := RateLimitMiddleware(limiter, IPKeyFunc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "192.168.1.1:1234"
		
		handler.ServeHTTP(w, r)
		
		if w.Code != http.StatusOK {
			t.Errorf("RateLimitMiddleware request %d status = %v, want %v", i+1, w.Code, http.StatusOK)
		}
		
		// Check headers
		if limit := w.Header().Get("X-RateLimit-Limit"); limit != "2" {
			t.Errorf("RateLimitMiddleware X-RateLimit-Limit = %v, want 2", limit)
		}
		
		expectedRemaining := fmt.Sprintf("%d", 1-i)
		if remaining := w.Header().Get("X-RateLimit-Remaining"); remaining != expectedRemaining {
			t.Errorf("RateLimitMiddleware X-RateLimit-Remaining = %v, want %v", remaining, expectedRemaining)
		}
	}

	// 3rd request should be rate limited
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.1:1234"
	
	handler.ServeHTTP(w, r)
	
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("RateLimitMiddleware 3rd request status = %v, want %v", w.Code, http.StatusTooManyRequests)
	}
	
	if retryAfter := w.Header().Get("Retry-After"); retryAfter == "" {
		t.Error("RateLimitMiddleware missing Retry-After header when rate limited")
	}
	
	// Check error response
	body := w.Body.String()
	if !strings.Contains(body, "RATE_LIMIT_EXCEEDED") {
		t.Error("RateLimitMiddleware should return RATE_LIMIT_EXCEEDED error")
	}
}

func TestIPKeyFunc(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		wantIP     string
	}{
		{
			name:       "from RemoteAddr",
			remoteAddr: "192.168.1.1:1234",
			wantIP:     "192.168.1.1",
		},
		{
			name:       "from X-Real-IP",
			remoteAddr: "10.0.0.1:1234",
			headers:    map[string]string{"X-Real-IP": "203.0.113.1"},
			wantIP:     "203.0.113.1",
		},
		{
			name:       "from X-Forwarded-For single",
			remoteAddr: "10.0.0.1:1234",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.2"},
			wantIP:     "203.0.113.2",
		},
		{
			name:       "from X-Forwarded-For multiple",
			remoteAddr: "10.0.0.1:1234",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.3, 10.0.0.2, 10.0.0.3"},
			wantIP:     "203.0.113.3",
		},
		{
			name:       "X-Real-IP takes precedence",
			remoteAddr: "10.0.0.1:1234",
			headers: map[string]string{
				"X-Real-IP":       "203.0.113.4",
				"X-Forwarded-For": "203.0.113.5",
			},
			wantIP: "203.0.113.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				r.Header.Set(k, v)
			}

			ip := IPKeyFunc(r)
			if ip != tt.wantIP {
				t.Errorf("IPKeyFunc() = %v, want %v", ip, tt.wantIP)
			}
		})
	}
}

func TestUserKeyFunc(t *testing.T) {
	userIDFunc := func(r *http.Request) string {
		return r.Header.Get("X-User-ID")
	}

	keyFunc := UserKeyFunc(userIDFunc)

	tests := []struct {
		name     string
		userID   string
		wantKey  string
	}{
		{
			name:    "with user ID",
			userID:  "user123",
			wantKey: "user:user123",
		},
		{
			name:    "without user ID falls back to IP",
			userID:  "",
			wantKey: "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = "192.168.1.1:1234"
			if tt.userID != "" {
				r.Header.Set("X-User-ID", tt.userID)
			}

			key := keyFunc(r)
			if key != tt.wantKey {
				t.Errorf("UserKeyFunc() = %v, want %v", key, tt.wantKey)
			}
		})
	}
}

func TestAPIKeyFunc(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		wantKey string
	}{
		{
			name:    "with API key",
			apiKey:  "sk-12345",
			wantKey: "api:sk-12345",
		},
		{
			name:    "without API key falls back to IP",
			apiKey:  "",
			wantKey: "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = "192.168.1.1:1234"
			if tt.apiKey != "" {
				r.Header.Set("X-API-Key", tt.apiKey)
			}

			key := APIKeyFunc(r)
			if key != tt.wantKey {
				t.Errorf("APIKeyFunc() = %v, want %v", key, tt.wantKey)
			}
		})
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{1, 2, 1},
		{2, 1, 1},
		{5, 5, 5},
		{-1, 0, -1},
		{10, 3, 3},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if got := min(tt.a, tt.b); got != tt.want {
				t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestTokenBucketCleanup(t *testing.T) {
	// Create token bucket with very short cleanup interval
	tb := &TokenBucket{
		buckets:  make(map[string]*bucket),
		rate:     1,
		capacity: 1,
		interval: 10 * time.Millisecond,
		cleanup:  20 * time.Millisecond,
	}

	// Add a bucket
	tb.Allow("old-key")

	// Start cleanup routine
	go tb.cleanupRoutine()

	// Wait for cleanup to run
	time.Sleep(50 * time.Millisecond)

	// Check that old bucket was cleaned up
	tb.mu.RLock()
	_, exists := tb.buckets["old-key"]
	tb.mu.RUnlock()

	if exists {
		t.Error("TokenBucket cleanup should have removed old bucket")
	}
}

func TestSlidingWindowCleanup(t *testing.T) {
	// Create sliding window with very short cleanup interval
	sw := &SlidingWindow{
		windows:  make(map[string]*window),
		limit:    1,
		duration: 10 * time.Millisecond,
		cleanup:  20 * time.Millisecond,
	}

	// Add a window
	sw.Allow("old-key")

	// Start cleanup routine
	go sw.cleanupRoutine()

	// Wait for cleanup to run
	time.Sleep(50 * time.Millisecond)

	// Check that old window was cleaned up
	sw.mu.RLock()
	_, exists := sw.windows["old-key"]
	sw.mu.RUnlock()

	if exists {
		t.Error("SlidingWindow cleanup should have removed old window")
	}
}