package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobManager(t *testing.T) {
	manager := NewJobManager(nil)
	
	err := manager.Start()
	require.NoError(t, err)
	
	defer func() {
		err := manager.Stop()
		require.NoError(t, err)
	}()
	
	handlerCalled := false
	manager.RegisterHandlerFunc("test", func(ctx context.Context, job *Job) error {
		handlerCalled = true
		return nil
	})
	
	job := NewJob("test", "", nil)
	err = manager.Enqueue(job)
	require.NoError(t, err)
	
	time.Sleep(100 * time.Millisecond)
	assert.True(t, handlerCalled)
	
	stats := manager.GetManagerStats()
	assert.True(t, stats.IsRunning)
	assert.True(t, stats.TotalWorkers > 0)
}

func TestJobManagerEnqueueIn(t *testing.T) {
	manager := NewJobManager(nil)
	
	handlerCalled := false
	manager.RegisterHandlerFunc("delayed", func(ctx context.Context, job *Job) error {
		handlerCalled = true
		return nil
	})
	
	job := NewJob("delayed", "default", nil)
	err := manager.EnqueueIn(job, 50*time.Millisecond)
	require.NoError(t, err)
	
	assert.Equal(t, JobStatusScheduled, job.Status)
	assert.NotNil(t, job.ScheduledAt)
	
	time.Sleep(20 * time.Millisecond)
	assert.False(t, handlerCalled)
	
	err = manager.Start()
	require.NoError(t, err)
	defer manager.Stop()
	
	time.Sleep(100 * time.Millisecond)
	assert.True(t, handlerCalled)
}

func TestJobManagerEnqueueAt(t *testing.T) {
	manager := NewJobManager(nil)
	
	job := NewJob("scheduled", "default", nil)
	scheduledTime := time.Now().Add(time.Hour)
	
	err := manager.EnqueueAt(job, scheduledTime)
	require.NoError(t, err)
	
	assert.Equal(t, JobStatusScheduled, job.Status)
	assert.Equal(t, &scheduledTime, job.ScheduledAt)
}

func TestJobManagerScheduleCron(t *testing.T) {
	manager := NewJobManager(nil)
	
	handlerCalled := false
	manager.RegisterHandlerFunc("cron_job", func(ctx context.Context, job *Job) error {
		handlerCalled = true
		return nil
	})
	
	entryID, err := manager.ScheduleCron("* * * * * *", "cron_job", "default", nil)
	require.NoError(t, err)
	
	err = manager.Start()
	require.NoError(t, err)
	defer manager.Stop()
	
	time.Sleep(1200 * time.Millisecond)
	assert.True(t, handlerCalled)
	
	manager.UnscheduleCron(entryID)
}

func TestJobManagerEventListeners(t *testing.T) {
	manager := NewJobManager(nil)
	
	events := make([]string, 0)
	manager.AddEventListenerFunc(func(event *JobEvent) {
		events = append(events, event.Type)
	})
	
	manager.RegisterHandlerFunc("test", func(ctx context.Context, job *Job) error {
		return nil
	})
	
	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()
	
	job := NewJob("test", "default", nil)
	err = manager.Enqueue(job)
	require.NoError(t, err)
	
	time.Sleep(100 * time.Millisecond)
	
	assert.Contains(t, events, EventJobQueued)
	assert.Contains(t, events, EventJobStarted)
	assert.Contains(t, events, EventJobCompleted)
}

func TestJobManagerScaleQueue(t *testing.T) {
	manager := NewJobManager(nil)
	
	err := manager.ScaleQueue("custom", 3)
	require.NoError(t, err)
	
	stats := manager.GetWorkerStats()
	customWorkers := 0
	for _, stat := range stats {
		if stat.Queue == "custom" {
			customWorkers++
		}
	}
	
	assert.Equal(t, 3, customWorkers)
}

