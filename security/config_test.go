package security

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoadFromEnv(t *testing.T) {
	// Clear environment first
	clearSecurityEnvVars()
	
	// Set some test environment variables
	os.Setenv("ENVIRONMENT", "testing")
	os.Setenv("SECURITY_RATE_LIMIT", "true")
	os.Setenv("SECURITY_HEADERS", "false")
	os.Setenv("RATE_LIMIT_RPM", "120")
	os.Setenv("MAX_REQUEST_SIZE", "5242880") // 5MB
	os.Setenv("TRUSTED_PROXIES", "127.0.0.1,10.0.0.1")
	
	defer clearSecurityEnvVars()
	
	config := LoadFromEnv()
	
	assert.Equal(t, "testing", config.Environment)
	assert.True(t, config.EnableRateLimit)
	assert.False(t, config.EnableHeaders)
	assert.Equal(t, 120, config.RateLimit.RequestsPerMinute)
	assert.Equal(t, int64(5242880), config.MaxRequestSize)
	assert.Contains(t, config.TrustedProxies, "127.0.0.1")
	assert.Contains(t, config.TrustedProxies, "10.0.0.1")
}

func TestDevelopmentConfig(t *testing.T) {
	clearSecurityEnvVars()
	defer clearSecurityEnvVars()
	
	config := DevelopmentConfig()
	
	assert.Equal(t, "development", config.Environment)
	assert.False(t, config.EnableIPFilter)
	assert.Equal(t, 300, config.RateLimit.RequestsPerMinute) // Higher limits for dev
	assert.Equal(t, 0, config.Headers.HSTSMaxAge) // HSTS disabled in dev
	assert.False(t, config.CSRF.CookieSecure) // Cookies not secure in dev
}

func TestProductionConfig(t *testing.T) {
	clearSecurityEnvVars()
	defer clearSecurityEnvVars()
	
	config := ProductionConfig()
	
	assert.Equal(t, "production", config.Environment)
	assert.True(t, config.EnableIPFilter)
	assert.True(t, config.APIKeyRequired)
	assert.Equal(t, 63072000, config.Headers.HSTSMaxAge) // 2 years HSTS
	assert.True(t, config.Headers.HSTSPreload)
	assert.Equal(t, "DENY", config.Headers.FrameOptions)
}

