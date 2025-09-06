package jobs

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type Worker struct {
	id          string
	queue       Queue
	processor   *JobProcessor
	ctx         context.Context
	cancel      context.CancelFunc
	status      WorkerStatus
	currentJob  *Job
	startedAt   time.Time
	completedJobs int
	failedJobs    int
	mutex       sync.RWMutex
}

type WorkerStatus string

const (
	WorkerStatusIdle    WorkerStatus = "idle"
	WorkerStatusBusy    WorkerStatus = "busy"
	WorkerStatusStopped WorkerStatus = "stopped"
)

func NewWorker(id string, queue Queue, processor *JobProcessor) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		id:        id,
		queue:     queue,
		processor: processor,
		ctx:       ctx,
		cancel:    cancel,
		status:    WorkerStatusIdle,
		startedAt: time.Now(),
	}
}

func (w *Worker) Start() {
	go w.run()
}

func (w *Worker) Stop() {
	w.cancel()
	w.setStatus(WorkerStatusStopped)
}

func (w *Worker) run() {
	for {
		select {
		case <-w.ctx.Done():
			return
		default:
			w.processNextJob()
		}
	}
}

func (w *Worker) processNextJob() {
	job, err := w.queue.Pop(w.ctx)
	if err != nil {
		if err == context.Canceled {
			return
		}
		log.Printf("Worker %s: Error popping job from queue: %v", w.id, err)
		time.Sleep(time.Second)
		return
	}

	w.setCurrentJob(job)
	w.setStatus(WorkerStatusBusy)

	err = w.processor.ProcessJob(w.ctx, job)
	
	w.mutex.Lock()
	if err != nil {
		w.failedJobs++
	} else {
		w.completedJobs++
	}
	w.mutex.Unlock()

	w.setCurrentJob(nil)
	w.setStatus(WorkerStatusIdle)
}

func (w *Worker) setStatus(status WorkerStatus) {
	w.mutex.Lock()
	w.status = status
	w.mutex.Unlock()
}

func (w *Worker) setCurrentJob(job *Job) {
	w.mutex.Lock()
	w.currentJob = job
	w.mutex.Unlock()
}

func (w *Worker) GetStatus() WorkerStatus {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return w.status
}

func (w *Worker) GetCurrentJob() *Job {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return w.currentJob
}

func (w *Worker) GetStats() WorkerStats {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	
	return WorkerStats{
		ID:            w.id,
		Status:        w.status,
		Queue:         w.queue.Name(),
		StartedAt:     w.startedAt,
		CompletedJobs: w.completedJobs,
		FailedJobs:    w.failedJobs,
		CurrentJob:    w.currentJob,
		Uptime:        time.Since(w.startedAt),
	}
}

type WorkerStats struct {
	ID            string        `json:"id"`
	Status        WorkerStatus  `json:"status"`
	Queue         string        `json:"queue"`
	StartedAt     time.Time     `json:"started_at"`
	CompletedJobs int           `json:"completed_jobs"`
	FailedJobs    int           `json:"failed_jobs"`
	CurrentJob    *Job          `json:"current_job,omitempty"`
	Uptime        time.Duration `json:"uptime"`
}

type WorkerPool struct {
	workers   map[string]*Worker
	processor *JobProcessor
	mutex     sync.RWMutex
}

func NewWorkerPool(processor *JobProcessor) *WorkerPool {
	return &WorkerPool{
		workers:   make(map[string]*Worker),
		processor: processor,
	}
}

func (wp *WorkerPool) AddWorker(queueName string, queue Queue) string {
	wp.mutex.Lock()
	defer wp.mutex.Unlock()

	workerID := fmt.Sprintf("%s-worker-%d", queueName, len(wp.workers)+1)
	worker := NewWorker(workerID, queue, wp.processor)
	wp.workers[workerID] = worker
	worker.Start()

	return workerID
}

func (wp *WorkerPool) RemoveWorker(workerID string) error {
	wp.mutex.Lock()
	defer wp.mutex.Unlock()

	worker, exists := wp.workers[workerID]
	if !exists {
		return fmt.Errorf("worker %s not found", workerID)
	}

	worker.Stop()
	delete(wp.workers, workerID)
	return nil
}

func (wp *WorkerPool) GetWorker(workerID string) (*Worker, error) {
	wp.mutex.RLock()
	defer wp.mutex.RUnlock()

	worker, exists := wp.workers[workerID]
	if !exists {
		return nil, fmt.Errorf("worker %s not found", workerID)
	}

	return worker, nil
}

func (wp *WorkerPool) ListWorkers() []string {
	wp.mutex.RLock()
	defer wp.mutex.RUnlock()

	workerIDs := make([]string, 0, len(wp.workers))
	for id := range wp.workers {
		workerIDs = append(workerIDs, id)
	}

	return workerIDs
}

func (wp *WorkerPool) GetAllWorkerStats() []WorkerStats {
	wp.mutex.RLock()
	defer wp.mutex.RUnlock()

	stats := make([]WorkerStats, 0, len(wp.workers))
	for _, worker := range wp.workers {
		stats = append(stats, worker.GetStats())
	}

	return stats
}

func (wp *WorkerPool) StopAll() {
	wp.mutex.Lock()
	defer wp.mutex.Unlock()

	for _, worker := range wp.workers {
		worker.Stop()
	}

	wp.workers = make(map[string]*Worker)
}

func (wp *WorkerPool) GetActiveWorkers() int {
	wp.mutex.RLock()
	defer wp.mutex.RUnlock()

	count := 0
	for _, worker := range wp.workers {
		if worker.GetStatus() != WorkerStatusStopped {
			count++
		}
	}

	return count
}

func (wp *WorkerPool) GetBusyWorkers() int {
	wp.mutex.RLock()
	defer wp.mutex.RUnlock()

	count := 0
	for _, worker := range wp.workers {
		if worker.GetStatus() == WorkerStatusBusy {
			count++
		}
	}

	return count
}

func (wp *WorkerPool) ScaleQueue(queueName string, queue Queue, targetWorkers int) error {
	wp.mutex.Lock()
	defer wp.mutex.Unlock()

	currentWorkers := 0
	var queueWorkers []string
	
	for id, worker := range wp.workers {
		if worker.queue.Name() == queueName {
			queueWorkers = append(queueWorkers, id)
			if worker.GetStatus() != WorkerStatusStopped {
				currentWorkers++
			}
		}
	}

	if currentWorkers < targetWorkers {
		for i := currentWorkers; i < targetWorkers; i++ {
			workerID := fmt.Sprintf("%s-worker-%d", queueName, i+1)
			worker := NewWorker(workerID, queue, wp.processor)
			wp.workers[workerID] = worker
			worker.Start()
		}
	} else if currentWorkers > targetWorkers {
		stopCount := currentWorkers - targetWorkers
		for i := 0; i < stopCount && i < len(queueWorkers); i++ {
			worker := wp.workers[queueWorkers[i]]
			worker.Stop()
			delete(wp.workers, queueWorkers[i])
		}
	}

	return nil
}