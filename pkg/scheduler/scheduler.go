package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"ElasticObservability/pkg/config"
	"ElasticObservability/pkg/logger"

	"github.com/robfig/cron/v3"
)

// JobFunc represents a job execution function
type JobFunc func(ctx context.Context, params map[string]interface{}) error

// Scheduler manages job scheduling and execution
type Scheduler struct {
	cron          *cron.Cron
	jobs          map[string]*Job
	jobFuncs      map[string]JobFunc
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	initJobs      []*Job
	dependencyMap map[string][]string // job name -> list of dependent job names
}

// Job represents a scheduled job
type Job struct {
	Config     *config.JobConfig
	EntryID    cron.EntryID
	Running    bool
	LastRun    time.Time
	NextRun    time.Time
	RunCount   int
	ErrorCount int
	mu         sync.RWMutex
}

// NewScheduler creates a new scheduler instance
func NewScheduler() *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		cron:          cron.New(cron.WithSeconds()),
		jobs:          make(map[string]*Job),
		jobFuncs:      make(map[string]JobFunc),
		ctx:           ctx,
		cancel:        cancel,
		initJobs:      make([]*Job, 0),
		dependencyMap: make(map[string][]string),
	}
}

// RegisterJobFunc registers a predefined job function
func (s *Scheduler) RegisterJobFunc(name string, fn JobFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobFuncs[name] = fn
	logger.AppInfo("Registered job function: %s", name)
}

// AddJob adds a job to the scheduler
func (s *Scheduler) AddJob(jobConfig *config.JobConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !jobConfig.Enabled {
		logger.AppInfo("Job %s is disabled, skipping", jobConfig.Name)
		return nil
	}

	job := &Job{
		Config: jobConfig,
	}

	s.jobs[jobConfig.Name] = job

	// Handle initialization jobs
	if jobConfig.InitJob {
		s.initJobs = append(s.initJobs, job)
		logger.AppInfo("Added initialization job: %s", jobConfig.Name)
		return nil
	}

	// Build dependency map
	if len(jobConfig.DependsOn) > 0 {
		for _, depJob := range jobConfig.DependsOn {
			s.dependencyMap[depJob] = append(s.dependencyMap[depJob], jobConfig.Name)
		}
		logger.AppInfo("Job %s depends on: %v", jobConfig.Name, jobConfig.DependsOn)
		return nil
	}

	// Schedule job if it has a schedule
	if jobConfig.Schedule != nil {
		return s.scheduleJob(job)
	}

	logger.AppInfo("Added job: %s (no schedule)", jobConfig.Name)
	return nil
}

// scheduleJob schedules a job based on its configuration
func (s *Scheduler) scheduleJob(job *Job) error {
	schedule := job.Config.Schedule

	// Parse initial wait duration
	var initialWait time.Duration
	if schedule.InitialWait != "" {
		dur, err := time.ParseDuration(schedule.InitialWait)
		if err != nil {
			return fmt.Errorf("invalid initial wait duration: %w", err)
		}
		initialWait = dur
	}

	// Create wrapped job function
	wrappedFunc := func() {
		s.executeJob(job)
	}

	var err error
	var entryID cron.EntryID

	// Schedule based on cron or interval
	if schedule.Cron != "" {
		entryID, err = s.cron.AddFunc(schedule.Cron, wrappedFunc)
		if err != nil {
			return fmt.Errorf("failed to schedule cron job: %w", err)
		}
		logger.AppInfo("Scheduled job %s with cron: %s", job.Config.Name, schedule.Cron)
	} else if schedule.Interval != "" {
		_, err := time.ParseDuration(schedule.Interval)
		if err != nil {
			return fmt.Errorf("invalid interval duration: %w", err)
		}

		// Use cron-like interval scheduling
		cronSpec := fmt.Sprintf("@every %s", schedule.Interval)
		entryID, err = s.cron.AddFunc(cronSpec, wrappedFunc)
		if err != nil {
			return fmt.Errorf("failed to schedule interval job: %w", err)
		}
		logger.AppInfo("Scheduled job %s with interval: %s", job.Config.Name, schedule.Interval)
	}

	job.EntryID = entryID

	// Schedule initial run if specified
	if initialWait > 0 {
		go func() {
			time.Sleep(initialWait)
			s.executeJob(job)
		}()
	}

	return nil
}

