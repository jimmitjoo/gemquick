package security

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// SecurityEventType represents different types of security events
type SecurityEventType string

const (
	EventRateLimitExceeded  SecurityEventType = "rate_limit_exceeded"
	EventCSRFFailure        SecurityEventType = "csrf_failure"
	EventSuspiciousRequest  SecurityEventType = "suspicious_request"
	EventInvalidOrigin      SecurityEventType = "invalid_origin"
	EventIPBlocked          SecurityEventType = "ip_blocked"
	EventAuthFailure        SecurityEventType = "auth_failure"
	EventSQLInjectionAttempt SecurityEventType = "sql_injection_attempt"
	EventXSSAttempt         SecurityEventType = "xss_attempt"
	EventPathTraversal      SecurityEventType = "path_traversal_attempt"
)

// SecurityEvent represents a security-related event
type SecurityEvent struct {
	Timestamp   time.Time         `json:"timestamp"`
	EventType   SecurityEventType `json:"event_type"`
	Severity    string            `json:"severity"`    // low, medium, high, critical
	ClientIP    string            `json:"client_ip"`
	UserAgent   string            `json:"user_agent"`
	RequestURI  string            `json:"request_uri"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers,omitempty"`
	UserID      string            `json:"user_id,omitempty"`
	Message     string            `json:"message"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Action      string            `json:"action"`      // blocked, allowed, monitored
}

// SecurityLogger handles structured logging of security events
type SecurityLogger struct {
	logger *log.Logger
}

// NewSecurityLogger creates a new security logger
func NewSecurityLogger(logger *log.Logger) *SecurityLogger {
	if logger == nil {
		logger = log.Default()
	}
	return &SecurityLogger{logger: logger}
}

// LogEvent logs a security event in structured format
func (sl *SecurityLogger) LogEvent(event SecurityEvent) {
	// Ensure timestamp is set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	
	// Convert to JSON for structured logging
	if jsonData, err := json.Marshal(event); err == nil {
		sl.logger.Printf("[SECURITY] %s", string(jsonData))
	} else {
		// Fallback to simple logging if JSON marshaling fails
		sl.logger.Printf("[SECURITY] %s - %s - %s - %s", 
			event.EventType, event.Severity, event.ClientIP, event.Message)
	}
}

// LogRateLimitExceeded logs rate limit violations
func (sl *SecurityLogger) LogRateLimitExceeded(r *http.Request, limit int, duration time.Duration) {
	event := SecurityEvent{
		EventType:  EventRateLimitExceeded,
		Severity:   "medium",
		ClientIP:   getClientIPForLogging(r),
		UserAgent:  r.UserAgent(),
		RequestURI: r.RequestURI,
		Method:     r.Method,
		Message:    "Rate limit exceeded",
		Details: map[string]interface{}{
			"limit":    limit,
			"duration": duration.String(),
		},
		Action: "blocked",
	}
	sl.LogEvent(event)
}

// LogCSRFFailure logs CSRF token validation failures
func (sl *SecurityLogger) LogCSRFFailure(r *http.Request, reason string) {
	event := SecurityEvent{
		EventType:  EventCSRFFailure,
		Severity:   "high",
		ClientIP:   getClientIPForLogging(r),
		UserAgent:  r.UserAgent(),
		RequestURI: r.RequestURI,
		Method:     r.Method,
		Message:    "CSRF validation failed",
		Details: map[string]interface{}{
			"reason":    reason,
			"referer":   r.Header.Get("Referer"),
			"origin":    r.Header.Get("Origin"),
		},
		Action: "blocked",
	}
	sl.LogEvent(event)
}

// LogSuspiciousRequest logs requests that match suspicious patterns
func (sl *SecurityLogger) LogSuspiciousRequest(r *http.Request, reason string, severity string) {
	event := SecurityEvent{
		EventType:  EventSuspiciousRequest,
		Severity:   severity,
		ClientIP:   getClientIPForLogging(r),
		UserAgent:  r.UserAgent(),
		RequestURI: r.RequestURI,
		Method:     r.Method,
		Message:    "Suspicious request detected",
		Details: map[string]interface{}{
			"reason": reason,
		},
		Action: "monitored",
	}
	sl.LogEvent(event)
}

// LogInvalidOrigin logs invalid CORS origins
func (sl *SecurityLogger) LogInvalidOrigin(r *http.Request, origin string) {
	event := SecurityEvent{
		EventType:  EventInvalidOrigin,
		Severity:   "medium",
		ClientIP:   getClientIPForLogging(r),
		UserAgent:  r.UserAgent(),
		RequestURI: r.RequestURI,
		Method:     r.Method,
		Message:    "Invalid origin in CORS request",
		Details: map[string]interface{}{
			"origin":          origin,
			"allowed_origins": "check_configuration",
		},
		Action: "blocked",
	}
	sl.LogEvent(event)
}

// LogIPBlocked logs blocked IP addresses
func (sl *SecurityLogger) LogIPBlocked(r *http.Request, reason string) {
	event := SecurityEvent{
		EventType:  EventIPBlocked,
		Severity:   "high",
		ClientIP:   getClientIPForLogging(r),
		UserAgent:  r.UserAgent(),
		RequestURI: r.RequestURI,
		Method:     r.Method,
		Message:    "IP address blocked",
		Details: map[string]interface{}{
			"reason": reason,
		},
		Action: "blocked",
	}
	sl.LogEvent(event)
}

// LogAuthFailure logs authentication failures
func (sl *SecurityLogger) LogAuthFailure(r *http.Request, userID string, reason string) {
	event := SecurityEvent{
		EventType:  EventAuthFailure,
		Severity:   "medium",
		ClientIP:   getClientIPForLogging(r),
		UserAgent:  r.UserAgent(),
		RequestURI: r.RequestURI,
		Method:     r.Method,
		UserID:     userID,
		Message:    "Authentication failed",
		Details: map[string]interface{}{
			"reason": reason,
		},
		Action: "blocked",
	}
	sl.LogEvent(event)
}

// LogSQLInjectionAttempt logs potential SQL injection attempts
func (sl *SecurityLogger) LogSQLInjectionAttempt(r *http.Request, payload string) {
	event := SecurityEvent{
		EventType:  EventSQLInjectionAttempt,
		Severity:   "critical",
		ClientIP:   getClientIPForLogging(r),
		UserAgent:  r.UserAgent(),
		RequestURI: r.RequestURI,
		Method:     r.Method,
		Message:    "Potential SQL injection attempt detected",
		Details: map[string]interface{}{
			"payload": payload,
		},
		Action: "blocked",
	}
	sl.LogEvent(event)
}

// LogXSSAttempt logs potential XSS attempts
func (sl *SecurityLogger) LogXSSAttempt(r *http.Request, payload string) {
	event := SecurityEvent{
		EventType:  EventXSSAttempt,
		Severity:   "high",
		ClientIP:   getClientIPForLogging(r),
		UserAgent:  r.UserAgent(),
		RequestURI: r.RequestURI,
		Method:     r.Method,
		Message:    "Potential XSS attempt detected",
		Details: map[string]interface{}{
			"payload": payload,
		},
		Action: "blocked",
	}
	sl.LogEvent(event)
}

// LogPathTraversal logs potential path traversal attempts
func (sl *SecurityLogger) LogPathTraversal(r *http.Request, path string) {
	event := SecurityEvent{
		EventType:  EventPathTraversal,
		Severity:   "high",
		ClientIP:   getClientIPForLogging(r),
		UserAgent:  r.UserAgent(),
		RequestURI: r.RequestURI,
		Method:     r.Method,
		Message:    "Potential path traversal attempt detected",
		Details: map[string]interface{}{
			"attempted_path": path,
		},
		Action: "blocked",
	}
	sl.LogEvent(event)
}

// getClientIPForLogging safely extracts client IP for logging purposes
func getClientIPForLogging(r *http.Request) string {
	// Use the secure getClientIP function but fallback to headers for logging visibility
	ip := getClientIP(r)
	
	// Also include forwarded headers in a safe way for analysis
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return ip + " (XFF: " + xff + ")"
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return ip + " (XRI: " + xri + ")"
	}
	
	return ip
}

// SecurityLoggerMiddleware creates middleware that logs security events
func SecurityLoggerMiddleware(logger *SecurityLogger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add logger to request context for use by other middleware
			// (This would require context modifications not shown here)
			
			next.ServeHTTP(w, r)
		})
	}
}

// Global security logger instance (can be configured per application)
var DefaultSecurityLogger = NewSecurityLogger(nil)