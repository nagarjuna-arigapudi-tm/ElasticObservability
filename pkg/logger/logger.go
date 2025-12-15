package logger

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// LogLevel represents log severity
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var (
	appLogger  *Logger
	jobLogger  *Logger
	logLevel   LogLevel
	logLevelMu sync.RWMutex
)

// Logger represents a logger instance
type Logger struct {
	logger *log.Logger
	mu     sync.Mutex
}

// Init initializes the logging system
func Init(level string, appLogPath, jobLogPath string) error {
	logLevelMu.Lock()
	defer logLevelMu.Unlock()

	// Set log level
	switch level {
	case "debug":
		logLevel = DEBUG
	case "info":
		logLevel = INFO
	case "warn":
		logLevel = WARN
	case "error":
		logLevel = ERROR
	default:
		logLevel = INFO
	}

	// Create app logger
	appFile, err := os.OpenFile(appLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open app log file: %w", err)
	}
	appLogger = &Logger{
		logger: log.New(appFile, "", 0),
	}

	// Create job logger
	jobFile, err := os.OpenFile(jobLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open job log file: %w", err)
	}
	jobLogger = &Logger{
		logger: log.New(jobFile, "", 0),
	}

	return nil
}

// formatLog formats a log message with timestamp and level
func formatLog(level string, message string) string {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	return fmt.Sprintf("[%s] [%s] %s", timestamp, level, message)
}

// shouldLog checks if the message should be logged based on level
func shouldLog(level LogLevel) bool {
	logLevelMu.RLock()
	defer logLevelMu.RUnlock()
	return level >= logLevel
}

// AppDebug logs a debug message to application log
func AppDebug(format string, v ...interface{}) {
	if shouldLog(DEBUG) {
		msg := fmt.Sprintf(format, v...)
		appLogger.mu.Lock()
		appLogger.logger.Println(formatLog("DEBUG", msg))
		appLogger.mu.Unlock()
	}
}

// AppInfo logs an info message to application log
func AppInfo(format string, v ...interface{}) {
	if shouldLog(INFO) {
		msg := fmt.Sprintf(format, v...)
		appLogger.mu.Lock()
		appLogger.logger.Println(formatLog("INFO", msg))
		appLogger.mu.Unlock()
	}
}

// AppWarn logs a warning message to application log
func AppWarn(format string, v ...interface{}) {
	if shouldLog(WARN) {
		msg := fmt.Sprintf(format, v...)
		appLogger.mu.Lock()
		appLogger.logger.Println(formatLog("WARN", msg))
		appLogger.mu.Unlock()
	}
}

// AppError logs an error message to application log
func AppError(format string, v ...interface{}) {
	if shouldLog(ERROR) {
		msg := fmt.Sprintf(format, v...)
		appLogger.mu.Lock()
		appLogger.logger.Println(formatLog("ERROR", msg))
		appLogger.mu.Unlock()
	}
}

// JobDebug logs a debug message to job log
func JobDebug(jobName, format string, v ...interface{}) {
	if shouldLog(DEBUG) {
		msg := fmt.Sprintf(format, v...)
		jobLogger.mu.Lock()
		jobLogger.logger.Println(formatLog("DEBUG", fmt.Sprintf("[%s] %s", jobName, msg)))
		jobLogger.mu.Unlock()
	}
}

// JobInfo logs an info message to job log
func JobInfo(jobName, format string, v ...interface{}) {
	if shouldLog(INFO) {
		msg := fmt.Sprintf(format, v...)
		jobLogger.mu.Lock()
		jobLogger.logger.Println(formatLog("INFO", fmt.Sprintf("[%s] %s", jobName, msg)))
		jobLogger.mu.Unlock()
	}
}

// JobWarn logs a warning message to job log
func JobWarn(jobName, format string, v ...interface{}) {
	if shouldLog(WARN) {
		msg := fmt.Sprintf(format, v...)
		jobLogger.mu.Lock()
		jobLogger.logger.Println(formatLog("WARN", fmt.Sprintf("[%s] %s", jobName, msg)))
		jobLogger.mu.Unlock()
	}
}

// JobError logs an error message to job log
func JobError(jobName, format string, v ...interface{}) {
	if shouldLog(ERROR) {
		msg := fmt.Sprintf(format, v...)
		jobLogger.mu.Lock()
		jobLogger.logger.Println(formatLog("ERROR", fmt.Sprintf("[%s] %s", jobName, msg)))
		jobLogger.mu.Unlock()
	}
}
