package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// GetGOMODCACHE gets Go module cache directory
func GetGOMODCACHE() (string, error) {
	// First try environment variable
	gomodcache := os.Getenv("GOMODCACHE")
	if gomodcache != "" {
		return gomodcache, nil
	}

	// If environment variable doesn't exist, use go env command
	cmd := exec.Command("go", "env", "GOMODCACHE")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get GOMODCACHE: %w", err)
	}

	gomodcache = strings.TrimSpace(string(output))
	if gomodcache == "" {
		return "", fmt.Errorf("GOMODCACHE is empty")
	}

	return gomodcache, nil
}

// GetGOPATH gets GOPATH directory
func GetGOPATH() (string, error) {
	// First try environment variable
	gopath := os.Getenv("GOPATH")
	if gopath != "" {
		return gopath, nil
	}

	// If environment variable doesn't exist, use go env command
	cmd := exec.Command("go", "env", "GOPATH")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get GOPATH: %w", err)
	}

	gopath = strings.TrimSpace(string(output))
	if gopath == "" {
		return "", fmt.Errorf("GOPATH is empty")
	}

	return gopath, nil
}

// FormatSize formats file size to human readable format
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp+1])
}

// calculateDirectorySize calculates directory total size (concurrent version)
func (mc *ModuleCleaner) calculateDirectorySize(dirPath string) (int64, error) {
	var totalSize int64
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Work channel to limit concurrency
	semaphore := make(chan struct{}, 10)

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		if !info.IsDir() {
			wg.Add(1)
			go func(size int64) {
				defer wg.Done()
				semaphore <- struct{}{}        // Acquire semaphore
				defer func() { <-semaphore }() // Release semaphore

				mu.Lock()
				totalSize += size
				mu.Unlock()
			}(info.Size())
		}

		return nil
	})

	wg.Wait()
	return totalSize, err
}

// CalculateDirectorySizeSimple calculates directory size simple version (for small directories)
func CalculateDirectorySizeSimple(dirPath string) (int64, error) {
	var totalSize int64

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		if !info.IsDir() {
			totalSize += info.Size()
		}

		return nil
	})

	return totalSize, err
}

// PathExists checks if path exists
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// IsGoModule checks if directory contains go.mod file
func IsGoModule(dirPath string) bool {
	goModPath := filepath.Join(dirPath, "go.mod")
	return PathExists(goModPath)
}

// ExpandHomePath expands user directory symbol in path
func ExpandHomePath(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if len(path) == 1 {
		return homeDir, nil
	}

	return filepath.Join(homeDir, path[2:]), nil
}

// CalculateCacheSize calculates total size of Go module cache
func CalculateCacheSize(cacheDir string) (int64, error) {
	var totalSize int64

	err := filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		if !info.IsDir() {
			totalSize += info.Size()
		}

		return nil
	})

	return totalSize, err
}
