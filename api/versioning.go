package api

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// VersionConfig holds versioning configuration
type VersionConfig struct {
	DefaultVersion   string
	SupportedVersions []string
	VersionHeader    string
	VersionInPath    bool
	VersionInQuery   bool
	Deprecated       map[string]string // Maps deprecated versions to sunset dates
}

// DefaultVersionConfig returns a default version configuration
func DefaultVersionConfig() *VersionConfig {
	return &VersionConfig{
		DefaultVersion:    "v1",
		SupportedVersions: []string{"v1"},
		VersionHeader:     "X-API-Version",
		VersionInPath:     true,
		VersionInQuery:    false,
		Deprecated:        make(map[string]string),
	}
}

// VersionRouter manages API versioning
type VersionRouter struct {
	config   *VersionConfig
	routers  map[string]*chi.Mux
	handlers map[string]map[string]http.HandlerFunc
}

// NewVersionRouter creates a new version router
func NewVersionRouter(config *VersionConfig) *VersionRouter {
	if config == nil {
		config = DefaultVersionConfig()
	}
	
	return &VersionRouter{
		config:   config,
		routers:  make(map[string]*chi.Mux),
		handlers: make(map[string]map[string]http.HandlerFunc),
	}
}

// RegisterVersion registers a new API version
func (vr *VersionRouter) RegisterVersion(version string) *chi.Mux {
	if vr.routers[version] == nil {
		vr.routers[version] = chi.NewRouter()
		vr.handlers[version] = make(map[string]http.HandlerFunc)
		
		// Add version to supported versions if not present
		found := false
		for _, v := range vr.config.SupportedVersions {
			if v == version {
				found = true
				break
			}
		}
		if !found {
			vr.config.SupportedVersions = append(vr.config.SupportedVersions, version)
		}
	}
	
	return vr.routers[version]
}

// DeprecateVersion marks a version as deprecated
func (vr *VersionRouter) DeprecateVersion(version, sunsetDate string) {
	vr.config.Deprecated[version] = sunsetDate
}

// GetVersion extracts API version from request
func (vr *VersionRouter) GetVersion(r *http.Request) string {
	var version string
	
	// Check path first (highest priority)
	if vr.config.VersionInPath {
		// Extract version from path like /v1/users or /api/v2/users
		pathVersion := extractVersionFromPath(r.URL.Path)
		if pathVersion != "" {
			version = pathVersion
		}
	}
	
	// Check header (medium priority)
	if version == "" && vr.config.VersionHeader != "" {
		headerVersion := r.Header.Get(vr.config.VersionHeader)
		if headerVersion != "" {
			version = normalizeVersion(headerVersion)
		}
	}
	
	// Check query parameter (lowest priority)
	if version == "" && vr.config.VersionInQuery {
		queryVersion := r.URL.Query().Get("version")
		if queryVersion == "" {
			queryVersion = r.URL.Query().Get("v")
		}
		if queryVersion != "" {
			version = normalizeVersion(queryVersion)
		}
	}
	
	// Use default version if none specified
	if version == "" {
		version = vr.config.DefaultVersion
	}
	
	// Validate version is supported
	if !vr.isVersionSupported(version) {
		return vr.config.DefaultVersion
	}
	
	return version
}

// ServeHTTP implements http.Handler
func (vr *VersionRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	version := vr.GetVersion(r)
	
	// Add deprecation warning if applicable
	if sunsetDate, deprecated := vr.config.Deprecated[version]; deprecated {
		w.Header().Set("Sunset", sunsetDate)
		w.Header().Set("Deprecation", "true")
		w.Header().Set("Link", fmt.Sprintf("</api/%s>; rel=\"successor-version\"", vr.config.DefaultVersion))
	}
	
	// Set version header in response
	w.Header().Set(vr.config.VersionHeader, version)
	
	// Route to appropriate version handler
	if router, exists := vr.routers[version]; exists {
		// Strip version from path if it's in the path
		if vr.config.VersionInPath {
			r.URL.Path = stripVersionFromPath(r.URL.Path)
			rctx := chi.NewRouteContext()
			rctx.Reset()
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
		}
		
		router.ServeHTTP(w, r)
	} else {
		// Version not found
		Error(w, http.StatusNotFound, "VERSION_NOT_FOUND", 
			fmt.Sprintf("API version '%s' is not supported", version),
			map[string]interface{}{
				"supported_versions": vr.config.SupportedVersions,
				"default_version":    vr.config.DefaultVersion,
			})
	}
}

