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

// DiscoverGoProjects automatically finds Go projects in common locations.
// It searches for go.mod files in ~/go and $GOPATH/src.
func DiscoverGoProjects() ([]string, error) {
	var projectDirs []string
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not get user home directory: %w", err)
	}

	gopath, err := GetGOPATH()
	if err != nil {
		// GOPATH might not be set, which is not a fatal error.
		if os.Getenv("GOCLEAN_VERBOSE") == "true" {
			fmt.Println("Warning: Could not determine GOPATH. Search will be limited to ~/go.")
		}
		gopath = "" // Ensure gopath is empty if not found
	}

	// Define standard search paths
	searchPaths := []string{
		filepath.Join(homeDir, "go"), // Modern default
	}
	if gopath != "" {
		searchPaths = append(searchPaths, filepath.Join(gopath, "src")) // Legacy GOPATH
	}

	foundProjects := make(map[string]bool)

	for _, path := range searchPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue // Skip paths that don't exist
		}

		walkErr := filepath.Walk(path, func(modPath string, info os.FileInfo, err error) error {
			if err != nil {
				// Ignore errors from paths we can't access
				return nil
			}

			// Skip vendor directories and the module cache itself to avoid deep crawls
			if info.IsDir() && (info.Name() == "vendor" || info.Name() == "pkg" || info.Name() == ".git") {
				return filepath.SkipDir
			}

			if !info.IsDir() && info.Name() == "go.mod" {
				projectDir := filepath.Dir(modPath)
				if !foundProjects[projectDir] {
					projectDirs = append(projectDirs, projectDir)
					foundProjects[projectDir] = true
				}
			}
			return nil
		})

		if walkErr != nil {
			fmt.Printf("Warning: error walking path %s: %v\n", path, walkErr)
		}
	}
	return projectDirs, nil
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

// ExpandPath expands various path formats including ~, ., $ENV_VAR, and relative paths
func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}

	// Handle current directory
	if path == "." {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current working directory: %w", err)
		}
		return wd, nil
	}

	// Handle relative paths starting with "./"
	if strings.HasPrefix(path, "./") {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current working directory: %w", err)
		}
		return filepath.Join(wd, path[2:]), nil
	}

	// Handle home directory paths
	if strings.HasPrefix(path, "~") {
		return ExpandHomePath(path)
	}

	// Handle environment variable expansion
	if strings.Contains(path, "$") {
		return expandEnvVariables(path)
	}

	// Handle absolute paths and other cases
	if filepath.IsAbs(path) {
		return path, nil
	}

	// Convert relative path to absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for %s: %w", path, err)
	}
	return absPath, nil
}

// expandEnvVariables expands environment variables in path
func expandEnvVariables(path string) (string, error) {
	// Common Go environment variables that users might use
	envMap := map[string]func() (string, error){
		"$GOPATH":     GetGOPATH,
		"$GOMODCACHE": GetGOMODCACHE,
		"$HOME": func() (string, error) {
			return os.UserHomeDir()
		},
		"$PWD": func() (string, error) {
			return os.Getwd()
		},
	}

	// Replace known environment variables
	expandedPath := path
	for envVar, getFunc := range envMap {
		if strings.Contains(expandedPath, envVar) {
			value, err := getFunc()
			if err != nil {
				return "", fmt.Errorf("failed to get %s: %w", envVar, err)
			}
			expandedPath = strings.ReplaceAll(expandedPath, envVar, value)
		}
	}

	// Handle other environment variables like $VAR_NAME
	expandedPath = os.ExpandEnv(expandedPath)

	return expandedPath, nil
}

// ParseModulePaths parses and expands a comma-separated list of module paths
func ParseModulePaths(modulePathsStr string) ([]string, error) {
	if modulePathsStr == "" {
		return []string{}, nil
	}

	rawPaths := strings.Split(modulePathsStr, ",")
	var expandedPaths []string

	for _, rawPath := range rawPaths {
		trimmedPath := strings.TrimSpace(rawPath)
		if trimmedPath == "" {
			continue
		}

		expandedPath, err := ExpandPath(trimmedPath)
		if err != nil {
			return nil, fmt.Errorf("failed to expand path '%s': %w", trimmedPath, err)
		}

		// Verify the path exists
		if _, err := os.Stat(expandedPath); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("path does not exist: %s (expanded from %s)", expandedPath, trimmedPath)
			}
			return nil, fmt.Errorf("cannot access path %s: %w", expandedPath, err)
		}

		expandedPaths = append(expandedPaths, expandedPath)
	}

	return expandedPaths, nil
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