func TestStagingConfig(t *testing.T) {
	clearSecurityEnvVars()
	defer clearSecurityEnvVars()
	
	config := StagingConfig()
	
	assert.Equal(t, "staging", config.Environment)
	assert.True(t, config.EnableIPFilter) // Same as production
	assert.False(t, config.APIKeyRequired) // But no API key required
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid config",
			config: Config{
				EnableRateLimit:  true,
				RateLimit:        RateLimitConfig{RequestsPerMinute: 60, BurstSize: 10},
				EnableCSRF:       true,
				CSRF:            CSRFConfig{TokenLength: 32, CookieMaxAge: 3600},
				EnableThrottling: true,
				Throttle:        ThrottleConfig{RequestsPerMinute: 100, SuspiciousThreshold: 5},
				MaxRequestSize:   1024 * 1024,
				RequestTimeout:   30 * time.Second,
			},
			expectErr: false,
		},
		{
			name: "invalid rate limit",
			config: Config{
				EnableRateLimit: true,
				RateLimit:       RateLimitConfig{RequestsPerMinute: 0, BurstSize: 10},
				MaxRequestSize:  1024 * 1024,
				RequestTimeout:  30 * time.Second,
			},
			expectErr: true,
			errMsg:    "rate limit requests per minute must be positive",
		},
		{
			name: "invalid CSRF token length",
			config: Config{
				EnableCSRF:     true,
				CSRF:          CSRFConfig{TokenLength: 8, CookieMaxAge: 3600},
				MaxRequestSize: 1024 * 1024,
				RequestTimeout: 30 * time.Second,
			},
			expectErr: true,
			errMsg:    "CSRF token length must be at least 16 bytes",
		},
		{
			name: "invalid throttle config",
			config: Config{
				EnableThrottling: true,
				Throttle:        ThrottleConfig{RequestsPerMinute: 0, SuspiciousThreshold: 5},
				MaxRequestSize:   1024 * 1024,
				RequestTimeout:   30 * time.Second,
			},
			expectErr: true,
			errMsg:    "throttle requests per minute must be positive",
		},
		{
			name: "invalid max request size",
			config: Config{
				MaxRequestSize: 0,
				RequestTimeout: 30 * time.Second,
			},
			expectErr: true,
			errMsg:    "max request size must be positive",
		},
		{
			name: "invalid request timeout",
			config: Config{
				MaxRequestSize: 1024 * 1024,
				RequestTimeout: 0,
			},
			expectErr: true,
			errMsg:    "request timeout must be positive",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateConfig()
			
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetSecuritySummary(t *testing.T) {
	config := Config{
		Environment:     "production",
		EnableRateLimit: true,
		EnableHeaders:   true,
		EnableCSRF:      true,
		EnableThrottling: true,
		EnableCORS:      true,
		EnableIPFilter:  true,
		APIKeyRequired:  true,
		MaxRequestSize:  10 * 1024 * 1024,
		RequestTimeout:  30 * time.Second,
		Throttle: ThrottleConfig{
			EnableProgressive:       true,
			EnableSuspiciousDetection: true,
			EnableSubnetLimiting:    true,
		},
		Headers: SecurityConfig{
			HSTSMaxAge:            31536000,
			ContentSecurityPolicy: "default-src 'self'",
		},
	}
	
	summary := config.GetSecuritySummary()
	
	assert.Equal(t, "production", summary["environment"])
	assert.True(t, summary["rate_limiting"].(bool))
	assert.True(t, summary["security_headers"].(bool))
	assert.True(t, summary["csrf_protection"].(bool))
	assert.True(t, summary["ip_throttling"].(bool))
	assert.True(t, summary["cors_enabled"].(bool))
	assert.True(t, summary["ip_filtering"].(bool))
	assert.True(t, summary["api_key_required"].(bool))
	assert.Equal(t, int64(10*1024*1024), summary["max_request_size"])
	assert.Equal(t, "30s", summary["request_timeout"])
	
	features := summary["features"].(map[string]interface{})
	assert.True(t, features["progressive_throttling"].(bool))
	assert.True(t, features["suspicious_detection"].(bool))
	assert.True(t, features["subnet_limiting"].(bool))
	assert.True(t, features["hsts_enabled"].(bool))
	assert.True(t, features["csp_enabled"].(bool))
}

func TestLoadRateLimitConfig(t *testing.T) {
	clearSecurityEnvVars()
	
	os.Setenv("RATE_LIMIT_RPM", "120")
	os.Setenv("RATE_LIMIT_BURST", "15")
	os.Setenv("RATE_LIMIT_WINDOW", "90")
	os.Setenv("RATE_LIMIT_SKIP_SUCCESSFUL", "true")
	os.Setenv("RATE_LIMIT_SKIP_PATHS", "/api/health,/api/status")
	
	defer clearSecurityEnvVars()
	
	config := loadRateLimitConfig()
	
	assert.Equal(t, 120, config.RequestsPerMinute)
	assert.Equal(t, 15, config.BurstSize)
	assert.Equal(t, 90*time.Second, config.WindowSize)
	assert.True(t, config.SkipSuccessful)
	assert.Contains(t, config.SkipPaths, "/api/health")
	assert.Contains(t, config.SkipPaths, "/api/status")
}

func TestLoadSecurityHeadersConfig(t *testing.T) {
	clearSecurityEnvVars()
	
	os.Setenv("CSP_POLICY", "default-src 'self'; script-src 'self' 'unsafe-inline'")
	os.Setenv("HSTS_MAX_AGE", "7200")
	os.Setenv("HSTS_INCLUDE_SUBDOMAINS", "false")
	os.Setenv("HSTS_PRELOAD", "true")
	os.Setenv("FRAME_OPTIONS", "SAMEORIGIN")
	os.Setenv("CORS_ALLOWED_ORIGINS", "https://example.com,https://api.example.com")
	
	defer clearSecurityEnvVars()
	
	config := loadSecurityHeadersConfig("production")
	
	assert.Equal(t, "default-src 'self'; script-src 'self' 'unsafe-inline'", config.ContentSecurityPolicy)
	assert.Equal(t, 7200, config.HSTSMaxAge)
	assert.False(t, config.HSTSIncludeSubdomains)
	assert.True(t, config.HSTSPreload)
	assert.Equal(t, "SAMEORIGIN", config.FrameOptions)
	assert.Contains(t, config.AllowedOrigins, "https://example.com")
	assert.Contains(t, config.AllowedOrigins, "https://api.example.com")
}

func TestLoadCSRFConfig(t *testing.T) {
	clearSecurityEnvVars()
	
	os.Setenv("CSRF_TOKEN_LENGTH", "64")
	os.Setenv("CSRF_COOKIE_NAME", "custom_csrf")
	os.Setenv("CSRF_COOKIE_MAX_AGE", "7200")
	os.Setenv("CSRF_EXEMPT_PATHS", "/api/webhook,/api/callback")
	
	defer clearSecurityEnvVars()
	
	config := loadCSRFConfig("production")
	
	assert.Equal(t, 64, config.TokenLength)
	assert.Equal(t, "custom_csrf", config.CookieName)
	assert.Equal(t, 7200, config.CookieMaxAge)
	assert.Contains(t, config.ExemptPaths, "/api/webhook")
	assert.Contains(t, config.ExemptPaths, "/api/callback")
}

func TestLoadThrottleConfig(t *testing.T) {
	clearSecurityEnvVars()
	
	os.Setenv("THROTTLE_RPM", "200")
	os.Setenv("THROTTLE_BURST", "25")
	os.Setenv("THROTTLE_PROGRESSIVE", "false")
	os.Setenv("THROTTLE_SUSPICIOUS_THRESHOLD", "15")
	os.Setenv("THROTTLE_WHITELIST", "127.0.0.1,192.168.1.100")
	os.Setenv("THROTTLE_BLACKLIST", "10.0.0.1,172.16.0.1")
	
	defer clearSecurityEnvVars()
	
	config := loadThrottleConfig()
	
	assert.Equal(t, 200, config.RequestsPerMinute)
	assert.Equal(t, 25, config.BurstSize)
	assert.False(t, config.EnableProgressive)
	assert.Equal(t, 15, config.SuspiciousThreshold)
	assert.Contains(t, config.WhitelistedIPs, "127.0.0.1")
	assert.Contains(t, config.WhitelistedIPs, "192.168.1.100")
	assert.Contains(t, config.BlacklistedIPs, "10.0.0.1")
	assert.Contains(t, config.BlacklistedIPs, "172.16.0.1")
}

func TestEnvironmentHelperFunctions(t *testing.T) {
	clearSecurityEnvVars()
	
	// Test string helper
	os.Setenv("TEST_STRING", "hello world")
	assert.Equal(t, "hello world", getEnvString("TEST_STRING", "default"))
	assert.Equal(t, "default", getEnvString("NONEXISTENT", "default"))
	
	// Test int helper
	os.Setenv("TEST_INT", "42")
	assert.Equal(t, 42, getEnvInt("TEST_INT", 0))
	assert.Equal(t, 10, getEnvInt("NONEXISTENT", 10))
	
	// Test invalid int
	os.Setenv("TEST_INVALID_INT", "not_a_number")
	assert.Equal(t, 10, getEnvInt("TEST_INVALID_INT", 10))
	
	// Test int64 helper
	os.Setenv("TEST_INT64", "9223372036854775807")
	assert.Equal(t, int64(9223372036854775807), getEnvInt64("TEST_INT64", 0))
	assert.Equal(t, int64(100), getEnvInt64("NONEXISTENT", 100))
	
	// Test bool helper
	os.Setenv("TEST_BOOL_TRUE", "true")
	os.Setenv("TEST_BOOL_FALSE", "false")
	assert.True(t, getEnvBool("TEST_BOOL_TRUE", false))
	assert.False(t, getEnvBool("TEST_BOOL_FALSE", true))
	assert.True(t, getEnvBool("NONEXISTENT", true))
	
	// Test invalid bool
	os.Setenv("TEST_INVALID_BOOL", "not_a_bool")
	assert.True(t, getEnvBool("TEST_INVALID_BOOL", true))
	
	// Test string slice helper
	os.Setenv("TEST_STRING_SLICE", "item1,item2,item3")
	result := getEnvStringSlice("TEST_STRING_SLICE", []string{"default"})
	assert.Equal(t, []string{"item1", "item2", "item3"}, result)
	
	defaultSlice := []string{"default1", "default2"}
	result = getEnvStringSlice("NONEXISTENT", defaultSlice)
	assert.Equal(t, defaultSlice, result)
	
	defer clearSecurityEnvVars()
}

func TestConfigurationPresets(t *testing.T) {
	clearSecurityEnvVars()
	defer clearSecurityEnvVars()
	
	// Test that different presets have expected characteristics
	devConfig := DevelopmentConfig()
	prodConfig := ProductionConfig()
	stagingConfig := StagingConfig()
	
	// Development should be more permissive
	assert.True(t, devConfig.RateLimit.RequestsPerMinute > prodConfig.RateLimit.RequestsPerMinute)
	assert.False(t, devConfig.EnableIPFilter)
	assert.Equal(t, 0, devConfig.Headers.HSTSMaxAge)
	
	// Production should be stricter
	assert.True(t, prodConfig.APIKeyRequired)
	assert.True(t, prodConfig.EnableIPFilter)
	assert.True(t, prodConfig.Headers.HSTSMaxAge > 0)
	assert.True(t, prodConfig.Headers.HSTSPreload)
	
	// Staging should be like production but more lenient
	assert.False(t, stagingConfig.APIKeyRequired)
	assert.True(t, stagingConfig.EnableIPFilter) // Same as prod
}

func TestDefaultConfigurations(t *testing.T) {
	// Test that default configurations are sane
	rateLimitConfig := DefaultRateLimitConfig()
	assert.True(t, rateLimitConfig.RequestsPerMinute > 0)
	assert.True(t, rateLimitConfig.BurstSize > 0)
	assert.True(t, len(rateLimitConfig.SkipPaths) > 0)
	
	securityConfig := DefaultSecurityConfig()
	assert.NotEmpty(t, securityConfig.ContentSecurityPolicy)
	assert.True(t, securityConfig.HSTSMaxAge > 0)
	assert.True(t, securityConfig.ContentTypeNosniff)
	
	csrfConfig := DefaultCSRFConfig()
	assert.True(t, csrfConfig.TokenLength >= 16)
	assert.True(t, csrfConfig.CookieHttpOnly)
	assert.True(t, len(csrfConfig.ExemptPaths) > 0)
	
	throttleConfig := DefaultThrottleConfig()
	assert.True(t, throttleConfig.RequestsPerMinute > 0)
	assert.True(t, throttleConfig.SuspiciousThreshold > 0)
	assert.True(t, len(throttleConfig.TrustedProxyHeaders) > 0)
}

// Helper function to clear all security-related environment variables
func clearSecurityEnvVars() {
	envVars := []string{
		"ENVIRONMENT",
		"SECURITY_RATE_LIMIT",
		"SECURITY_HEADERS",
		"SECURITY_CSRF",
		"SECURITY_THROTTLING",
		"SECURITY_CORS",
		"SECURITY_IP_FILTER",
		"MAX_REQUEST_SIZE",
		"REQUEST_TIMEOUT",
		"TRUSTED_PROXIES",
		"BLOCKED_IPS",
		"ALLOWED_IPS",
		"API_KEY_REQUIRED",
		"API_KEY_HEADER",
		"RATE_LIMIT_RPM",
		"RATE_LIMIT_BURST",
		"RATE_LIMIT_WINDOW",
		"RATE_LIMIT_SKIP_SUCCESSFUL",
		"RATE_LIMIT_SKIP_PATHS",
		"CSP_POLICY",
		"HSTS_MAX_AGE",
		"HSTS_INCLUDE_SUBDOMAINS",
		"HSTS_PRELOAD",
		"FRAME_OPTIONS",
		"CONTENT_TYPE_NOSNIFF",
		"XSS_PROTECTION",
		"XSS_PROTECTION_MODE",
		"REFERRER_POLICY",
		"PERMISSIONS_POLICY",
		"CORS_ALLOWED_ORIGINS",
		"CORS_ALLOWED_METHODS",
		"CORS_ALLOWED_HEADERS",
		"CORS_ALLOW_CREDENTIALS",
		"CORS_MAX_AGE",
		"CSRF_TOKEN_LENGTH",
		"CSRF_COOKIE_NAME",
		"CSRF_COOKIE_PATH",
		"CSRF_COOKIE_DOMAIN",
		"CSRF_COOKIE_SECURE",
		"CSRF_COOKIE_HTTP_ONLY",
		"CSRF_COOKIE_MAX_AGE",
		"CSRF_REQUEST_HEADER",
		"CSRF_FORM_FIELD",
		"CSRF_EXEMPT_PATHS",
		"CSRF_EXEMPT_GLOBS",
		"CSRF_EXEMPT_METHODS",
		"THROTTLE_RPM",
		"THROTTLE_BURST",
		"THROTTLE_WINDOW",
		"THROTTLE_PROGRESSIVE",
		"THROTTLE_MAX_PENALTY",
		"THROTTLE_SUSPICIOUS_DETECTION",
		"THROTTLE_SUSPICIOUS_THRESHOLD",
		"THROTTLE_SUSPICIOUS_PENALTY",
		"THROTTLE_SUBNET_LIMITING",
		"THROTTLE_SUBNET_RPM",
		"THROTTLE_SUBNET_MASK",
		"THROTTLE_WHITELIST",
		"THROTTLE_BLACKLIST",
		"THROTTLE_PROXY_HEADERS",
		"THROTTLE_TRUSTED_PROXIES",
		"TEST_STRING",
		"TEST_INT",
		"TEST_INVALID_INT",
		"TEST_INT64",
		"TEST_BOOL_TRUE",
		"TEST_BOOL_FALSE",
		"TEST_INVALID_BOOL",
		"TEST_STRING_SLICE",
	}
	
	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}