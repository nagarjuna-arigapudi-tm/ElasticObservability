package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// GlobalConfig holds application-wide configuration
type GlobalConfig struct {
	LogLevel          string     `json:"logLevel" yaml:"logLevel"`
	MetricsPort       int        `json:"metricsPort" yaml:"metricsPort"`
	HistoryForIndices uint8      `json:"historyForIndices" yaml:"historyForIndices"`
	APIPort           int        `json:"apiPort" yaml:"apiPort"`
	Cert              CertConfig `json:"cert" yaml:"cert"`
	OutDir            string     `json:"out_dir" yaml:"out_dir"`
	ConfigDir         string     `json:"config_dir" yaml:"config_dir"`
}

// CertConfig holds certificate paths
type CertConfig struct {
	Cert   string `json:"cert" yaml:"cert"`
	Key    string `json:"key" yaml:"key"`
	CaCert string `json:"caCert" yaml:"caCert"`
}

// JobConfig represents a job configuration
type JobConfig struct {
	Name            string                 `json:"name" yaml:"name"`
	Type            string                 `json:"type" yaml:"type"` // shell, api, func, preDefined
	InternalJobName string                 `json:"internalJobName" yaml:"internalJobName"`
	Enabled         bool                   `json:"enabled" yaml:"enabled"`
	Schedule        *ScheduleConfig        `json:"schedule,omitempty" yaml:"schedule,omitempty"`
	DependsOn       []string               `json:"dependsOn,omitempty" yaml:"dependsOn,omitempty"`
	InitJob         bool                   `json:"initJob,omitempty" yaml:"initJob,omitempty"`
	ExcludeClusters []string               `json:"excludeClusters,omitempty" yaml:"excludeClusters,omitempty"`
	Parameters      map[string]interface{} `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

// ScheduleConfig represents job scheduling configuration
type ScheduleConfig struct {
	Cron        string `json:"cron,omitempty" yaml:"cron,omitempty"`
	Interval    string `json:"interval,omitempty" yaml:"interval,omitempty"` // e.g., "3m", "1h"
	InitialWait string `json:"initialWait,omitempty" yaml:"initialWait,omitempty"`
}

// CSVMappingConfig represents CSV mapping configuration for loadFromMasterCSV
type CSVMappingConfig struct {
	CSVFileName       string       `json:"csv_fileName" yaml:"csv_fileName"`
	CSVDeleteFileName string       `json:"csv_deleteFileName,omitempty" yaml:"csv_deleteFileName,omitempty"`
	InputMapping      InputMapping `json:"inputMapping" yaml:"inputMapping"`
}

// InputMapping represents the mapping from CSV to internal structures
type InputMapping struct {
	Constant map[string]interface{} `json:"constant,omitempty" yaml:"constant,omitempty"`
	Straight map[string]string      `json:"straight,omitempty" yaml:"straight,omitempty"`
	Derived  []DerivedField         `json:"derived,omitempty" yaml:"derived,omitempty"`
}

// DerivedField represents a derived field configuration
type DerivedField struct {
	Field    string      `json:"field" yaml:"field"`
	Column   string      `json:"column" yaml:"column"`
	Function string      `json:"function" yaml:"function"`
	Arg      interface{} `json:"arg" yaml:"arg"`
	RetVal   []string    `json:"retVal,omitempty" yaml:"retVal,omitempty"`
}

var (
	Global *GlobalConfig
)

// LoadGlobalConfig loads global configuration from file
func LoadGlobalConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	Global = &GlobalConfig{
		HistoryForIndices: 20, // default value
	}

	ext := filepath.Ext(configPath)
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, Global); err != nil {
			return fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, Global); err != nil {
			return fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		return fmt.Errorf("unsupported config file format: %s", ext)
	}

	// Set defaults if not specified
	if Global.HistoryForIndices == 0 {
		Global.HistoryForIndices = 20
	}
	if Global.OutDir == "" {
		Global.OutDir = "./outputs"
	}
	if Global.ConfigDir == "" {
		Global.ConfigDir = "./configs"
	}
	if Global.APIPort == 0 {
		Global.APIPort = 9092
	}
	if Global.MetricsPort == 0 {
		Global.MetricsPort = 9091
	}

	return nil
}

// LoadInitializationJobs loads initialization job configurations from initialization_jobs file
func LoadInitializationJobs(configDir string) ([]*JobConfig, error) {
	// Try YAML first
	yamlPath := filepath.Join(configDir, "initialization_jobs.yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		return LoadJobConfigFile(yamlPath)
	}

	// Try YML
	ymlPath := filepath.Join(configDir, "initialization_jobs.yml")
	if _, err := os.Stat(ymlPath); err == nil {
		return LoadJobConfigFile(ymlPath)
	}

	// Try JSON
	jsonPath := filepath.Join(configDir, "initialization_jobs.json")
	if _, err := os.Stat(jsonPath); err == nil {
		return LoadJobConfigFile(jsonPath)
	}

	return nil, fmt.Errorf("initialization_jobs file not found in %s", configDir)
}

// LoadScheduledJobs loads scheduled job configurations from scheduled_jobs file
func LoadScheduledJobs(configDir string) ([]*JobConfig, error) {
	// Try YAML first
	yamlPath := filepath.Join(configDir, "scheduled_jobs.yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		return LoadJobConfigFile(yamlPath)
	}

	// Try YML
	ymlPath := filepath.Join(configDir, "scheduled_jobs.yml")
	if _, err := os.Stat(ymlPath); err == nil {
		return LoadJobConfigFile(ymlPath)
	}

	// Try JSON
	jsonPath := filepath.Join(configDir, "scheduled_jobs.json")
	if _, err := os.Stat(jsonPath); err == nil {
		return LoadJobConfigFile(jsonPath)
	}

	return nil, fmt.Errorf("scheduled_jobs file not found in %s", configDir)
}

// LoadJobConfigs loads job configurations from a directory (kept for backward compatibility)
func LoadJobConfigs(configDir string) ([]*JobConfig, error) {
	var jobs []*JobConfig

	files, err := os.ReadDir(configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read config directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		ext := filepath.Ext(file.Name())
		if ext != ".yaml" && ext != ".yml" && ext != ".json" {
			continue
		}

		filePath := filepath.Join(configDir, file.Name())
		jobConfigs, err := LoadJobConfigFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load job config %s: %w", file.Name(), err)
		}
		jobs = append(jobs, jobConfigs...)
	}

	return jobs, nil
}

// LoadJobConfigFile loads job configurations from a single file
func LoadJobConfigFile(filePath string) ([]*JobConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read job config file: %w", err)
	}

	var configs struct {
		Jobs []*JobConfig `json:"jobs" yaml:"jobs"`
	}

	ext := filepath.Ext(filePath)
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &configs); err != nil {
			return nil, fmt.Errorf("failed to parse YAML job config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &configs); err != nil {
			return nil, fmt.Errorf("failed to parse JSON job config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file format: %s", ext)
	}

	return configs.Jobs, nil
}

// LoadOneTimeJobs loads one-time job configurations
func LoadOneTimeJobs(oneTimeDir string) ([]*JobConfig, error) {
	jobs, err := LoadJobConfigs(oneTimeDir)
	if err != nil {
		return nil, err
	}
	return jobs, nil
}

// MoveProcessedJob moves a processed one-time job to processed directory
func MoveProcessedJob(filePath, processedDir string, status string) error {
	fileName := filepath.Base(filePath)
	var destPath string

	if status == "failed" || status == "unparsed" {
		destPath = filepath.Join(processedDir, fileName+"."+status)
	} else {
		destPath = filepath.Join(processedDir, fileName)
	}

	// Ensure processed directory exists
	if err := os.MkdirAll(processedDir, 0755); err != nil {
		return fmt.Errorf("failed to create processed directory: %w", err)
	}

	if err := os.Rename(filePath, destPath); err != nil {
		return fmt.Errorf("failed to move processed job: %w", err)
	}

	return nil
}
