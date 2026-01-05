package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"spotisync/internal/config"
	"spotisync/internal/db"
	"spotisync/internal/db/models"
	"spotisync/internal/services/download"
	"spotisync/internal/services/navidrome"
	"spotisync/internal/storage"
	"spotisync/internal/websocket"
)

// JobWithUser represents a job in the queue with its user ID
type JobWithUser struct {
	JobID  string
	UserID int64
}

// JobScheduler manages the job queue
type JobScheduler struct {
	db            *db.Database
	queue         chan JobWithUser // job IDs to process with user info
	workers       []*Worker
	mu            sync.RWMutex
	stopChan      chan struct{}
	retryPolicy   RetryPolicy
	orchestrator  *download.Orchestrator
	hub           *websocket.Hub
	cfg           *config.Config
	droppedJobs   int64 // counter for dropped jobs
	metricsStopCh chan struct{}
}

// NewJobScheduler creates a new job scheduler
func NewJobScheduler(database *db.Database, numWorkers int, retryPolicy RetryPolicy, queueSize int) *JobScheduler {
	workers := make([]*Worker, numWorkers)
	for i := 0; i < numWorkers; i++ {
		workers[i] = NewWorker(i, nil)
	}

	return &JobScheduler{
		db:            database,
		queue:         make(chan JobWithUser, queueSize),
		workers:       workers,
		stopChan:      make(chan struct{}),
		retryPolicy:   retryPolicy,
		metricsStopCh: make(chan struct{}),
	}
}

// NewJobSchedulerWithOrchestrator creates a new job scheduler with download orchestrator
func NewJobSchedulerWithOrchestrator(database *db.Database, numWorkers int, retryPolicy RetryPolicy, cfg *config.Config, hub *websocket.Hub) *JobScheduler {
	workers := make([]*Worker, numWorkers)
	for i := 0; i < numWorkers; i++ {
		workers[i] = NewWorker(i, nil)
	}

	// Initialize storage backend based on config
	storageConfig := storage.Config{
		Type: cfg.Storage.Type,
		SFTP: storage.SFTPConfig{
			Host:       cfg.Storage.SFTP.Host,
			Port:       cfg.Storage.SFTP.Port,
			Username:   cfg.Storage.SFTP.Username,
			Password:   cfg.Storage.SFTP.Password,
			SSHKeyPath: cfg.Storage.SFTP.SSHKeyPath,
			RemotePath: cfg.Storage.SFTP.RemotePath,
		},
	}

	storageBackend, err := storage.NewStorage(storageConfig)
	if err != nil {
		log.Fatalf("Failed to initialize storage backend: %v", err)
	}

	// Determine the music root path based on storage type
	musicRoot := cfg.Storage.MusicRoot
	if cfg.Storage.Type == "sftp" {
		// When using SFTP storage, use the remote path for building file paths
		musicRoot = cfg.Storage.SFTP.RemotePath
	}

	// Create the download orchestrator
	// Use Sources config for Tidal/Qobuz credentials (these are optional)
	orchestratorCfg := download.OrchestratorConfig{
		TidalClientID:     cfg.Sources.Tidal.ClientID,
		TidalClientSecret: cfg.Sources.Tidal.ClientSecret,
		QobuzAppID:        cfg.Sources.Qobuz.AppID,
		QobuzSecret:       cfg.Sources.Qobuz.Secret,
		MusicRoot:         musicRoot,
		TempDir:           cfg.Storage.TempDir,
		UseThirdPartyAPIs: true, // Default to third-party APIs
		Storage:           storageBackend,
	}
	orchestrator := download.NewOrchestrator(orchestratorCfg)

	scheduler := &JobScheduler{
		db:            database,
		queue:         make(chan JobWithUser, cfg.Workers.QueueSize),
		workers:       workers,
		stopChan:      make(chan struct{}),
		retryPolicy:   retryPolicy,
		orchestrator:  orchestrator,
		hub:           hub,
		cfg:           cfg,
		metricsStopCh: make(chan struct{}),
	}

	return scheduler
}

// SetOrchestrator sets the download orchestrator
func (s *JobScheduler) SetOrchestrator(orchestrator *download.Orchestrator) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orchestrator = orchestrator
}

// SetHub sets the WebSocket hub for progress updates
func (s *JobScheduler) SetHub(hub *websocket.Hub) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hub = hub
}

// SetConfig sets the configuration
func (s *JobScheduler) SetConfig(cfg *config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
}