func TestJobManagerQueueOperations(t *testing.T) {
	manager := NewJobManager(nil)
	
	job1 := NewJob("test", "test_queue", nil)
	job2 := NewJob("test", "test_queue", nil)
	
	err := manager.Enqueue(job1)
	require.NoError(t, err)
	
	err = manager.Enqueue(job2)
	require.NoError(t, err)
	
	queueStats := manager.GetQueueStats()
	assert.Equal(t, 2, queueStats["test_queue"].Size)
	
	err = manager.ClearQueue("test_queue")
	require.NoError(t, err)
	
	queueStats = manager.GetQueueStats()
	assert.Equal(t, 0, queueStats["test_queue"].Size)
}

func TestJobManagerPauseResumeQueue(t *testing.T) {
	manager := NewJobManager(nil)
	
	err := manager.ScaleQueue("pausable", 2)
	require.NoError(t, err)
	
	stats := manager.GetWorkerStats()
	activeWorkers := 0
	for _, stat := range stats {
		if stat.Queue == "pausable" && stat.Status != WorkerStatusStopped {
			activeWorkers++
		}
	}
	assert.Equal(t, 2, activeWorkers)
	
	err = manager.PauseQueue("pausable")
	require.NoError(t, err)
	
	time.Sleep(50 * time.Millisecond)
	
	stats = manager.GetWorkerStats()
	activeWorkers = 0
	for _, stat := range stats {
		if stat.Queue == "pausable" && stat.Status != WorkerStatusStopped {
			activeWorkers++
		}
	}
	assert.Equal(t, 0, activeWorkers)
	
	err = manager.ResumeQueue("pausable", 1)
	require.NoError(t, err)
	
	time.Sleep(50 * time.Millisecond)
	
	stats = manager.GetWorkerStats()
	activeWorkers = 0
	for _, stat := range stats {
		if stat.Queue == "pausable" && stat.Status != WorkerStatusStopped {
			activeWorkers++
		}
	}
	assert.Equal(t, 1, activeWorkers)
}

func TestJobManagerMaxQueueSize(t *testing.T) {
	config := DefaultManagerConfig()
	config.MaxQueueSize = 2
	
	manager := NewJobManager(config)
	
	job1 := NewJob("test", "limited", nil)
	job2 := NewJob("test", "limited", nil)
	job3 := NewJob("test", "limited", nil)
	
	err := manager.Enqueue(job1)
	require.NoError(t, err)
	
	err = manager.Enqueue(job2)
	require.NoError(t, err)
	
	err = manager.Enqueue(job3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "queue limited is full")
}

func TestJobManagerStartStop(t *testing.T) {
	manager := NewJobManager(nil)
	
	stats := manager.GetManagerStats()
	assert.False(t, stats.IsRunning)
	
	err := manager.Start()
	require.NoError(t, err)
	
	stats = manager.GetManagerStats()
	assert.True(t, stats.IsRunning)
	
	err = manager.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
	
	err = manager.Stop()
	require.NoError(t, err)
	
	stats = manager.GetManagerStats()
	assert.False(t, stats.IsRunning)
	
	err = manager.Stop()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestJobManagerRetryLogic(t *testing.T) {
	config := DefaultManagerConfig()
	config.RetryConfig.MaxAttempts = 2
	config.RetryConfig.BaseDelay = 10 * time.Millisecond
	
	manager := NewJobManager(config)
	
	callCount := 0
	manager.RegisterHandlerFunc("retry_test", func(ctx context.Context, job *Job) error {
		callCount++
		if callCount < 2 {
			return errors.New("temporary error")
		}
		return nil
	})
	
	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()
	
	job := NewJob("retry_test", "default", nil)
	err = manager.Enqueue(job)
	require.NoError(t, err)
	
	time.Sleep(200 * time.Millisecond)
	
	assert.Equal(t, 2, callCount)
	
	metrics := manager.GetProcessorMetrics()
	assert.Equal(t, int64(1), metrics.JobsRetried)
	assert.Equal(t, int64(1), metrics.JobsCompleted)
}