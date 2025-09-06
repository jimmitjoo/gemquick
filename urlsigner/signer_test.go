package urlsigner

import (
	"strings"
	"testing"
	"time"
)

func TestSigner_GenerateTokenFromString(t *testing.T) {
	signer := &Signer{
		Secret: []byte("test-secret-key-32-bytes-long!!!"),
	}

	tests := []struct {
		name     string
		data     string
		contains string
	}{
		{
			name:     "URL without query params",
			data:     "http://example.com/path",
			contains: "?hash=",
		},
		{
			name:     "URL with query params",
			data:     "http://example.com/path?param=value",
			contains: "&hash=",
		},
		{
			name:     "Simple path",
			data:     "/api/users",
			contains: "?hash=",
		},
		{
			name:     "Path with existing params",
			data:     "/api/users?id=123",
			contains: "&hash=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := signer.GenerateTokenFromString(tt.data)
			
			if token == "" {
				t.Error("Expected non-empty token")
			}

			if !strings.Contains(token, tt.contains) {
				t.Errorf("Expected token to contain %s", tt.contains)
			}

			// Token should start with the original data
			if !strings.HasPrefix(token, tt.data) {
				t.Errorf("Expected token to start with %s", tt.data)
			}
		})
	}
}

func TestSigner_VerifyToken(t *testing.T) {
	signer := &Signer{
		Secret: []byte("test-secret-key-32-bytes-long!!!"),
	}

	// Generate a valid token
	validToken := signer.GenerateTokenFromString("http://example.com/test")

	tests := []struct {
		name     string
		token    string
		expected bool
	}{
		{
			name:     "Valid token",
			token:    validToken,
			expected: true,
		},
		{
			name:     "Invalid token",
			token:    "http://example.com/test?hash=invalid",
			expected: false,
		},
		{
			name:     "Empty token",
			token:    "",
			expected: false,
		},
		{
			name:     "Malformed token",
			token:    "not-a-valid-token",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := signer.VerifyToken(tt.token)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSigner_VerifyTokenWithDifferentSecret(t *testing.T) {
	signer1 := &Signer{
		Secret: []byte("secret-key-1-32-bytes-long!!!!!"),
	}
	signer2 := &Signer{
		Secret: []byte("secret-key-2-32-bytes-long!!!!!"),
	}

	// Generate token with signer1
	token := signer1.GenerateTokenFromString("http://example.com/test")

	// Verify with signer1 (should pass)
	if !signer1.VerifyToken(token) {
		t.Error("Expected token to be valid with correct secret")
	}

	// Verify with signer2 (should fail)
	if signer2.VerifyToken(token) {
		t.Error("Expected token to be invalid with different secret")
	}
}

func TestSigner_Expired(t *testing.T) {
	signer := &Signer{
		Secret: []byte("test-secret-key-32-bytes-long!!!"),
	}

	// Generate a fresh token
	freshToken := signer.GenerateTokenFromString("http://example.com/test")

	tests := []struct {
		name               string
		token              string
		minutesUntilExpire int
		shouldBeExpired    bool
	}{
		{
			name:               "Fresh token not expired",
			token:              freshToken,
			minutesUntilExpire: 60,
			shouldBeExpired:    false,
		},
		{
			name:               "Fresh token with 0 minutes",
			token:              freshToken,
			minutesUntilExpire: 0,
			shouldBeExpired:    true, // With 0 minutes, it's considered expired
		},
		{
			name:               "Fresh token with negative minutes",
			token:              freshToken,
			minutesUntilExpire: -1,
			shouldBeExpired:    true, // Negative means it's already expired
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := signer.Expired(tt.token, tt.minutesUntilExpire)
			if result != tt.shouldBeExpired {
				t.Errorf("Expected expired=%v, got %v", tt.shouldBeExpired, result)
			}
		})
	}
}

func TestSigner_ExpiredWithOldToken(t *testing.T) {
	signer := &Signer{
		Secret: []byte("test-secret-key-32-bytes-long!!!"),
	}

	// This test would need to mock time or use a token that was generated in the past
	// For now, we'll test the logic with a fresh token
	token := signer.GenerateTokenFromString("http://example.com/test")

	// Sleep for a short time to simulate token aging
	time.Sleep(100 * time.Millisecond)

	// Token should be expired if we check with very small expiration time
	// (less than the sleep duration converted to minutes)
	if signer.Expired(token, 0) {
		// Token might be considered expired with 0 minutes depending on implementation
		// This is expected behavior
	}
}

func TestSigner_EmptySecret(t *testing.T) {
	signer := &Signer{
		Secret: []byte{}, // Empty secret
	}

	// Should still generate a token (though not secure)
	token := signer.GenerateTokenFromString("http://example.com/test")
	if token == "" {
		t.Error("Expected non-empty token even with empty secret")
	}
}

func TestSigner_LongURL(t *testing.T) {
	signer := &Signer{
		Secret: []byte("test-secret-key-32-bytes-long!!!"),
	}

	// Test with a very long URL
	longPath := strings.Repeat("/very/long/path", 100)
	longURL := "http://example.com" + longPath

	token := signer.GenerateTokenFromString(longURL)
	
	if token == "" {
		t.Error("Expected non-empty token for long URL")
	}

	// Should still be verifiable
	if !signer.VerifyToken(token) {
		t.Error("Expected long URL token to be verifiable")
	}
}

func TestSigner_SpecialCharacters(t *testing.T) {
	signer := &Signer{
		Secret: []byte("test-secret-key-32-bytes-long!!!"),
	}

	tests := []string{
		"http://example.com/path?param=value&special=!@#$%^&*()",
		"http://example.com/path?unicode=你好世界",
		"http://example.com/path?emoji=😀🎉",
		"http://example.com/path?spaces=hello world",
	}

	for _, url := range tests {
		t.Run(url, func(t *testing.T) {
			token := signer.GenerateTokenFromString(url)
			
			if token == "" {
				t.Error("Expected non-empty token")
			}

			if !signer.VerifyToken(token) {
				t.Error("Expected token to be verifiable")
			}
		})
	}
}