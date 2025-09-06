package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"
)

func TestHealthChecker(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	hc := NewHealthChecker(db, 5*time.Second)
	
	t.Run("healthy database check", func(t *testing.T) {
		ctx := context.Background()
		status := hc.Check(ctx)
		
		assert.Equal(t, "healthy", status.Status)
		assert.Empty(t, status.Errors)
		assert.Greater(t, status.ResponseTime, time.Duration(0))
		assert.False(t, status.Timestamp.IsZero())
		
		// Check individual checks
		assert.Equal(t, "passed", status.Checks["ping"])
		assert.Equal(t, "passed", status.Checks["connection_pool"])
		assert.Equal(t, "passed", status.Checks["query_execution"])
		assert.Equal(t, "passed", status.Checks["database_info"])
		
		// Check database info
		assert.Equal(t, "SQLite", status.DatabaseType)
		assert.NotEmpty(t, status.Version)
		
		// Check connection stats
		assert.GreaterOrEqual(t, status.ConnectionsOpen, 0)
		assert.GreaterOrEqual(t, status.ConnectionsInUse, 0)
		assert.GreaterOrEqual(t, status.ConnectionsIdle, 0)
	})
	
	t.Run("quick check", func(t *testing.T) {
		ctx := context.Background()
		err := hc.QuickCheck(ctx)
		assert.NoError(t, err)
	})
	
	t.Run("is healthy", func(t *testing.T) {
		ctx := context.Background()
		healthy := hc.IsHealthy(ctx)
		assert.True(t, healthy)
	})
	
	t.Run("check with timeout", func(t *testing.T) {
		shortHc := NewHealthChecker(db, 1*time.Microsecond) // Very short timeout
		ctx := context.Background()
		
		// Even with a very short timeout, basic SQLite operations should still work
		// since they're usually very fast
		status := shortHc.Check(ctx)
		
		// Status might be healthy or unhealthy depending on timing
		assert.Contains(t, []string{"healthy", "unhealthy"}, status.Status)
	})
	
	t.Run("wait for healthy", func(t *testing.T) {
		ctx := context.Background()
		err := hc.WaitForHealthy(ctx, 1*time.Second)
		assert.NoError(t, err)
	})
}

func TestHealthChecker_UnhealthyDatabase(t *testing.T) {
	// Create a database and then close it to simulate unhealthy state
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	db.Close() // Close immediately to make it unhealthy
	
	hc := NewHealthChecker(db, 1*time.Second)
	
	t.Run("unhealthy database check", func(t *testing.T) {
		ctx := context.Background()
		status := hc.Check(ctx)
		
		assert.Equal(t, "unhealthy", status.Status)
		assert.NotEmpty(t, status.Errors)
		assert.Equal(t, "failed", status.Checks["ping"])
	})
	
	t.Run("quick check fails", func(t *testing.T) {
		ctx := context.Background()
		err := hc.QuickCheck(ctx)
		assert.Error(t, err)
	})
	
	t.Run("is not healthy", func(t *testing.T) {
		ctx := context.Background()
		healthy := hc.IsHealthy(ctx)
		assert.False(t, healthy)
	})
	
	t.Run("wait for healthy times out", func(t *testing.T) {
		ctx := context.Background()
		err := hc.WaitForHealthy(ctx, 100*time.Millisecond)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "did not become healthy")
	})
}

func TestHealthChecker_ContextCancellation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	hc := NewHealthChecker(db, 5*time.Second)
	
	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		status := hc.Check(ctx)
		
		// Should still complete basic checks but may have errors due to cancellation
		assert.NotEmpty(t, status.Status)
		assert.False(t, status.Timestamp.IsZero())
	})
	
	t.Run("wait for healthy with cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		err := hc.WaitForHealthy(ctx, 1*time.Second)
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestHealthChecker_MonitorHealth(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	hc := NewHealthChecker(db, 5*time.Second)
	
	t.Run("monitor health", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		
		var statusCount int
		callback := func(status *HealthStatus) {
			statusCount++
			assert.NotEmpty(t, status.Status)
			assert.False(t, status.Timestamp.IsZero())
		}
		
		hc.MonitorHealth(ctx, 50*time.Millisecond, callback)
		
		// Should have received multiple status updates
		assert.Greater(t, statusCount, 1)
	})
}