// Mount mounts versioned API routes
func (vr *VersionRouter) Mount(pattern string, handler http.Handler) {
	for _, router := range vr.routers {
		// Don't add version prefix here since it's stripped in ServeHTTP
		router.Mount(pattern, handler)
	}
}

// isVersionSupported checks if a version is supported
func (vr *VersionRouter) isVersionSupported(version string) bool {
	for _, v := range vr.config.SupportedVersions {
		if v == version {
			return true
		}
	}
	return false
}

// extractVersionFromPath extracts version from URL path
func extractVersionFromPath(path string) string {
	// Match patterns like /v1/, /v2/, /api/v1/, etc.
	re := regexp.MustCompile(`/(v\d+)(/|$)`)
	matches := re.FindStringSubmatch(path)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// stripVersionFromPath removes version from URL path
func stripVersionFromPath(path string) string {
	// Remove patterns like /v1/, /v2/, etc.
	re := regexp.MustCompile(`/(v\d+)(/|$)`)
	return re.ReplaceAllString(path, "$2")
}

// normalizeVersion normalizes version string
func normalizeVersion(version string) string {
	version = strings.ToLower(strings.TrimSpace(version))
	
	// Add 'v' prefix if it's just a number
	if matched, _ := regexp.MatchString(`^\d+$`, version); matched {
		version = "v" + version
	}
	
	// Remove dots and convert to v1, v2 format
	if strings.Contains(version, ".") {
		parts := strings.Split(version, ".")
		if len(parts) > 0 {
			if major, err := strconv.Atoi(strings.TrimPrefix(parts[0], "v")); err == nil {
				version = fmt.Sprintf("v%d", major)
			}
		}
	}
	
	return version
}

// VersionNegotiator handles version negotiation
type VersionNegotiator struct {
	versions map[string]http.HandlerFunc
	defaultVersion string
}

// NewVersionNegotiator creates a new version negotiator
func NewVersionNegotiator(defaultVersion string) *VersionNegotiator {
	return &VersionNegotiator{
		versions: make(map[string]http.HandlerFunc),
		defaultVersion: defaultVersion,
	}
}

// AddVersion adds a version handler
func (vn *VersionNegotiator) AddVersion(version string, handler http.HandlerFunc) {
	vn.versions[version] = handler
}

// ServeHTTP negotiates and serves the appropriate version
func (vn *VersionNegotiator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	version := r.Header.Get("X-API-Version")
	if version == "" {
		version = vn.defaultVersion
	}
	
	if handler, exists := vn.versions[version]; exists {
		handler(w, r)
	} else {
		Error(w, http.StatusNotImplemented, "VERSION_NOT_IMPLEMENTED",
			fmt.Sprintf("Version %s is not implemented for this endpoint", version),
			map[string]interface{}{
				"available_versions": getMapKeys(vn.versions),
			})
	}
}

// getMapKeys returns keys from a map
func getMapKeys(m map[string]http.HandlerFunc) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// VersionMiddleware adds version to context and validates it
func VersionMiddleware(config *VersionConfig) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			vr := NewVersionRouter(config)
			version := vr.GetVersion(r)
			
			// Add version to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, ContextKeyAPIVersion, version)
			
			// Check if version is deprecated
			if sunsetDate, deprecated := config.Deprecated[version]; deprecated {
				w.Header().Set("Sunset", sunsetDate)
				w.Header().Set("Deprecation", "true")
			}
			
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}