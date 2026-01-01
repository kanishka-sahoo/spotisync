package scheduler

import (
	"context"
	"log"
)

// Worker processes jobs from the scheduler
type Worker struct {
	id        int
	scheduler *JobScheduler
	stopChan  chan struct{}
}

// NewWorker creates a new worker
func NewWorker(id int, scheduler *JobScheduler) *Worker {
	return &Worker{
		id:        id,
		scheduler: scheduler,
		stopChan:  make(chan struct{}),
	}
}

// Start starts the worker
func (w *Worker) Start(ctx context.Context) {
	log.Printf("Worker %d started", w.id)
	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d stopping due to context cancellation", w.id)
			return
		case <-w.stopChan:
			log.Printf("Worker %d stopping", w.id)
			return
		case jobWithUser := <-w.scheduler.queue:
			w.processJob(ctx, jobWithUser)
		}
	}
}

// Stop stops the worker
func (w *Worker) Stop() {
	close(w.stopChan)
}

// processJob processes a job
func (w *Worker) processJob(ctx context.Context, jobWithUser JobWithUser) {
	jobID := jobWithUser.JobID
	enqueuedUserID := jobWithUser.UserID

	log.Printf("Worker %d processing job %s", w.id, jobID)

	job, err := w.scheduler.db.GetJobByID(jobID)
	if err != nil {
		log.Printf("Worker %d: failed to get job %s: %v", w.id, jobID, err)
		return
	}
	if job == nil {
		log.Printf("Worker %d: job %s not found", w.id, jobID)
		return
	}

	// Verify user ownership
	if job.UserID != enqueuedUserID {
		log.Printf("SECURITY VIOLATION: Worker %d detected unauthorized job access - User %d attempted to process job %s owned by user %d",
			w.id, enqueuedUserID, jobID, job.UserID)
		return
	}

	// Skip if job is not pending
	if job.Status != "pending" {
		log.Printf("Worker %d: job %s is not pending (status: %s)", w.id, jobID, job.Status)
		return
	}

	// Execute with retry
	err = w.scheduler.executeWithRetry(ctx, job)
	if err != nil {
		log.Printf("Worker %d: job %s failed: %v", w.id, jobID, err)
		w.scheduler.db.UpdateJobFailed(jobID, err.Error())
	} else {
		log.Printf("Worker %d: job %s completed", w.id, jobID)
	}
}
