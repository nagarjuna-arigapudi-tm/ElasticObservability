package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"ElasticObservability/pkg/api"
	"ElasticObservability/pkg/config"
	"ElasticObservability/pkg/jobs"
	"ElasticObservability/pkg/logger"
	"ElasticObservability/pkg/scheduler"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	configFile = flag.String("config", "config.yaml", "Path to configuration file")
	logDir     = flag.String("log-dir", "./logs", "Directory for log files")
)

func main() {
	flag.Parse()

	fmt.Println("ElasticObservability - Starting...")

	// Load global configuration
	if err := config.LoadGlobalConfig(*configFile); err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	if err := os.MkdirAll(*logDir, 0755); err != nil {
		fmt.Printf("Failed to create log directory: %v\n", err)
		os.Exit(1)
	}

	appLogPath := filepath.Join(*logDir, "application.log")
	jobLogPath := filepath.Join(*logDir, "job.log")

	if err := logger.Init(config.Global.LogLevel, appLogPath, jobLogPath); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	logger.AppInfo("ElasticObservability started")
	logger.AppInfo("Configuration loaded from: %s", *configFile)

	// Create scheduler
	sched := scheduler.NewScheduler()

	// Register predefined jobs
	registerPredefinedJobs(sched)

	// Load and run initialization jobs first
	if err := loadAndRunInitializationJobs(sched); err != nil {
		logger.AppError("Failed to run initialization jobs: %v", err)
		os.Exit(1)
	}

	// Load and execute one-time jobs
	if err := loadOneTimeJobs(sched); err != nil {
		logger.AppError("Failed to load one-time jobs: %v", err)
		// Continue execution even if one-time jobs fail
	}

	// Load scheduled jobs
	if err := loadScheduledJobs(sched); err != nil {
		logger.AppError("Failed to load scheduled jobs: %v", err)
		os.Exit(1)
	}

	// Start scheduler
	sched.Start()
	logger.AppInfo("Job scheduler started")

	// Start API server
	apiServer := api.NewServer(sched)
	apiAddr := fmt.Sprintf(":%d", config.Global.APIPort)
	httpServer := &http.Server{
		Addr:    apiAddr,
		Handler: apiServer,
	}

	go func() {
		logger.AppInfo("API server listening on %s", apiAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.AppError("API server error: %v", err)
		}
	}()

	// Start Prometheus metrics server
	metricsAddr := fmt.Sprintf(":%d", config.Global.MetricsPort)
	metricsServer := &http.Server{
		Addr:    metricsAddr,
		Handler: promhttp.Handler(),
	}

	go func() {
		logger.AppInfo("Metrics server listening on %s", metricsAddr)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.AppError("Metrics server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.AppInfo("Shutdown signal received, stopping gracefully...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop scheduler
	sched.Stop()

	// Shutdown API server
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.AppError("API server shutdown error: %v", err)
	}

	// Shutdown metrics server
	if err := metricsServer.Shutdown(ctx); err != nil {
		logger.AppError("Metrics server shutdown error: %v", err)
	}

	logger.AppInfo("ElasticObservability stopped")
}

func registerPredefinedJobs(sched *scheduler.Scheduler) {
	sched.RegisterJobFunc("loadFromMasterCSV", jobs.LoadFromMasterCSV)
	sched.RegisterJobFunc("updateActiveEndpoint", jobs.UpdateActiveEndpoint)
	sched.RegisterJobFunc("updateAccessCredentials", jobs.UpdateAccessCredentials)
	sched.RegisterJobFunc("runCatIndices", jobs.RunCatIndices)
	sched.RegisterJobFunc("analyseIngest", jobs.AnalyseIngest)
	sched.RegisterJobFunc("updateStatsByDay", jobs.UpdateStatsByDay)
	sched.RegisterJobFunc("getThreadPoolWriteQueue", jobs.GetThreadPoolWriteQueue)
	logger.AppInfo("Predefined jobs registered")
}

func loadJobConfigurations(sched *scheduler.Scheduler) error {
	// Ensure config directory exists
	if err := os.MkdirAll(config.Global.ConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	jobConfigs, err := config.LoadJobConfigs(config.Global.ConfigDir)
	if err != nil {
		return fmt.Errorf("failed to load job configs: %w", err)
	}

	logger.AppInfo("Loaded %d job configurations", len(jobConfigs))

	for _, jobConfig := range jobConfigs {
		if err := sched.AddJob(jobConfig); err != nil {
			logger.AppWarn("Failed to add job %s: %v", jobConfig.Name, err)
			continue
		}
	}

	return nil
}

func loadAndRunInitializationJobs(sched *scheduler.Scheduler) error {
	// Ensure config directory exists
	if err := os.MkdirAll(config.Global.ConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	logger.AppInfo("Loading initialization jobs...")
	jobConfigs, err := config.LoadInitializationJobs(config.Global.ConfigDir)
	if err != nil {
		return fmt.Errorf("failed to load initialization jobs: %w", err)
	}

	logger.AppInfo("Loaded %d initialization job(s)", len(jobConfigs))

	// Add all initialization jobs to scheduler
	for _, jobConfig := range jobConfigs {
		if !jobConfig.Enabled {
			logger.AppInfo("Skipping disabled initialization job: %s", jobConfig.Name)
			continue
		}

		if err := sched.AddJob(jobConfig); err != nil {
			return fmt.Errorf("failed to add initialization job %s: %w", jobConfig.Name, err)
		}
		logger.AppInfo("Added initialization job: %s", jobConfig.Name)
	}

	// Run all initialization jobs in order
	logger.AppInfo("Running initialization jobs...")
	if err := sched.RunInitJobs(); err != nil {
		return fmt.Errorf("initialization jobs failed: %w", err)
	}

	logger.AppInfo("All initialization jobs completed successfully")
	return nil
}

func loadScheduledJobs(sched *scheduler.Scheduler) error {
	logger.AppInfo("Loading scheduled jobs...")
	jobConfigs, err := config.LoadScheduledJobs(config.Global.ConfigDir)
	if err != nil {
		return fmt.Errorf("failed to load scheduled jobs: %w", err)
	}

	logger.AppInfo("Loaded %d scheduled job(s)", len(jobConfigs))

	for _, jobConfig := range jobConfigs {
		if !jobConfig.Enabled {
			logger.AppInfo("Skipping disabled scheduled job: %s", jobConfig.Name)
			continue
		}

		if err := sched.AddJob(jobConfig); err != nil {
			logger.AppWarn("Failed to add scheduled job %s: %v", jobConfig.Name, err)
			continue
		}
		logger.AppInfo("Added scheduled job: %s", jobConfig.Name)
	}

	return nil
}

func loadOneTimeJobs(sched *scheduler.Scheduler) error {
	oneTimeDir := filepath.Join(config.Global.ConfigDir, "oneTime")
	processedDir := filepath.Join(config.Global.ConfigDir, "processedOneTime")

	// Ensure directories exist
	os.MkdirAll(oneTimeDir, 0755)
	os.MkdirAll(processedDir, 0755)

	files, err := os.ReadDir(oneTimeDir)
	if err != nil {
		return fmt.Errorf("failed to read oneTime directory: %w", err)
	}

	logger.AppInfo("Found %d one-time job(s)", len(files))

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(oneTimeDir, file.Name())
		jobConfigs, err := config.LoadJobConfigFile(filePath)
		if err != nil {
			logger.AppWarn("Failed to parse one-time job %s: %v", file.Name(), err)
			config.MoveProcessedJob(filePath, processedDir, "unparsed")
			continue
		}

		// Execute one-time jobs immediately
		for _, jobConfig := range jobConfigs {
			if !jobConfig.Enabled {
				continue
			}

			// Create a temporary job to execute
			tempSched := scheduler.NewScheduler()
			registerPredefinedJobs(tempSched)

			if err := tempSched.AddJob(jobConfig); err != nil {
				logger.AppError("Failed to add one-time job %s: %v", jobConfig.Name, err)
				config.MoveProcessedJob(filePath, processedDir, "failed")
				continue
			}

			// Trigger the job
			if err := tempSched.TriggerJob(jobConfig.Name); err != nil {
				logger.AppError("Failed to trigger one-time job %s: %v", jobConfig.Name, err)
				config.MoveProcessedJob(filePath, processedDir, "failed")
				continue
			}

			// Wait a bit for job to complete
			time.Sleep(2 * time.Second)
		}

		// Move to processed directory
		config.MoveProcessedJob(filePath, processedDir, "success")
	}

	return nil
}