// Start starts the scheduler and workers
func (s *JobScheduler) Start(ctx context.Context) {
	// Update worker references to this scheduler
	for _, worker := range s.workers {
		worker.scheduler = s
	}

	// Build ISRC index for duplicate detection (runs in background)
	if s.orchestrator != nil {
		go s.orchestrator.BuildISRCIndex()
	}

	// Start workers
	s.mu.Lock()
	for i := 0; i < len(s.workers); i++ {
		go s.workers[i].Start(ctx)
	}
	s.mu.Unlock()

	// Process queue
	go s.processQueue(ctx)

	// Start periodic metrics logging
	go s.logQueueMetrics(ctx)
}

// Stop stops the scheduler gracefully
func (s *JobScheduler) Stop() {
	close(s.stopChan)
	close(s.metricsStopCh)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, worker := range s.workers {
		worker.Stop()
	}
}

// logQueueMetrics logs queue metrics periodically
func (s *JobScheduler) logQueueMetrics(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.metricsStopCh:
			return
		case <-ticker.C:
			queueLen := len(s.queue)
			queueCap := cap(s.queue)
			droppedJobs := s.droppedJobs
			utilization := float64(queueLen) / float64(queueCap) * 100

			log.Printf("[Queue Metrics] Queue: %d/%d (%.1f%% full) | Workers: %d | Dropped jobs: %d",
				queueLen, queueCap, utilization, len(s.workers), droppedJobs)
		}
	}
}

// Enqueue adds a job ID to the processing queue with user ID
func (s *JobScheduler) Enqueue(jobID string, userID int64) {
	jobWithUser := JobWithUser{
		JobID:  jobID,
		UserID: userID,
	}

	queueLen := len(s.queue)
	queueCap := cap(s.queue)
	utilization := float64(queueLen) / float64(queueCap)

	// Warn when queue is approaching capacity (80% full)
	if utilization >= 0.8 && utilization < 1.0 {
		log.Printf("WARNING: Queue is %.1f%% full (%d/%d). Consider increasing queue size or worker count.",
			utilization*100, queueLen, queueCap)
	}

	select {
	case s.queue <- jobWithUser:
		// Successfully enqueued
	default:
		// Queue is full - job dropped
		s.droppedJobs++
		log.Printf("ERROR: Queue full (%d/%d), job %s dropped. Total dropped: %d",
			queueCap, queueCap, jobID, s.droppedJobs)
	}
}

// processQueue processes jobs from the queue
func (s *JobScheduler) processQueue(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case jobWithUser := <-s.queue:
			s.processJob(ctx, jobWithUser)
		}
	}
}

// processJob processes a single job
func (s *JobScheduler) processJob(ctx context.Context, jobWithUser JobWithUser) {
	jobID := jobWithUser.JobID
	enqueuedUserID := jobWithUser.UserID

	job, err := s.db.GetJobByID(jobID)
	if err != nil {
		log.Printf("Failed to get job %s: %v", jobID, err)
		return
	}
	if job == nil {
		log.Printf("Job %s not found", jobID)
		return
	}

	// Verify user ownership
	if job.UserID != enqueuedUserID {
		log.Printf("SECURITY VIOLATION: User %d attempted to process job %s owned by user %d",
			enqueuedUserID, jobID, job.UserID)
		return
	}

	// Skip if job is not pending
	if job.Status != models.JobStatusPending {
		log.Printf("Job %s is not pending (status: %s)", jobID, job.Status)
		return
	}

	// Execute job with retry logic
	err = s.executeWithRetry(ctx, job)
	if err != nil {
		log.Printf("Job %s failed after retries: %v", jobID, err)
		s.db.UpdateJobFailed(jobID, err.Error())

		// Send WebSocket update for failure
		if s.hub != nil {
			s.hub.BroadcastJobUpdate(job.UserID, jobID, job.BatchID, 0, string(models.JobStatusFailed))
		}

		// Update batch status
		s.updateBatchStatus(job.BatchID)
	}
}

// executeWithRetry executes a job with retry logic
func (s *JobScheduler) executeWithRetry(ctx context.Context, job *models.Job) error {
	var lastErr error

	for attempt := 0; attempt <= s.retryPolicy.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := s.retryPolicy.GetDelay(attempt - 1)
			log.Printf("Retrying job %s in %v (attempt %d/%d)", job.ID, delay, attempt, s.retryPolicy.MaxRetries)
			time.Sleep(delay)

			// Increment retry count in database
			s.db.IncrementJobRetryCount(job.ID)
		}

		err := s.executeJob(ctx, job)
		if err == nil {
			return nil
		}

		// Don't retry for "not found" or "skipped" errors
		if download.IsNotFoundError(err) || download.IsSkippedError(err) {
			return err
		}

		lastErr = err
	}

	return lastErr
}

