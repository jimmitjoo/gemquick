package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVersionRouter(t *testing.T) {
	config := &VersionConfig{
		DefaultVersion:    "v1",
		SupportedVersions: []string{"v1", "v2"},
		VersionHeader:     "X-API-Version",
		VersionInPath:     true,
		VersionInQuery:    false,
		Deprecated:        map[string]string{"v1": "2024-12-31"},
	}

	vr := NewVersionRouter(config)

	// Register v1 routes
	v1Router := vr.RegisterVersion("v1")
	v1Router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("v1 response"))
	})

	// Register v2 routes
	v2Router := vr.RegisterVersion("v2")
	v2Router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("v2 response"))
	})

	tests := []struct {
		name           string
		path           string
		header         string
		wantVersion    string
		wantResponse   string
		wantDeprecated bool
	}{
		{
			name:         "v1 from path",
			path:         "/v1/test",
			wantVersion:  "v1",
			wantResponse: "v1 response",
			wantDeprecated: true,
		},
		{
			name:         "v2 from path",
			path:         "/v2/test",
			wantVersion:  "v2",
			wantResponse: "v2 response",
			wantDeprecated: false,
		},
		{
			name:         "v2 from header",
			path:         "/test",
			header:       "v2",
			wantVersion:  "v2",
			wantResponse: "v2 response",
			wantDeprecated: false,
		},
		{
			name:         "default version",
			path:         "/test",
			wantVersion:  "v1",
			wantResponse: "v1 response",
			wantDeprecated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", tt.path, nil)
			if tt.header != "" {
				r.Header.Set("X-API-Version", tt.header)
			}

			vr.ServeHTTP(w, r)

			if got := w.Header().Get("X-API-Version"); got != tt.wantVersion {
				t.Errorf("VersionRouter version header = %v, want %v", got, tt.wantVersion)
			}

			if tt.wantDeprecated {
				if sunset := w.Header().Get("Sunset"); sunset == "" {
					t.Error("VersionRouter deprecated version missing Sunset header")
				}
				if deprecation := w.Header().Get("Deprecation"); deprecation != "true" {
					t.Error("VersionRouter deprecated version missing Deprecation header")
				}
			}

			if body := w.Body.String(); body != tt.wantResponse {
				t.Errorf("VersionRouter response = %v, want %v", body, tt.wantResponse)
			}
		})
	}
}

func TestGetVersion(t *testing.T) {
	tests := []struct {
		name          string
		config        *VersionConfig
		path          string
		header        string
		query         string
		wantVersion   string
	}{
		{
			name: "path priority",
			config: &VersionConfig{
				DefaultVersion:    "v1",
				SupportedVersions: []string{"v1", "v2", "v3"},
				VersionHeader:     "X-API-Version",
				VersionInPath:     true,
				VersionInQuery:    true,
			},
			path:        "/v2/users",
			header:      "v3",
			query:       "?version=v1",
			wantVersion: "v2", // Path has highest priority
		},
		{
			name: "header priority over query",
			config: &VersionConfig{
				DefaultVersion:    "v1",
				SupportedVersions: []string{"v1", "v2"},
				VersionHeader:     "X-API-Version",
				VersionInPath:     false,
				VersionInQuery:    true,
			},
			header:      "v2",
			query:       "?version=v1",
			wantVersion: "v2", // Header has priority over query
		},
		{
			name: "query parameter",
			config: &VersionConfig{
				DefaultVersion:    "v1",
				SupportedVersions: []string{"v1", "v2"},
				VersionInPath:     false,
				VersionInQuery:    true,
			},
			query:       "?v=2",
			wantVersion: "v2",
		},
		{
			name: "unsupported version falls back to default",
			config: &VersionConfig{
				DefaultVersion:    "v1",
				SupportedVersions: []string{"v1", "v2"},
				VersionHeader:     "X-API-Version",
			},
			header:      "v99",
			wantVersion: "v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vr := NewVersionRouter(tt.config)
			
			path := tt.path
			if path == "" {
				path = "/"
			}
			url := path + tt.query
			r := httptest.NewRequest("GET", url, nil)
			if tt.header != "" {
				r.Header.Set(tt.config.VersionHeader, tt.header)
			}

			version := vr.GetVersion(r)
			if version != tt.wantVersion {
				t.Errorf("GetVersion() = %v, want %v", version, tt.wantVersion)
			}
		})
	}
}

func TestDeprecateVersion(t *testing.T) {
	vr := NewVersionRouter(DefaultVersionConfig())
	
	vr.DeprecateVersion("v1", "2024-12-31")
	
	if sunset, ok := vr.config.Deprecated["v1"]; !ok || sunset != "2024-12-31" {
		t.Errorf("DeprecateVersion() failed to set deprecation")
	}
}

