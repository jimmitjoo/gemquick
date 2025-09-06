package jobs

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type JobProcessor struct {
	handlers        map[string]JobHandler
	eventListeners  []EventListener
	retryConfig     RetryConfig
	deadLetterQueue Queue
	mutex           sync.RWMutex
	metrics         *ProcessorMetrics
}

type RetryConfig struct {
	BaseDelay      time.Duration
	MaxDelay       time.Duration
	MaxAttempts    int
	BackoffFactor  float64
	EnableJitter   bool
}

type EventListener interface {
	OnEvent(event *JobEvent)
}

type EventListenerFunc func(event *JobEvent)

func (f EventListenerFunc) OnEvent(event *JobEvent) {
	f(event)
}

type ProcessorMetrics struct {
	JobsProcessed   int64
	JobsCompleted   int64
	JobsFailed      int64
	JobsRetried     int64
	TotalDuration   time.Duration
	AverageDuration time.Duration
	mutex           sync.RWMutex
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		BaseDelay:     time.Second * 5,
		MaxDelay:      time.Hour,
		MaxAttempts:   3,
		BackoffFactor: 2.0,
		EnableJitter:  true,
	}
}

func NewJobProcessor(retryConfig RetryConfig) *JobProcessor {
	return &JobProcessor{
		handlers:       make(map[string]JobHandler),
		eventListeners: make([]EventListener, 0),
		retryConfig:    retryConfig,
		metrics:        &ProcessorMetrics{},
	}
}

func (jp *JobProcessor) RegisterHandler(jobType string, handler JobHandler) {
	jp.mutex.Lock()
	defer jp.mutex.Unlock()
	jp.handlers[jobType] = handler
}

func (jp *JobProcessor) RegisterHandlerFunc(jobType string, handler JobHandlerFunc) {
	jp.RegisterHandler(jobType, handler)
}

func (jp *JobProcessor) UnregisterHandler(jobType string) {
	jp.mutex.Lock()
	defer jp.mutex.Unlock()
	delete(jp.handlers, jobType)
}

func (jp *JobProcessor) AddEventListener(listener EventListener) {
	jp.mutex.Lock()
	defer jp.mutex.Unlock()
	jp.eventListeners = append(jp.eventListeners, listener)
}

func (jp *JobProcessor) AddEventListenerFunc(listener EventListenerFunc) {
	jp.AddEventListener(listener)
}

func (jp *JobProcessor) SetDeadLetterQueue(queue Queue) {
	jp.mutex.Lock()
	defer jp.mutex.Unlock()
	jp.deadLetterQueue = queue
}

func (jp *JobProcessor) ProcessJob(ctx context.Context, job *Job) error {
	startTime := time.Now()
	
	jp.emitEvent(EventJobStarted, job, nil, nil)
	jp.metrics.incrementProcessed()
	
	job.MarkRunning()
	
	err := jp.executeJob(ctx, job)
	
	duration := time.Since(startTime)
	jp.metrics.updateDuration(duration)
	
	if err != nil {
		if job.Status != JobStatusFailed {
			return jp.handleJobFailure(job, err)
		}
		return err
	}
	
	job.MarkCompleted(nil)
	jp.emitEvent(EventJobCompleted, job, nil, nil)
	jp.metrics.incrementCompleted()
	
	return nil
}

func (jp *JobProcessor) executeJob(ctx context.Context, job *Job) error {
	jp.mutex.RLock()
	handler, exists := jp.handlers[job.Type]
	jp.mutex.RUnlock()
	
	if !exists {
		job.MarkFailed(fmt.Errorf("no handler registered for job type: %s", job.Type))
		return fmt.Errorf("no handler registered for job type: %s", job.Type)
	}
	
	timeout := 30 * time.Minute
	if timeoutValue, exists := job.GetPayloadValue("timeout"); exists {
		if timeoutStr, ok := timeoutValue.(string); ok {
			if parsedTimeout, err := time.ParseDuration(timeoutStr); err == nil {
				timeout = parsedTimeout
			}
		}
	}
	
	jobCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	return handler.Handle(jobCtx, job)
}