// executeJob executes a single job using the download orchestrator
func (s *JobScheduler) executeJob(ctx context.Context, job *models.Job) error {
	// Mark job as in progress
	if err := s.db.UpdateJobStarted(job.ID); err != nil {
		return err
	}

	log.Printf("Processing job %s: %s - %s", job.ID, job.TrackName, job.ArtistName)

	// Send initial WebSocket update
	if s.hub != nil {
		s.hub.BroadcastJobUpdate(job.UserID, job.ID, job.BatchID, 0, string(models.JobStatusInProgress))
	}

	// Check if orchestrator is available
	if s.orchestrator == nil || !s.orchestrator.IsConfigured() {
		log.Printf("Warning: Download orchestrator not configured, using placeholder")
		return s.executeJobPlaceholder(job)
	}

	// Create progress callback for WebSocket updates
	progressCallback := func(progress float64, status string) {
		// Update database with progress
		s.db.UpdateJobProgress(job.ID, progress, 0, 0, 0)

		// Send WebSocket update
		if s.hub != nil {
			s.hub.BroadcastJobUpdate(job.UserID, job.ID, job.BatchID, progress, status)
		}
	}

	// Execute download using orchestrator
	result, err := s.orchestrator.DownloadTrack(ctx, job, progressCallback)

	// Handle different result statuses
	if err != nil {
		if download.IsSkippedError(err) {
			// Mark as skipped (file already exists)
			log.Printf("Job %s skipped: %s", job.ID, err.Error())
			s.db.UpdateJobStatus(job.ID, models.JobStatusSkipped, "", "", "", "", "", 0, err.Error(), 100)
			// Update granular status for skipped jobs (all components are complete)
			s.db.UpdateJobGranularStatus(job.ID, "completed", "completed", "completed")
			if s.hub != nil {
				s.hub.BroadcastJobUpdateWithGranular(job.UserID, job.ID, job.BatchID, 100, string(models.JobStatusSkipped), "completed", "completed", "completed")
			}
			s.updateBatchStatus(job.BatchID)
			return nil // Not an error, just skipped
		}

		if download.IsNotFoundError(err) {
			// Mark as not found
			log.Printf("Job %s not found: %s", job.ID, err.Error())
			s.db.UpdateJobStatus(job.ID, models.JobStatusNotFound, "", "", "", "", "", 0, err.Error(), 0)
			// Update granular status for not found
			if result != nil && result.SongStatus != "" {
				s.db.UpdateJobGranularStatus(job.ID, result.SongStatus, result.LyricsStatus, result.CoverStatus)
			} else {
				s.db.UpdateJobGranularStatus(job.ID, "not_found", "", "")
			}
			if s.hub != nil {
				s.hub.BroadcastJobUpdate(job.UserID, job.ID, job.BatchID, 0, string(models.JobStatusNotFound))
			}
			s.updateBatchStatus(job.BatchID)
			return err
		}

		// Generic failure - update granular status if available
		log.Printf("Job %s failed: %s", job.ID, err.Error())
		if result != nil && result.SongStatus != "" {
			s.db.UpdateJobGranularStatus(job.ID, result.SongStatus, result.LyricsStatus, result.CoverStatus)
		}
		return err
	}

	// Success - update job with result
	sourceService := string(result.SourceService)
	if err := s.db.UpdateJobStatus(
		job.ID,
		models.JobStatusCompleted,
		sourceService,
		result.SourceID,
		result.LocalPath,
		result.CoverPath,
		result.LyricsPath,
		result.FileSize,
		"",
		100,
	); err != nil {
		return err
	}

	// Update granular status
	if result.SongStatus != "" || result.LyricsStatus != "" || result.CoverStatus != "" {
		s.db.UpdateJobGranularStatus(job.ID, result.SongStatus, result.LyricsStatus, result.CoverStatus)
	}

	log.Printf("Job %s completed: %s", job.ID, result.LocalPath)

	// Send final WebSocket update with granular status
	if s.hub != nil {
		s.hub.BroadcastJobUpdateWithGranular(job.UserID, job.ID, job.BatchID, 100, string(models.JobStatusCompleted), result.SongStatus, result.LyricsStatus, result.CoverStatus)
	}

	// Update batch status
	s.updateBatchStatus(job.BatchID)

	return nil
}

