package jobs

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

type Queue interface {
	Push(job *Job) error
	Pop(ctx context.Context) (*Job, error)
	Peek() (*Job, error)
	Size() int
	Name() string
	Clear() error
}

type MemoryQueue struct {
	name  string
	jobs  []*Job
	mutex sync.RWMutex
}

func NewMemoryQueue(name string) *MemoryQueue {
	return &MemoryQueue{
		name: name,
		jobs: make([]*Job, 0),
	}
}

func (mq *MemoryQueue) Push(job *Job) error {
	mq.mutex.Lock()
	defer mq.mutex.Unlock()
	
	job.Queue = mq.name
	mq.jobs = append(mq.jobs, job)
	
	sort.Slice(mq.jobs, func(i, j int) bool {
		if mq.jobs[i].Priority != mq.jobs[j].Priority {
			return getQueuePriority(mq.jobs[i].Priority) > getQueuePriority(mq.jobs[j].Priority)
		}
		return mq.jobs[i].CreatedAt.Before(mq.jobs[j].CreatedAt)
	})
	
	return nil
}

func (mq *MemoryQueue) Pop(ctx context.Context) (*Job, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			mq.mutex.Lock()
			
			for i, job := range mq.jobs {
				if job.IsReady() {
					mq.jobs = append(mq.jobs[:i], mq.jobs[i+1:]...)
					mq.mutex.Unlock()
					return job, nil
				}
			}
			
			mq.mutex.Unlock()
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (mq *MemoryQueue) Peek() (*Job, error) {
	mq.mutex.RLock()
	defer mq.mutex.RUnlock()
	
	for _, job := range mq.jobs {
		if job.IsReady() {
			return job, nil
		}
	}
	
	return nil, fmt.Errorf("no ready jobs in queue")
}

func (mq *MemoryQueue) Size() int {
	mq.mutex.RLock()
	defer mq.mutex.RUnlock()
	return len(mq.jobs)
}

func (mq *MemoryQueue) Name() string {
	return mq.name
}

func (mq *MemoryQueue) Clear() error {
	mq.mutex.Lock()
	defer mq.mutex.Unlock()
	mq.jobs = mq.jobs[:0]
	return nil
}

func (mq *MemoryQueue) GetJobs() []*Job {
	mq.mutex.RLock()
	defer mq.mutex.RUnlock()
	
	jobs := make([]*Job, len(mq.jobs))
	copy(jobs, mq.jobs)
	return jobs
}

func (mq *MemoryQueue) GetJobsByStatus(status JobStatus) []*Job {
	mq.mutex.RLock()
	defer mq.mutex.RUnlock()
	
	var jobs []*Job
	for _, job := range mq.jobs {
		if job.Status == status {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

func (mq *MemoryQueue) RemoveJob(jobID string) error {
	mq.mutex.Lock()
	defer mq.mutex.Unlock()
	
	for i, job := range mq.jobs {
		if job.ID == jobID {
			mq.jobs = append(mq.jobs[:i], mq.jobs[i+1:]...)
			return nil
		}
	}
	
	return fmt.Errorf("job with ID %s not found", jobID)
}

type PriorityQueue struct {
	*MemoryQueue
}

func NewPriorityQueue(name string) *PriorityQueue {
	return &PriorityQueue{
		MemoryQueue: NewMemoryQueue(name),
	}
}

type DelayedQueue struct {
	*MemoryQueue
}

func NewDelayedQueue(name string) *DelayedQueue {
	return &DelayedQueue{
		MemoryQueue: NewMemoryQueue(name),
	}
}

func (dq *DelayedQueue) Push(job *Job) error {
	if job.ScheduledAt == nil {
		now := time.Now()
		job.ScheduledAt = &now
	}
	job.Status = JobStatusScheduled
	return dq.MemoryQueue.Push(job)
}

type QueueManager struct {
	queues map[string]Queue
	mutex  sync.RWMutex
}

func NewQueueManager() *QueueManager {
	return &QueueManager{
		queues: make(map[string]Queue),
	}
}

func (qm *QueueManager) RegisterQueue(name string, queue Queue) {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()
	qm.queues[name] = queue
}

func (qm *QueueManager) GetQueue(name string) (Queue, error) {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()
	
	queue, exists := qm.queues[name]
	if !exists {
		return nil, fmt.Errorf("queue %s not found", name)
	}
	
	return queue, nil
}

func (qm *QueueManager) GetOrCreateQueue(name string) Queue {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()
	
	if queue, exists := qm.queues[name]; exists {
		return queue
	}
	
	queue := NewMemoryQueue(name)
	qm.queues[name] = queue
	return queue
}

func (qm *QueueManager) ListQueues() []string {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()
	
	names := make([]string, 0, len(qm.queues))
	for name := range qm.queues {
		names = append(names, name)
	}
	
	return names
}

func (qm *QueueManager) GetQueueStats() map[string]QueueStats {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()
	
	stats := make(map[string]QueueStats)
	for name, queue := range qm.queues {
		if memQueue, ok := queue.(*MemoryQueue); ok {
			stats[name] = QueueStats{
				Name:      name,
				Size:      memQueue.Size(),
				Pending:   len(memQueue.GetJobsByStatus(JobStatusPending)),
				Running:   len(memQueue.GetJobsByStatus(JobStatusRunning)),
				Completed: len(memQueue.GetJobsByStatus(JobStatusCompleted)),
				Failed:    len(memQueue.GetJobsByStatus(JobStatusFailed)),
				Scheduled: len(memQueue.GetJobsByStatus(JobStatusScheduled)),
			}
		} else {
			stats[name] = QueueStats{
				Name: name,
				Size: queue.Size(),
			}
		}
	}
	
	return stats
}

func (qm *QueueManager) ClearQueue(name string) error {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()
	
	queue, exists := qm.queues[name]
	if !exists {
		return fmt.Errorf("queue %s not found", name)
	}
	
	return queue.Clear()
}

func (qm *QueueManager) RemoveQueue(name string) error {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()
	
	if _, exists := qm.queues[name]; !exists {
		return fmt.Errorf("queue %s not found", name)
	}
	
	delete(qm.queues, name)
	return nil
}

type QueueStats struct {
	Name      string `json:"name"`
	Size      int    `json:"size"`
	Pending   int    `json:"pending"`
	Running   int    `json:"running"`
	Completed int    `json:"completed"`
	Failed    int    `json:"failed"`
	Scheduled int    `json:"scheduled"`
}