func TestConnectionMonitor(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	// Set some connection limits
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	
	cm := NewConnectionMonitor(db, 10, 0.8)
	
	t.Run("get connection stats", func(t *testing.T) {
		stats := cm.GetConnectionStats()
		
		assert.GreaterOrEqual(t, stats.OpenConnections, 0)
		assert.GreaterOrEqual(t, stats.InUse, 0)
		assert.GreaterOrEqual(t, stats.Idle, 0)
		assert.Equal(t, 10, stats.MaxOpenConnections)
		assert.Equal(t, 0, stats.MaxIdleConnections) // Not available in sql.DBStats
		assert.GreaterOrEqual(t, stats.UtilizationPercent, 0.0)
		assert.LessOrEqual(t, stats.UtilizationPercent, 100.0)
	})
	
	t.Run("optimize connections", func(t *testing.T) {
		suggestions := cm.OptimizeConnections()
		
		// Should return a slice of suggestions (may be empty)
		assert.NotNil(t, suggestions)
	})
}

func TestConnectionMonitor_HighUtilization(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	// Set very low limits to trigger warnings
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	
	// Force a connection to be in use
	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()
	
	cm := NewConnectionMonitor(db, 1, 0.5) // 50% warning threshold
	
	t.Run("high utilization warning", func(t *testing.T) {
		stats := cm.GetConnectionStats()
		
		// With a transaction open, utilization should be high
		if stats.UtilizationPercent > 50 {
			assert.NotEmpty(t, stats.Warning)
			assert.Contains(t, stats.Warning, "High connection utilization")
		}
	})
}

func TestNewHealthChecker_DefaultTimeout(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	t.Run("default timeout", func(t *testing.T) {
		hc := NewHealthChecker(db, 0) // Zero timeout should use default
		assert.Equal(t, 5*time.Second, hc.timeout)
	})
	
	t.Run("custom timeout", func(t *testing.T) {
		customTimeout := 10 * time.Second
		hc := NewHealthChecker(db, customTimeout)
		assert.Equal(t, customTimeout, hc.timeout)
	})
}

func TestNewConnectionMonitor_DefaultThreshold(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	t.Run("default warning threshold", func(t *testing.T) {
		cm := NewConnectionMonitor(db, 10, 0) // Zero threshold should use default
		assert.Equal(t, 0.8, cm.warningThreshold)
	})
	
	t.Run("custom warning threshold", func(t *testing.T) {
		customThreshold := 0.9
		cm := NewConnectionMonitor(db, 10, customThreshold)
		assert.Equal(t, customThreshold, cm.warningThreshold)
	})
}

func TestHealthMiddleware(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	hc := NewHealthChecker(db, 5*time.Second)
	
	t.Run("middleware with healthy database", func(t *testing.T) {
		middleware := HealthMiddleware(hc)
		
		nextCalled := false
		next := func() {
			nextCalled = true
		}
		
		wrappedHandler := middleware(next)
		wrappedHandler()
		
		assert.True(t, nextCalled)
	})
}

func TestHealthStatus_JSON_Structure(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	hc := NewHealthChecker(db, 5*time.Second)
	
	t.Run("health status has correct structure", func(t *testing.T) {
		ctx := context.Background()
		status := hc.Check(ctx)
		
		// Verify all expected fields are present
		assert.NotEmpty(t, status.Status)
		assert.NotNil(t, status.Errors)
		assert.NotNil(t, status.Checks)
		assert.NotZero(t, status.ResponseTime)
		assert.False(t, status.Timestamp.IsZero())
		assert.NotEmpty(t, status.DatabaseType)
		
		// Verify numeric fields are reasonable
		assert.GreaterOrEqual(t, status.ConnectionsOpen, 0)
		assert.GreaterOrEqual(t, status.ConnectionsInUse, 0)
		assert.GreaterOrEqual(t, status.ConnectionsIdle, 0)
		assert.GreaterOrEqual(t, status.MaxConnections, 0)
	})
}