// executeJobPlaceholder is the placeholder implementation when orchestrator is not configured
func (s *JobScheduler) executeJobPlaceholder(job *models.Job) error {
	log.Printf("Placeholder: Processing job %s: %s - %s", job.ID, job.TrackName, job.ArtistName)

	// Simulate processing with progress updates
	for progress := 0.0; progress <= 100; progress += 20 {
		time.Sleep(200 * time.Millisecond)
		s.db.UpdateJobProgress(job.ID, progress, 0, 0, 0)
		if s.hub != nil {
			s.hub.BroadcastJobUpdate(job.UserID, job.ID, job.BatchID, progress, "Processing")
		}
	}

	// Mark as completed with placeholder path
	if err := s.db.UpdateJobCompleted(job.ID, "/path/to/placeholder.flac", 1024*1024); err != nil {
		return err
	}

	// Send final WebSocket update
	if s.hub != nil {
		s.hub.BroadcastJobUpdate(job.UserID, job.ID, job.BatchID, 100, string(models.JobStatusCompleted))
	}

	// Update batch status
	s.updateBatchStatus(job.BatchID)

	return nil
}

// updateBatchStatus updates the batch status based on completed/failed jobs
// When a batch completes, it automatically triggers Navidrome sync and playlist creation
func (s *JobScheduler) updateBatchStatus(batchID string) {
	if batchID == "" {
		return
	}

	jobs, err := s.db.GetJobsByBatchID(batchID)
	if err != nil {
		log.Printf("Failed to get jobs for batch %s: %v", batchID, err)
		return
	}

	batch, err := s.db.GetBatchByID(batchID)
	if err != nil || batch == nil {
		log.Printf("Failed to get batch %s: %v", batchID, err)
		return
	}

	// Count completed and failed jobs
	completed := 0
	failed := 0
	pending := 0
	inProgress := 0

	for _, job := range jobs {
		switch job.Status {
		case models.JobStatusCompleted, models.JobStatusSkipped:
			completed++
		case models.JobStatusFailed, models.JobStatusNotFound:
			failed++
		case models.JobStatusPending:
			pending++
		case models.JobStatusInProgress:
			inProgress++
		}
	}

	// Determine batch status
	var batchStatus models.BatchStatus
	batchJustCompleted := false

	if pending == 0 && inProgress == 0 {
		// All jobs finished
		if failed == 0 {
			batchStatus = models.BatchStatusCompleted
		} else if completed == 0 {
			batchStatus = models.BatchStatusFailed
		} else {
			batchStatus = models.BatchStatusCompleted // Partial success
		}

		// Check if this is a new completion (batch wasn't already completed)
		if batch.Status != models.BatchStatusCompleted && batch.Status != models.BatchStatusFailed {
			batchJustCompleted = true
		}
	} else {
		batchStatus = models.BatchStatusProcessing
	}

	// Update batch in database
	s.db.UpdateBatchStatus(batchID, batchStatus, completed, failed)

	// Send WebSocket update
	if s.hub != nil {
		s.hub.BroadcastBatchUpdate(batch.UserID, batchID, completed, failed)
	}

	// If batch just completed, trigger automatic Navidrome sync and playlist creation
	if batchJustCompleted && completed > 0 {
		go s.onBatchComplete(batch, jobs)
	}
}