// executeJob executes a job
func (s *Scheduler) executeJob(job *Job) {
	job.mu.Lock()
	if job.Running {
		job.mu.Unlock()
		logger.JobWarn(job.Config.Name, "Job is already running, skipping")
		return
	}
	job.Running = true
	job.LastRun = time.Now()
	job.mu.Unlock()

	defer func() {
		job.mu.Lock()
		job.Running = false
		job.RunCount++
		job.mu.Unlock()

		// Execute dependent jobs
		s.executeDependentJobs(job.Config.Name)
	}()

	logger.JobInfo(job.Config.Name, "Starting job execution")

	var err error

	switch job.Config.Type {
	case "preDefined", "func":
		err = s.executePredefinedJob(job)
	case "shell":
		err = s.executeShellJob(job)
	case "api":
		err = s.executeAPIJob(job)
	default:
		err = fmt.Errorf("unknown job type: %s", job.Config.Type)
	}

	if err != nil {
		job.mu.Lock()
		job.ErrorCount++
		job.mu.Unlock()
		logger.JobError(job.Config.Name, "Job execution failed: %v", err)
	} else {
		logger.JobInfo(job.Config.Name, "Job execution completed successfully")
	}
}

// executePredefinedJob executes a predefined job function
func (s *Scheduler) executePredefinedJob(job *Job) error {
	s.mu.RLock()
	fn, exists := s.jobFuncs[job.Config.InternalJobName]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("job function not registered: %s", job.Config.InternalJobName)
	}

	return fn(s.ctx, job.Config.Parameters)
}

// executeShellJob executes a shell command job
func (s *Scheduler) executeShellJob(job *Job) error {
	// TODO: Implement shell command execution
	return fmt.Errorf("shell job execution not implemented yet")
}

// executeAPIJob executes an API call job
func (s *Scheduler) executeAPIJob(job *Job) error {
	// TODO: Implement API call execution
	return fmt.Errorf("API job execution not implemented yet")
}

// executeDependentJobs executes jobs that depend on the completed job
func (s *Scheduler) executeDependentJobs(completedJobName string) {
	s.mu.RLock()
	dependentJobs, exists := s.dependencyMap[completedJobName]
	s.mu.RUnlock()

	if !exists || len(dependentJobs) == 0 {
		return
	}

	logger.AppInfo("Executing dependent jobs of %s: %v", completedJobName, dependentJobs)

	for _, jobName := range dependentJobs {
		s.mu.RLock()
		job, exists := s.jobs[jobName]
		s.mu.RUnlock()

		if exists {
			go s.executeJob(job)
		}
	}
}

// RunInitJobs runs all initialization jobs
func (s *Scheduler) RunInitJobs() error {
	logger.AppInfo("Running %d initialization jobs", len(s.initJobs))

	for _, job := range s.initJobs {
		s.executeJob(job)

		// Wait for init job to complete before starting next one
		for {
			job.mu.RLock()
			running := job.Running
			job.mu.RUnlock()

			if !running {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		// Check if job failed
		job.mu.RLock()
		errorCount := job.ErrorCount
		job.mu.RUnlock()

		if errorCount > 0 {
			return fmt.Errorf("initialization job %s failed", job.Config.Name)
		}
	}

	logger.AppInfo("All initialization jobs completed successfully")
	return nil
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	logger.AppInfo("Starting job scheduler")
	s.cron.Start()
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	logger.AppInfo("Stopping job scheduler")
	s.cancel()
	ctx := s.cron.Stop()
	<-ctx.Done()
}

// GetJobStatus returns status information for all jobs
func (s *Scheduler) GetJobStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := make(map[string]interface{})
	for name, job := range s.jobs {
		job.mu.RLock()
		status[name] = map[string]interface{}{
			"running":    job.Running,
			"lastRun":    job.LastRun,
			"nextRun":    job.NextRun,
			"runCount":   job.RunCount,
			"errorCount": job.ErrorCount,
		}
		job.mu.RUnlock()
	}

	return status
}

// TriggerJob manually triggers a job by name
func (s *Scheduler) TriggerJob(jobName string) error {
	s.mu.RLock()
	job, exists := s.jobs[jobName]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("job not found: %s", jobName)
	}

	go s.executeJob(job)
	return nil
}
