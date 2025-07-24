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
func NewConfig(modulePaths []string, verbose, dryRun, fastMode bool, maxWorkers int) (*Config, error) {
	gomodcache, err := GetGOMODCACHE()
	if err != nil {
		return nil, fmt.Errorf("failed to get GOMODCACHE: %w", err)
	}

	// Set default maxWorkers if not specified or invalid
	if maxWorkers <= 0 {
		maxWorkers = 8 // Default to 8 workers for better performance
	}

	return &Config{
		ModulePaths: modulePaths,
		Verbose:     verbose,
		DryRun:      dryRun,
		FastMode:    fastMode,
		MaxWorkers:  maxWorkers,
		GoModCache:  gomodcache,
	}, nil
}
