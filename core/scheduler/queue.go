package scheduler

import (
	"container/heap"
	"sync"
	"time"

	"gpu-orchestrator/core/models"
)

// JobQueue is a priority queue for jobs
type JobQueue struct {
	jobs []*QueuedJob
	mu   sync.Mutex
}

// QueuedJob wraps a job with priority information
type QueuedJob struct {
	Job      *models.Job
	Priority float64 // Lower is higher priority
	Index    int     // For heap.Interface
}

// NewJobQueue creates a new job queue
func NewJobQueue() *JobQueue {
	jq := &JobQueue{
		jobs: make([]*QueuedJob, 0),
	}
	heap.Init(jq)
	return jq
}

// Enqueue adds a job to the queue
func (jq *JobQueue) Enqueue(job *models.Job) {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	priority := jq.calculatePriority(job)
	heap.Push(jq, &QueuedJob{
		Job:      job,
		Priority: priority,
	})
}

// PopJob removes and returns the highest priority job
func (jq *JobQueue) PopJob() *models.Job {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	if jq.Len() == 0 {
		return nil
	}

	item := heap.Pop(jq).(*QueuedJob)
	return item.Job
}

// Len returns the number of jobs in the queue
func (jq *JobQueue) Len() int {
	return len(jq.jobs)
}

// Less compares two jobs for priority (lower priority value = higher priority)
func (jq *JobQueue) Less(i, j int) bool {
	// Priority: deadline first, then budget
	if jq.jobs[i].Job.Constraints.Deadline != nil && jq.jobs[j].Job.Constraints.Deadline != nil {
		return jq.jobs[i].Job.Constraints.Deadline.Before(*jq.jobs[j].Job.Constraints.Deadline)
	}
	if jq.jobs[i].Job.Constraints.Deadline != nil {
		return true
	}
	if jq.jobs[j].Job.Constraints.Deadline != nil {
		return false
	}
	return jq.jobs[i].Job.Constraints.MaxBudget < jq.jobs[j].Job.Constraints.MaxBudget
}

// Swap swaps two jobs
func (jq *JobQueue) Swap(i, j int) {
	jq.jobs[i], jq.jobs[j] = jq.jobs[j], jq.jobs[i]
	jq.jobs[i].Index = i
	jq.jobs[j].Index = j
}

// Push implements heap.Interface
func (jq *JobQueue) Push(x interface{}) {
	n := len(jq.jobs)
	item := x.(*QueuedJob)
	item.Index = n
	jq.jobs = append(jq.jobs, item)
}

// Pop implements heap.Interface
func (jq *JobQueue) Pop() interface{} {
	old := jq.jobs
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.Index = -1
	jq.jobs = old[0 : n-1]
	return item
}

// calculatePriority calculates priority for a job
func (jq *JobQueue) calculatePriority(job *models.Job) float64 {
	priority := 0.0

	// Deadline urgency (sooner = higher priority)
	if job.Constraints.Deadline != nil {
		timeUntilDeadline := time.Until(*job.Constraints.Deadline).Hours()
		if timeUntilDeadline > 0 {
			priority += timeUntilDeadline // Lower time = lower priority value = higher priority
		}
	}

	// Budget (lower budget = higher priority, to process cheaper jobs first)
	priority += job.Constraints.MaxBudget

	return priority
}
