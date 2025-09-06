package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// HealthChecker provides database health monitoring functionality
type HealthChecker struct {
	db      *sql.DB
	timeout time.Duration
}

// NewHealthChecker creates a new database health checker
func NewHealthChecker(db *sql.DB, timeout time.Duration) *HealthChecker {
	if timeout == 0 {
		timeout = 5 * time.Second // Default timeout
	}
	
	return &HealthChecker{
		db:      db,
		timeout: timeout,
	}
}

// HealthStatus represents the health status of the database
type HealthStatus struct {
	Status           string            `json:"status"`
	ResponseTime     time.Duration     `json:"response_time"`
	ConnectionsOpen  int               `json:"connections_open"`
	ConnectionsInUse int               `json:"connections_in_use"`
	ConnectionsIdle  int               `json:"connections_idle"`
	MaxConnections   int               `json:"max_connections"`
	Errors           []string          `json:"errors,omitempty"`
	Checks           map[string]string `json:"checks"`
	Timestamp        time.Time         `json:"timestamp"`
	DatabaseType     string            `json:"database_type"`
	Version          string            `json:"version"`
}

// Check performs comprehensive health checks on the database
func (hc *HealthChecker) Check(ctx context.Context) *HealthStatus {
	status := &HealthStatus{
		Status:    "healthy",
		Errors:    []string{},
		Checks:    make(map[string]string),
		Timestamp: time.Now(),
	}
	
	start := time.Now()
	
	// Create context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, hc.timeout)
	defer cancel()
	
	// Check 1: Basic connectivity (ping)
	if err := hc.checkPing(checkCtx, status); err != nil {
		status.Status = "unhealthy"
		status.Errors = append(status.Errors, fmt.Sprintf("ping failed: %v", err))
		status.Checks["ping"] = "failed"
	} else {
		status.Checks["ping"] = "passed"
	}
	
	// Check 2: Connection pool stats
	if err := hc.checkConnectionPool(status); err != nil {
		status.Errors = append(status.Errors, fmt.Sprintf("connection pool check failed: %v", err))
		status.Checks["connection_pool"] = "failed"
	} else {
		status.Checks["connection_pool"] = "passed"
	}
	
	// Check 3: Simple query execution
	if err := hc.checkSimpleQuery(checkCtx, status); err != nil {
		status.Status = "unhealthy"
		status.Errors = append(status.Errors, fmt.Sprintf("query execution failed: %v", err))
		status.Checks["query_execution"] = "failed"
	} else {
		status.Checks["query_execution"] = "passed"
	}
	
	// Check 4: Database version and type
	if err := hc.checkDatabaseInfo(checkCtx, status); err != nil {
		status.Errors = append(status.Errors, fmt.Sprintf("database info check failed: %v", err))
		status.Checks["database_info"] = "failed"
	} else {
		status.Checks["database_info"] = "passed"
	}
	
	status.ResponseTime = time.Since(start)
	
	// Set status based on critical errors
	if len(status.Errors) > 0 {
		// If only non-critical errors, mark as degraded
		criticalErrors := 0
		for check, result := range status.Checks {
			if result == "failed" && (check == "ping" || check == "query_execution") {
				criticalErrors++
			}
		}
		
		if criticalErrors > 0 {
			status.Status = "unhealthy"
		} else {
			status.Status = "degraded"
		}
	}
	
	return status
}

// checkPing tests basic database connectivity
func (hc *HealthChecker) checkPing(ctx context.Context, status *HealthStatus) error {
	return hc.db.PingContext(ctx)
}

// checkConnectionPool examines connection pool statistics
func (hc *HealthChecker) checkConnectionPool(status *HealthStatus) error {
	stats := hc.db.Stats()
	
	status.ConnectionsOpen = stats.OpenConnections
	status.ConnectionsInUse = stats.InUse
	status.ConnectionsIdle = stats.Idle
	status.MaxConnections = stats.MaxOpenConnections
	
	// Check for potential issues
	if stats.MaxOpenConnections > 0 && stats.OpenConnections >= stats.MaxOpenConnections {
		return fmt.Errorf("connection pool exhausted: %d/%d connections in use", 
			stats.OpenConnections, stats.MaxOpenConnections)
	}
	
	return nil
}

// checkSimpleQuery tests query execution capability
func (hc *HealthChecker) checkSimpleQuery(ctx context.Context, status *HealthStatus) error {
	// Try a simple SELECT 1 query that should work on most databases
	var result int
	err := hc.db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return err
	}
	
	if result != 1 {
		return fmt.Errorf("unexpected query result: expected 1, got %d", result)
	}
	
	return nil
}

// checkDatabaseInfo retrieves database type and version information
func (hc *HealthChecker) checkDatabaseInfo(ctx context.Context, status *HealthStatus) error {
	// Try to determine database type and version
	// This is database-specific, so we'll try common queries
	
	// Try PostgreSQL version query
	var version string
	err := hc.db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err == nil {
		status.DatabaseType = "PostgreSQL"
		status.Version = version
		return nil
	}
	
	// Try MySQL version query
	err = hc.db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version)
	if err == nil {
		status.DatabaseType = "MySQL"
		status.Version = version
		return nil
	}
	
	// Try SQLite version query
	err = hc.db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version)
	if err == nil {
		status.DatabaseType = "SQLite"
		status.Version = version
		return nil
	}
	
	// If all specific queries fail, just mark as unknown
	status.DatabaseType = "Unknown"
	status.Version = "Unknown"
	
	return nil // Not a critical error
}

