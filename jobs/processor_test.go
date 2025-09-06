package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobProcessor(t *testing.T) {
	processor := NewJobProcessor(DefaultRetryConfig())
	
	handlerCalled := false
	processor.RegisterHandlerFunc("test", func(ctx context.Context, job *Job) error {
		handlerCalled = true
		return nil
	})
	
	job := NewJob("test", "default", nil)
	err := processor.ProcessJob(context.Background(), job)
	
	require.NoError(t, err)
	assert.True(t, handlerCalled)
	assert.Equal(t, JobStatusCompleted, job.Status)
	assert.NotNil(t, job.StartedAt)
	assert.NotNil(t, job.CompletedAt)
}

func TestJobProcessorWithError(t *testing.T) {
	processor := NewJobProcessor(DefaultRetryConfig())
	
	testError := errors.New("test error")
	processor.RegisterHandlerFunc("test", func(ctx context.Context, job *Job) error {
		return testError
	})
	
	job := NewJob("test", "default", nil)
	job.MaxAttempts = 2
	
	err := processor.ProcessJob(context.Background(), job)
	assert.Error(t, err)
	assert.Equal(t, JobStatusScheduled, job.Status)
	assert.Equal(t, 1, job.Attempts)
	assert.NotNil(t, job.ScheduledAt)
}

func TestJobProcessorMaxAttemptsReached(t *testing.T) {
	processor := NewJobProcessor(DefaultRetryConfig())
	
	deadLetterQueue := NewMemoryQueue("dead_letter")
	processor.SetDeadLetterQueue(deadLetterQueue)
	
	testError := errors.New("test error")
	processor.RegisterHandlerFunc("test", func(ctx context.Context, job *Job) error {
		return testError
	})
	
	job := NewJob("test", "default", nil)
	job.MaxAttempts = 1
	job.Attempts = 1
	
	err := processor.ProcessJob(context.Background(), job)
	assert.Error(t, err)
	assert.Equal(t, JobStatusFailed, job.Status)
	assert.Equal(t, 1, deadLetterQueue.Size())
}

func TestJobProcessorNoHandler(t *testing.T) {
	processor := NewJobProcessor(DefaultRetryConfig())
	
	job := NewJob("nonexistent", "default", nil)
	err := processor.ProcessJob(context.Background(), job)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no handler registered")
}

func TestJobProcessorTimeout(t *testing.T) {
	processor := NewJobProcessor(DefaultRetryConfig())
	
	processor.RegisterHandlerFunc("slow", func(ctx context.Context, job *Job) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return nil
		}
	})
	
	job := NewJob("slow", "default", map[string]interface{}{
		"timeout": "50ms",
	})
	
	err := processor.ProcessJob(context.Background(), job)
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestJobProcessorEventListeners(t *testing.T) {
	processor := NewJobProcessor(DefaultRetryConfig())
	
	events := make([]string, 0)
	
	processor.AddEventListenerFunc(func(event *JobEvent) {
		events = append(events, event.Type)
	})
	
	processor.RegisterHandlerFunc("test", func(ctx context.Context, job *Job) error {
		return nil
	})
	
	job := NewJob("test", "default", nil)
	err := processor.ProcessJob(context.Background(), job)
	
	require.NoError(t, err)
	
	time.Sleep(10 * time.Millisecond)
	
	assert.Contains(t, events, EventJobStarted)
	assert.Contains(t, events, EventJobCompleted)
}

func TestJobProcessorEventListenersWithError(t *testing.T) {
	processor := NewJobProcessor(DefaultRetryConfig())
	
	events := make([]string, 0)
	
	processor.AddEventListenerFunc(func(event *JobEvent) {
		events = append(events, event.Type)
	})
	
	processor.RegisterHandlerFunc("test", func(ctx context.Context, job *Job) error {
		return errors.New("test error")
	})
	
	job := NewJob("test", "default", nil)
	job.MaxAttempts = 1
	job.Attempts = 1
	
	err := processor.ProcessJob(context.Background(), job)
	assert.Error(t, err)
	
	time.Sleep(10 * time.Millisecond)
	
	assert.Contains(t, events, EventJobStarted)
	assert.Contains(t, events, EventJobFailed)
}

func TestJobProcessorMetrics(t *testing.T) {
	processor := NewJobProcessor(DefaultRetryConfig())
	
	processor.RegisterHandlerFunc("success", func(ctx context.Context, job *Job) error {
		return nil
	})
	
	processor.RegisterHandlerFunc("failure", func(ctx context.Context, job *Job) error {
		return errors.New("test error")
	})
	
	successJob := NewJob("success", "default", nil)
	failureJob := NewJob("failure", "default", nil)
	failureJob.MaxAttempts = 1
	failureJob.Attempts = 1
	
	processor.ProcessJob(context.Background(), successJob)
	processor.ProcessJob(context.Background(), failureJob)
	
	metrics := processor.GetMetrics()
	assert.Equal(t, int64(2), metrics.JobsProcessed)
	assert.Equal(t, int64(1), metrics.JobsCompleted)
	assert.Equal(t, int64(1), metrics.JobsFailed)
	assert.True(t, metrics.TotalDuration > 0)
	assert.True(t, metrics.AverageDuration > 0)
}

func TestJobProcessorResetMetrics(t *testing.T) {
	processor := NewJobProcessor(DefaultRetryConfig())
	
	processor.RegisterHandlerFunc("test", func(ctx context.Context, job *Job) error {
		return nil
	})
	
	job := NewJob("test", "default", nil)
	processor.ProcessJob(context.Background(), job)
	
	metrics := processor.GetMetrics()
	assert.Equal(t, int64(1), metrics.JobsProcessed)
	
	processor.ResetMetrics()
	
	metrics = processor.GetMetrics()
	assert.Equal(t, int64(0), metrics.JobsProcessed)
	assert.Equal(t, int64(0), metrics.JobsCompleted)
	assert.Equal(t, int64(0), metrics.JobsFailed)
	assert.Equal(t, time.Duration(0), metrics.TotalDuration)
	assert.Equal(t, time.Duration(0), metrics.AverageDuration)
}

func TestJobProcessorUnregisterHandler(t *testing.T) {
	processor := NewJobProcessor(DefaultRetryConfig())
	
	processor.RegisterHandlerFunc("test", func(ctx context.Context, job *Job) error {
		return nil
	})
	
	job := NewJob("test", "default", nil)
	err := processor.ProcessJob(context.Background(), job)
	require.NoError(t, err)
	
	processor.UnregisterHandler("test")
	
	err = processor.ProcessJob(context.Background(), job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no handler registered")
}