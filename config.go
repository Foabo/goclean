package main

import (
	"fmt"
)

// Config configuration structure
type Config struct {
	// ModulePaths paths to scan for go.mod files or directories containing go.mod files
	ModulePaths []string
	// Verbose enable verbose output mode
	Verbose bool
	// DryRun only simulate run, don't actually delete files
	DryRun bool
	// GoModCache Go module cache directory, defaults to $GOMODCACHE
	GoModCache string
	// FastMode skip indirect dependencies analysis for faster processing
	FastMode bool
	// MaxWorkers maximum number of concurrent workers for dependency analysis
	MaxWorkers int
	// Timeout timeout for go list commands in seconds
	Timeout int
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	gomodcache, _ := GetGOMODCACHE()
	return &Config{
		ModulePaths: []string{},
		Verbose:     false,
		DryRun:      false,
		GoModCache:  gomodcache,
	}
}

// NewConfig creates a new configuration instance
func NewConfig(modulePaths []string, verbose, dryRun, fastMode bool, maxWorkers, timeout int) (*Config, error) {
	gomodcache, err := GetGOMODCACHE()
	if err != nil {
		return nil, fmt.Errorf("failed to get GOMODCACHE: %w", err)
	}

	// Set default maxWorkers if not specified or invalid
	if maxWorkers <= 0 {
		maxWorkers = 16 // Default to 16 workers for better performance
	}

	// Set default timeout if not specified or invalid
	if timeout <= 0 {
		timeout = 60 // Default to 60 seconds
	}

	return &Config{
		ModulePaths: modulePaths,
		Verbose:     verbose,
		DryRun:      dryRun,
		FastMode:    fastMode,
		MaxWorkers:  maxWorkers,
		Timeout:     timeout,
		GoModCache:  gomodcache,
	}, nil
}