// onBatchComplete handles automatic Navidrome sync and playlist creation when a batch finishes
func (s *JobScheduler) onBatchComplete(batch *models.Batch, jobs []*models.Job) {
	log.Printf("[scheduler] Batch %s completed, triggering automatic Navidrome sync", batch.ID)
	ctx := context.Background()

	// Step 1: Trigger library scan using ADMIN credentials from config
	// Admin credentials are required for scan operations
	if s.cfg != nil && s.cfg.Navidrome.Host != "" && s.cfg.Navidrome.Username != "" {
		adminClient := navidrome.NewClient(s.cfg.Navidrome.Host, s.cfg.Navidrome.Username, s.cfg.Navidrome.Password)

		if err := adminClient.StartScan(ctx); err != nil {
			log.Printf("[scheduler] Failed to start Navidrome scan for batch %s: %v", batch.ID, err)
			// Continue anyway - scan might already be in progress
		} else {
			log.Printf("[scheduler] Navidrome scan triggered for batch %s", batch.ID)
		}

		// Wait for scan to complete (with timeout)
		scanTimeout := 5 * time.Minute
		if err := adminClient.WaitForScanComplete(ctx, scanTimeout); err != nil {
			log.Printf("[scheduler] Navidrome scan did not complete in time for batch %s: %v", batch.ID, err)
			// Continue anyway - we'll try to create playlist with whatever is indexed
		}
	} else {
		log.Printf("[scheduler] Navidrome admin credentials not configured, skipping auto-scan")
	}

	// Step 2: If this is a playlist batch, create the playlist using USER credentials
	// This ensures the playlist is owned by the correct user
	if batch.SpotifyType == models.SpotifyTypePlaylist {
		// Get user credentials for playlist creation
		user, err := s.db.GetUserByID(batch.UserID)
		if err != nil || user == nil {
			log.Printf("[scheduler] Failed to get user %d for playlist creation: %v", batch.UserID, err)
			return
		}

		// Use user's Navidrome URL, or fall back to admin URL if user hasn't configured one
		navidromeURL := user.NavidromeURL
		if navidromeURL == "" && s.cfg != nil {
			navidromeURL = s.cfg.Navidrome.Host
		}

		if navidromeURL == "" || user.NavidromeUsername == "" {
			log.Printf("[scheduler] Navidrome not configured for user %d, skipping playlist creation", batch.UserID)
			return
		}

		userClient := navidrome.NewClient(navidromeURL, user.NavidromeUsername, user.NavidromePassword)
		s.createNavidromePlaylist(userClient, batch, jobs)
	}
}

// createNavidromePlaylist creates a Navidrome playlist from a completed batch
// Tracks are added in the same order as the original Spotify playlist
func (s *JobScheduler) createNavidromePlaylist(client *navidrome.Client, batch *models.Batch, jobs []*models.Job) {
	log.Printf("[scheduler] Creating Navidrome playlist for batch %s: %s", batch.ID, batch.Name)
	ctx := context.Background()

	// Calculate total completed/skipped jobs (these are the ones we can include)
	var completedJobs []*models.Job
	for _, job := range jobs {
		if job.Status == models.JobStatusCompleted || job.Status == models.JobStatusSkipped {
			completedJobs = append(completedJobs, job)
		}
	}
	tracksTotal := len(completedJobs)

	// Send initial notification
	if s.hub != nil {
		s.hub.BroadcastPlaylistUpdate(batch.UserID, batch.ID, "searching", "",
			fmt.Sprintf("Searching for %d tracks in Navidrome...", tracksTotal), 0, tracksTotal, tracksTotal)
	}

	// Update database status
	s.db.UpdateBatchPlaylistStatus(batch.ID, models.PlaylistStatusCreating, "", "Searching for tracks...", 0, 0)

	// Determine playlist name
	playlistName := batch.Name
	if playlistName == "" {
		playlistName = "Spotisync Playlist"
	}

	// Call the new method to create playlist with detailed results
	result := client.CreatePlaylistWithDetails(ctx, playlistName, completedJobs)

	// Update database with results
	if result.Success {
		s.db.UpdateBatchPlaylistStatus(batch.ID, models.PlaylistStatusCompleted, result.PlaylistID,
			fmt.Sprintf("Playlist created with %d/%d tracks", result.TracksFound, result.TotalTracks),
			result.TracksFound, result.TracksFailed)
	} else {
		s.db.UpdateBatchPlaylistStatus(batch.ID, models.PlaylistStatusFailed, "",
			"Failed to create playlist: "+result.Error, 0, 0)
	}

	// Send final notification
	status := "completed"
	message := fmt.Sprintf("Playlist created with %d/%d tracks", result.TracksFound, result.TotalTracks)
	if !result.Success {
		status = "failed"
		message = "Failed to create playlist: " + result.Error
	}

	if s.hub != nil {
		s.hub.BroadcastPlaylistUpdate(batch.UserID, batch.ID, status, result.PlaylistID,
			message, result.TracksFound, result.TracksFailed, result.TotalTracks)
	}
}

// GetQueueLength returns the current queue length
func (s *JobScheduler) GetQueueLength() int {
	return len(s.queue)
}

// RetryPolicy defines the retry behavior
type RetryPolicy struct {
	MaxRetries int
	Delays     []time.Duration
}

// GetDelay returns the delay for a given retry attempt
func (p RetryPolicy) GetDelay(attempt int) time.Duration {
	if attempt < len(p.Delays) {
		return p.Delays[attempt]
	}
	return p.Delays[len(p.Delays)-1]
}