func TestExtractVersionFromPath(t *testing.T) {
	tests := []struct {
		path    string
		want    string
	}{
		{"/v1/users", "v1"},
		{"/api/v2/products", "v2"},
		{"/v3/", "v3"},
		{"/users", ""},
		{"/version/users", ""},
		{"/v1/v2/nested", "v1"}, // First match
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := extractVersionFromPath(tt.path)
			if got != tt.want {
				t.Errorf("extractVersionFromPath(%v) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestStripVersionFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/v1/users", "/users"},
		{"/api/v2/products", "/api/products"},
		{"/v3/", "/"},
		{"/users", "/users"},
		{"/v1/v2/nested", "/v2/nested"}, // Only first is stripped
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := stripVersionFromPath(tt.path)
			if got != tt.want {
				t.Errorf("stripVersionFromPath(%v) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1", "v1"},
		{"2", "v2"},
		{"v1", "v1"},
		{"V2", "v2"},
		{" v3 ", "v3"},
		{"1.0", "v1"},
		{"2.5.3", "v2"},
		{"v1.2.3", "v1"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeVersion(tt.input)
			if got != tt.want {
				t.Errorf("normalizeVersion(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestVersionNegotiator(t *testing.T) {
	vn := NewVersionNegotiator("v1")
	
	vn.AddVersion("v1", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("v1 handler"))
	})
	
	vn.AddVersion("v2", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("v2 handler"))
	})

	tests := []struct {
		name       string
		header     string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "v1 explicit",
			header:     "v1",
			wantStatus: http.StatusOK,
			wantBody:   "v1 handler",
		},
		{
			name:       "v2 explicit",
			header:     "v2",
			wantStatus: http.StatusOK,
			wantBody:   "v2 handler",
		},
		{
			name:       "default version",
			header:     "",
			wantStatus: http.StatusOK,
			wantBody:   "v1 handler",
		},
		{
			name:       "unsupported version",
			header:     "v3",
			wantStatus: http.StatusNotImplemented,
			wantBody:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				r.Header.Set("X-API-Version", tt.header)
			}

			vn.ServeHTTP(w, r)

			if w.Code != tt.wantStatus {
				t.Errorf("VersionNegotiator status = %v, want %v", w.Code, tt.wantStatus)
			}

			if tt.wantBody != "" && w.Body.String() != tt.wantBody {
				t.Errorf("VersionNegotiator body = %v, want %v", w.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestVersionMiddleware(t *testing.T) {
	config := &VersionConfig{
		DefaultVersion:    "v1",
		SupportedVersions: []string{"v1", "v2"},
		VersionHeader:     "X-API-Version",
		VersionInPath:     true,
		Deprecated:        map[string]string{"v1": "2024-12-31"},
	}

	handler := VersionMiddleware(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		version := r.Context().Value(ContextKeyAPIVersion)
		if version == nil {
			t.Error("VersionMiddleware didn't set version in context")
		}
		w.Write([]byte(version.(string)))
	}))

	tests := []struct {
		name           string
		path           string
		wantVersion    string
		wantDeprecated bool
	}{
		{
			name:           "v1 deprecated",
			path:           "/v1/test",
			wantVersion:    "v1",
			wantDeprecated: true,
		},
		{
			name:           "v2 not deprecated",
			path:           "/v2/test",
			wantVersion:    "v2",
			wantDeprecated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", tt.path, nil)

			handler.ServeHTTP(w, r)

			if body := w.Body.String(); body != tt.wantVersion {
				t.Errorf("VersionMiddleware version = %v, want %v", body, tt.wantVersion)
			}

			if tt.wantDeprecated {
				if sunset := w.Header().Get("Sunset"); sunset == "" {
					t.Error("VersionMiddleware missing Sunset header for deprecated version")
				}
			}
		})
	}
}

func TestMount(t *testing.T) {
	config := &VersionConfig{
		DefaultVersion:    "v1",
		SupportedVersions: []string{"v1", "v2"},
		VersionInPath:     true,
	}

	vr := NewVersionRouter(config)
	
	// Register versions
	vr.RegisterVersion("v1")
	vr.RegisterVersion("v2")

	// Mount a handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("mounted"))
	})
	
	vr.Mount("/api", testHandler)

	// Test that handler is mounted on both versions
	tests := []string{"/v1/api", "/v2/api"}
	
	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", path, nil)
			
			vr.ServeHTTP(w, r)
			
			if body := w.Body.String(); body != "mounted" {
				t.Errorf("Mount() path %v = %v, want 'mounted'", path, body)
			}
		})
	}
}