// QuickCheck performs a basic ping test
func (hc *HealthChecker) QuickCheck(ctx context.Context) error {
	checkCtx, cancel := context.WithTimeout(ctx, hc.timeout)
	defer cancel()
	
	return hc.db.PingContext(checkCtx)
}

// IsHealthy returns true if the database is healthy
func (hc *HealthChecker) IsHealthy(ctx context.Context) bool {
	status := hc.Check(ctx)
	return status.Status == "healthy"
}

// WaitForHealthy waits for the database to become healthy
func (hc *HealthChecker) WaitForHealthy(ctx context.Context, maxWait time.Duration) error {
	deadline := time.Now().Add(maxWait)
	
	for time.Now().Before(deadline) {
		if hc.IsHealthy(ctx) {
			return nil
		}
		
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			// Continue checking
		}
	}
	
	return fmt.Errorf("database did not become healthy within %v", maxWait)
}

// MonitorHealth continuously monitors database health
func (hc *HealthChecker) MonitorHealth(ctx context.Context, interval time.Duration, callback func(*HealthStatus)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	// Initial check
	status := hc.Check(ctx)
	callback(status)
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status := hc.Check(ctx)
			callback(status)
		}
	}
}

// ConnectionMonitor provides advanced connection monitoring
type ConnectionMonitor struct {
	db               *sql.DB
	maxConnections   int
	warningThreshold float64 // Percentage of max connections that triggers warning
}

// NewConnectionMonitor creates a new connection monitor
func NewConnectionMonitor(db *sql.DB, maxConnections int, warningThreshold float64) *ConnectionMonitor {
	if warningThreshold == 0 {
		warningThreshold = 0.8 // 80% by default
	}
	
	return &ConnectionMonitor{
		db:               db,
		maxConnections:   maxConnections,
		warningThreshold: warningThreshold,
	}
}

// ConnectionStats provides detailed connection statistics
type ConnectionStats struct {
	OpenConnections     int     `json:"open_connections"`
	InUse              int     `json:"in_use"`
	Idle               int     `json:"idle"`
	MaxOpenConnections int     `json:"max_open_connections"`
	MaxIdleConnections int     `json:"max_idle_connections"`
	MaxLifetime        string  `json:"max_lifetime"`
	MaxIdleTime        string  `json:"max_idle_time"`
	UtilizationPercent float64 `json:"utilization_percent"`
	Warning            string  `json:"warning,omitempty"`
}

// GetConnectionStats returns detailed connection statistics
func (cm *ConnectionMonitor) GetConnectionStats() *ConnectionStats {
	stats := cm.db.Stats()
	
	utilizationPercent := 0.0
	if stats.MaxOpenConnections > 0 {
		utilizationPercent = float64(stats.OpenConnections) / float64(stats.MaxOpenConnections) * 100
	}
	
	connStats := &ConnectionStats{
		OpenConnections:     stats.OpenConnections,
		InUse:              stats.InUse,
		Idle:               stats.Idle,
		MaxOpenConnections: stats.MaxOpenConnections,
		MaxIdleConnections: 0, // Not available in sql.DBStats
		UtilizationPercent: utilizationPercent,
	}
	
	// Add warning if utilization is high
	if utilizationPercent > cm.warningThreshold*100 {
		connStats.Warning = fmt.Sprintf("High connection utilization: %.1f%%", utilizationPercent)
	}
	
	return connStats
}

// OptimizeConnections suggests connection pool optimizations
func (cm *ConnectionMonitor) OptimizeConnections() []string {
	stats := cm.db.Stats()
	var suggestions []string
	
	// High idle connections
	if stats.Idle > stats.InUse*2 {
		suggestions = append(suggestions, 
			"Consider reducing MaxIdleConns - too many idle connections")
	}
	
	// Frequent connection creation/destruction
	if stats.MaxOpenConnections > 0 && stats.OpenConnections < stats.MaxOpenConnections/2 {
		suggestions = append(suggestions, 
			"Consider reducing MaxOpenConns - connection limit is too high")
	}
	
	// No idle connections but high usage
	if stats.Idle == 0 && stats.InUse > 10 {
		suggestions = append(suggestions, 
			"Consider increasing MaxIdleConns - frequent connection creation detected")
	}
	
	return suggestions
}

// HealthMiddleware creates HTTP middleware for database health checks
func HealthMiddleware(hc *HealthChecker) func(next func()) func() {
	return func(next func()) func() {
		return func() {
			ctx := context.Background()
			if !hc.IsHealthy(ctx) {
				// In a real HTTP middleware, this would return 503 Service Unavailable
				// For now, we'll just log the issue
				status := hc.Check(ctx)
				fmt.Printf("Database unhealthy: %v\n", status.Errors)
			}
			next()
		}
	}
}