func (jp *JobProcessor) handleJobFailure(job *Job, err error) error {
	jp.metrics.incrementFailed()
	
	if jp.shouldRetryJob(job) {
		return jp.scheduleRetry(job, err)
	}
	
	job.MarkFailed(err)
	jp.emitEvent(EventJobFailed, job, err, nil)
	
	if jp.deadLetterQueue != nil {
		jp.moveToDeadLetter(job)
	}
	
	return err
}

func (jp *JobProcessor) shouldRetryJob(job *Job) bool {
	return job.Attempts < jp.getMaxAttempts(job)
}

func (jp *JobProcessor) getMaxAttempts(job *Job) int {
	if job.MaxAttempts > 0 {
		return job.MaxAttempts
	}
	return jp.retryConfig.MaxAttempts
}

func (jp *JobProcessor) scheduleRetry(job *Job, err error) error {
	job.MarkRetrying(err)
	jp.metrics.incrementRetried()
	
	nextRetryAt := jp.calculateNextRetryTime(job)
	job.ScheduledAt = &nextRetryAt
	job.Status = JobStatusScheduled
	
	jp.emitEvent(EventJobRetrying, job, err, map[string]interface{}{
		"next_retry_at": nextRetryAt,
		"attempt":       job.Attempts,
	})
	
	return nil
}

func (jp *JobProcessor) calculateNextRetryTime(job *Job) time.Time {
	baseDelay := jp.retryConfig.BaseDelay
	maxDelay := jp.retryConfig.MaxDelay
	
	if customDelay, exists := job.GetPayloadValue("retry_delay"); exists {
		if delayStr, ok := customDelay.(string); ok {
			if parsedDelay, err := time.ParseDuration(delayStr); err == nil {
				baseDelay = parsedDelay
			}
		}
	}
	
	return calculateNextRetryAt(job.Attempts, baseDelay, maxDelay)
}

func (jp *JobProcessor) moveToDeadLetter(job *Job) {
	if err := jp.deadLetterQueue.Push(job); err != nil {
		log.Printf("Failed to move job %s to dead letter queue: %v", job.ID, err)
	} else {
		jp.emitEvent(EventJobDeadLetter, job, nil, nil)
	}
}

func (jp *JobProcessor) emitEvent(eventType string, job *Job, err error, metadata interface{}) {
	event := &JobEvent{
		Type:      eventType,
		Job:       job,
		Error:     err,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}
	
	jp.mutex.RLock()
	listeners := make([]EventListener, len(jp.eventListeners))
	copy(listeners, jp.eventListeners)
	jp.mutex.RUnlock()
	
	for _, listener := range listeners {
		go listener.OnEvent(event)
	}
}

func (jp *JobProcessor) GetMetrics() ProcessorMetrics {
	jp.metrics.mutex.RLock()
	defer jp.metrics.mutex.RUnlock()
	return *jp.metrics
}

func (jp *JobProcessor) ResetMetrics() {
	jp.metrics.mutex.Lock()
	defer jp.metrics.mutex.Unlock()
	jp.metrics.JobsProcessed = 0
	jp.metrics.JobsCompleted = 0
	jp.metrics.JobsFailed = 0
	jp.metrics.JobsRetried = 0
	jp.metrics.TotalDuration = 0
	jp.metrics.AverageDuration = 0
}

func (m *ProcessorMetrics) incrementProcessed() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.JobsProcessed++
}

func (m *ProcessorMetrics) incrementCompleted() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.JobsCompleted++
}

func (m *ProcessorMetrics) incrementFailed() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.JobsFailed++
}

func (m *ProcessorMetrics) incrementRetried() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.JobsRetried++
}

func (m *ProcessorMetrics) updateDuration(duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.TotalDuration += duration
	if m.JobsProcessed > 0 {
		m.AverageDuration = m.TotalDuration / time.Duration(m.JobsProcessed)
	}
}