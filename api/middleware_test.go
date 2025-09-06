package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestErrorHandler(t *testing.T) {
	handler := ErrorHandler(false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	// Set a request ID in context similar to what middleware would do
	ctx := context.WithValue(r.Context(), contextKey("request_id"), "test-123")
	r = r.WithContext(ctx)

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("ErrorHandler() status = %v, want %v", w.Code, http.StatusInternalServerError)
	}

	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error == nil || resp.Error.Code != "INTERNAL_ERROR" {
		t.Errorf("ErrorHandler() didn't produce proper error response")
	}
}

func TestContentTypeJSON(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		contentType string
		wantStatus  int
	}{
		{
			name:        "POST with JSON",
			method:      "POST",
			contentType: "application/json",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "POST with XML",
			method:      "POST",
			contentType: "application/xml",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "POST with form data",
			method:      "POST",
			contentType: "multipart/form-data",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "POST with invalid content type",
			method:      "POST",
			contentType: "text/plain",
			wantStatus:  http.StatusUnsupportedMediaType,
		},
		{
			name:        "GET request (no content type check)",
			method:      "GET",
			contentType: "",
			wantStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := ContentTypeJSON(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			w := httptest.NewRecorder()
			r := httptest.NewRequest(tt.method, "/", nil)
			if tt.contentType != "" {
				r.Header.Set("Content-Type", tt.contentType)
			}

			handler.ServeHTTP(w, r)

			if w.Code != tt.wantStatus {
				t.Errorf("ContentTypeJSON() status = %v, want %v", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestAPIVersion(t *testing.T) {
	handler := APIVersion("v1")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		version := GetAPIVersion(r.Context())
		if version != "v1" {
			t.Errorf("APIVersion() context version = %v, want v1", version)
		}
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	handler.ServeHTTP(w, r)

	if header := w.Header().Get("X-API-Version"); header != "v1" {
		t.Errorf("APIVersion() header = %v, want v1", header)
	}
}

func TestRequestTimer(t *testing.T) {
	handler := RequestTimer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := GetRequestStartTime(r.Context())
		if startTime.IsZero() {
			t.Error("RequestTimer() start time not in context")
		}
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	handler.ServeHTTP(w, r)

	responseTime := w.Header().Get("X-Response-Time")
	if responseTime == "" {
		t.Error("RequestTimer() X-Response-Time header not set")
	}

	if !strings.HasSuffix(responseTime, "ms") {
		t.Errorf("RequestTimer() X-Response-Time format = %v, want XXXms", responseTime)
	}
}

func TestCORS(t *testing.T) {
	tests := []struct {
		name           string
		allowedOrigins []string
		requestOrigin  string
		method         string
		wantAllow      bool
	}{
		{
			name:           "wildcard allows all",
			allowedOrigins: []string{"*"},
			requestOrigin:  "https://example.com",
			method:         "GET",
			wantAllow:      true,
		},
		{
			name:           "specific origin allowed",
			allowedOrigins: []string{"https://example.com", "https://other.com"},
			requestOrigin:  "https://example.com",
			method:         "GET",
			wantAllow:      true,
		},
		{
			name:           "origin not allowed",
			allowedOrigins: []string{"https://example.com"},
			requestOrigin:  "https://notallowed.com",
			method:         "GET",
			wantAllow:      false,
		},
		{
			name:           "OPTIONS preflight",
			allowedOrigins: []string{"*"},
			requestOrigin:  "https://example.com",
			method:         "OPTIONS",
			wantAllow:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := CORS(tt.allowedOrigins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			w := httptest.NewRecorder()
			r := httptest.NewRequest(tt.method, "/", nil)
			r.Header.Set("Origin", tt.requestOrigin)

			handler.ServeHTTP(w, r)

			allowOrigin := w.Header().Get("Access-Control-Allow-Origin")
			if tt.wantAllow {
				if allowOrigin != tt.requestOrigin {
					t.Errorf("CORS() Allow-Origin = %v, want %v", allowOrigin, tt.requestOrigin)
				}
				
				if tt.method == "OPTIONS" && w.Code != http.StatusNoContent {
					t.Errorf("CORS() OPTIONS status = %v, want %v", w.Code, http.StatusNoContent)
				}
			} else {
				if allowOrigin != "" {
					t.Errorf("CORS() Allow-Origin = %v, want empty", allowOrigin)
				}
			}
		})
	}
}

func TestSecureHeaders(t *testing.T) {
	tests := []struct {
		name   string
		tls    bool
		wantHSTS bool
	}{
		{
			name:     "HTTP request",
			tls:      false,
			wantHSTS: false,
		},
		{
			name:     "HTTPS request",
			tls:      true,
			wantHSTS: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := SecureHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/", nil)
			if tt.tls {
				r.TLS = &tls.ConnectionState{}
			}

			handler.ServeHTTP(w, r)

			headers := map[string]string{
				"X-Content-Type-Options": "nosniff",
				"X-Frame-Options":        "DENY",
				"X-XSS-Protection":       "1; mode=block",
				"Referrer-Policy":        "strict-origin-when-cross-origin",
			}

			for header, expected := range headers {
				if got := w.Header().Get(header); got != expected {
					t.Errorf("SecureHeaders() %s = %v, want %v", header, got, expected)
				}
			}

			hsts := w.Header().Get("Strict-Transport-Security")
			if tt.wantHSTS && hsts == "" {
				t.Error("SecureHeaders() HSTS header not set for HTTPS")
			}
			if !tt.wantHSTS && hsts != "" {
				t.Error("SecureHeaders() HSTS header set for HTTP")
			}
		})
	}
}

func TestJSONRequest(t *testing.T) {
	type testData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantError  bool
	}{
		{
			name:       "valid JSON",
			body:       `{"name":"test","value":42}`,
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "invalid JSON",
			body:       `{"name":"test","value":}`,
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name:       "unknown fields",
			body:       `{"name":"test","value":42,"unknown":"field"}`,
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name:       "empty body",
			body:       "",
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data testData
			handler := JSONRequest(&data)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			w := httptest.NewRecorder()
			var body *bytes.Buffer
			if tt.body != "" {
				body = bytes.NewBufferString(tt.body)
				r := httptest.NewRequest("POST", "/", body)
				handler.ServeHTTP(w, r)
			} else {
				r := httptest.NewRequest("POST", "/", nil)
				handler.ServeHTTP(w, r)
			}

			if w.Code != tt.wantStatus {
				t.Errorf("JSONRequest() status = %v, want %v", w.Code, tt.wantStatus)
			}

			if tt.wantError && w.Code == http.StatusOK {
				t.Error("JSONRequest() expected error but got success")
			}
		})
	}
}

func TestRequireAuth(t *testing.T) {
	tests := []struct {
		name       string
		authFunc   func(*http.Request) (bool, error)
		wantStatus int
	}{
		{
			name: "authorized",
			authFunc: func(r *http.Request) (bool, error) {
				return true, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "unauthorized",
			authFunc: func(r *http.Request) (bool, error) {
				return false, nil
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "auth error",
			authFunc: func(r *http.Request) (bool, error) {
				return false, http.ErrNotSupported
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := RequireAuth(tt.authFunc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/", nil)

			handler.ServeHTTP(w, r)

			if w.Code != tt.wantStatus {
				t.Errorf("RequireAuth() status = %v, want %v", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestGetAPIVersion(t *testing.T) {
	ctx := context.Background()
	
	// Test with no version
	version := GetAPIVersion(ctx)
	if version != "" {
		t.Errorf("GetAPIVersion() = %v, want empty", version)
	}

	// Test with version
	ctx = context.WithValue(ctx, ContextKeyAPIVersion, "v2")
	version = GetAPIVersion(ctx)
	if version != "v2" {
		t.Errorf("GetAPIVersion() = %v, want v2", version)
	}
}

func TestGetRequestStartTime(t *testing.T) {
	ctx := context.Background()
	
	// Test with no start time
	startTime := GetRequestStartTime(ctx)
	if !startTime.IsZero() {
		t.Errorf("GetRequestStartTime() = %v, want zero", startTime)
	}

	// Test with start time
	now := time.Now()
	ctx = context.WithValue(ctx, ContextKeyStartTime, now)
	startTime = GetRequestStartTime(ctx)
	if !startTime.Equal(now) {
		t.Errorf("GetRequestStartTime() = %v, want %v", startTime, now)
	}
}

func TestChainMiddleware(t *testing.T) {
	var order []string
	
	middleware1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m1-before")
			next.ServeHTTP(w, r)
			order = append(order, "m1-after")
		})
	}
	
	middleware2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m2-before")
			next.ServeHTTP(w, r)
			order = append(order, "m2-after")
		})
	}
	
	handler := ChainMiddleware(middleware1, middleware2)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	
	handler.ServeHTTP(w, r)

	expected := []string{"m1-before", "m2-before", "handler", "m2-after", "m1-after"}
	if len(order) != len(expected) {
		t.Fatalf("ChainMiddleware() order length = %v, want %v", len(order), len(expected))
	}
	
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("ChainMiddleware() order[%d] = %v, want %v", i, order[i], v)
		}
	}
}