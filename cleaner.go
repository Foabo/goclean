package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

// ModuleCleaner Go module cache cleaner
type ModuleCleaner struct {
	config           *Config
	usedModules      map[string]bool // Set of modules in use
	mutex            sync.RWMutex    // Protect concurrent access
	analyzedProjects map[string]bool // Cache for already analyzed projects
}

// ModuleInfo module information
type ModuleInfo struct {
	Path    string // Module path
	Version string // Version number
	Size    int64  // Size in bytes
	Type    string // Type: extracted or download
}

// NewModuleCleaner creates new cleaner instance
func NewModuleCleaner(config *Config) *ModuleCleaner {
	return &ModuleCleaner{
		config:           config,
		usedModules:      make(map[string]bool),
		analyzedProjects: make(map[string]bool), // Initialize here to avoid race condition
	}
}

// AnalyzeDependencies analyzes dependencies from all specified module paths
func (mc *ModuleCleaner) AnalyzeDependencies() error {
	if mc.config.Verbose {
		fmt.Printf("🔍 Analyzing %d module paths...\n", len(mc.config.ModulePaths))
	}

	// Use configurable worker pool for concurrent processing
	maxWorkers := mc.config.MaxWorkers
	semaphore := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	var analysisErrors []error
	var errorMutex sync.Mutex

	totalPaths := len(mc.config.ModulePaths)
	processed := int32(0)

	if mc.config.Verbose {
		fmt.Printf("Using %d concurrent workers for analysis\n", maxWorkers)
	}

	for i, modPath := range mc.config.ModulePaths {
		wg.Add(1)
		go func(index int, path string) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			if mc.config.Verbose {
				fmt.Printf("[%d/%d] Processing: %s\n", index+1, totalPaths, path)
			}

			if err := mc.analyzeModulePath(path); err != nil {
				errorMutex.Lock()
				analysisErrors = append(analysisErrors, fmt.Errorf("failed to analyze %s: %w", path, err))
				errorMutex.Unlock()
			}

			// Update progress
			current := atomic.AddInt32(&processed, 1)
			if mc.config.Verbose {
				fmt.Printf("✅ [%d/%d] Completed: %s\n", current, totalPaths, path)
			}
		}(i, modPath)
	}

	wg.Wait()

	// Report analysis results
	if mc.config.Verbose {
		fmt.Printf("📊 Analysis complete: %d projects processed\n", totalPaths)
		if len(analysisErrors) > 0 {
			fmt.Printf("⚠️  %d projects had analysis errors (non-fatal)\n", len(analysisErrors))
		}
	}

	return nil
}

// analyzeModulePath analyzes single module path
func (mc *ModuleCleaner) analyzeModulePath(modPath string) error {
	// Expand user directory
	if strings.HasPrefix(modPath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get user home directory: %w", err)
		}
		modPath = filepath.Join(homeDir, modPath[2:])
	}

	info, err := os.Stat(modPath)
	if err != nil {
		if mc.config.Verbose {
			fmt.Printf("Skipping non-existent path: %s\n", modPath)
		}
		return nil
	}

	if info.IsDir() {
		return mc.analyzeDirectory(modPath)
	} else {
		return mc.analyzeGoModFile(modPath)
	}
}

// analyzeDirectory recursively analyzes go.mod files in directory
func (mc *ModuleCleaner) analyzeDirectory(dirPath string) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Name() == "go.mod" {
			if mc.config.Verbose {
				fmt.Printf("Analyzing go.mod file: %s\n", path)
			}
			return mc.analyzeGoModFile(path)
		}

		return nil
	})
}

// analyzeGoModFile analyzes single go.mod file
func (mc *ModuleCleaner) analyzeGoModFile(goModPath string) error {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return fmt.Errorf("failed to read go.mod file: %w", err)
	}

	modFile, err := modfile.Parse(goModPath, content, nil)
	if err != nil {
		return fmt.Errorf("failed to parse go.mod file: %w", err)
	}

	// Add main module
	if modFile.Module != nil {
		mc.addUsedModule(modFile.Module.Mod.Path)
	}

	// Add direct dependencies
	for _, require := range modFile.Require {
		mc.addUsedModule(require.Mod.Path)
	}

	// Skip indirect dependencies analysis in fast mode
	if mc.config.FastMode {
		if mc.config.Verbose {
			fmt.Printf("    Fast mode: skipping indirect dependencies for %s\n", filepath.Dir(goModPath))
		}
		return nil
	}

	projectDir := filepath.Dir(goModPath)

	// Try static analysis methods first (they might provide additional info)
	goSumErr := mc.analyzeGoSumFile(projectDir)
	vendorErr := mc.analyzeVendorDirectory(projectDir)

	// Always try go list for complete dependency analysis, unless we're in fast mode
	// Static methods are supplementary, not replacement for go list
	if mc.config.Verbose {
		if goSumErr == nil {
			fmt.Printf("    Successfully supplemented with go.sum analysis\n")
		} else if vendorErr == nil {
			fmt.Printf("    Successfully supplemented with vendor analysis\n")
		} else {
			fmt.Printf("    No static dependency files found (go.sum/vendor), using go list only\n")
		}
	}

	// Always attempt go list for indirect dependencies (unless it times out)
	return mc.analyzeIndirectDependencies(projectDir)
}

// analyzeGoSumFile analyzes go.sum file to extract all used modules
func (mc *ModuleCleaner) analyzeGoSumFile(projectDir string) error {
	goSumPath := filepath.Join(projectDir, "go.sum")
	if !PathExists(goSumPath) {
		return nil // File doesn't exist, but this is not an error condition
	}

	if mc.config.Verbose {
		fmt.Printf("    Analyzing go.sum file: %s\n", goSumPath)
	}

	content, err := os.ReadFile(goSumPath)
	if err != nil {
		return fmt.Errorf("failed to read go.sum: %w", err)
	}

	moduleCount := 0
	processedModules := make(map[string]bool)

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// go.sum format: module version hash
		// Example: github.com/gin-gonic/gin v1.9.1 h1:abc123...
		// Also handle /go.mod entries: github.com/gin-gonic/gin v1.9.1/go.mod h1:def456...
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			modulePath := parts[0]

			if modulePath != "" && !processedModules[modulePath] {
				mc.addUsedModule(modulePath)
				processedModules[modulePath] = true
				moduleCount++

				if mc.config.Verbose && moduleCount <= 5 {
					fmt.Printf("      Added: %s\n", modulePath)
				}
			}
		}
	}

	if mc.config.Verbose {
		fmt.Printf("    Found %d unique modules from go.sum\n", moduleCount)
	}

	return nil // Always return success, even if no modules found
}

// analyzeVendorDirectory analyzes vendor directory to find used modules
func (mc *ModuleCleaner) analyzeVendorDirectory(projectDir string) error {
	vendorPath := filepath.Join(projectDir, "vendor")
	if !PathExists(vendorPath) {
		return nil // Directory doesn't exist, but this is not an error condition
	}

	if mc.config.Verbose {
		fmt.Printf("    Analyzing vendor directory: %s\n", vendorPath)
	}

	moduleCount := 0
	err := filepath.Walk(vendorPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if info.IsDir() {
			relPath, err := filepath.Rel(vendorPath, path)
			if err != nil {
				return nil
			}

			// Skip if this is the vendor root or a version directory
			if relPath == "." || strings.Contains(relPath, "@") {
				return nil
			}

			// Check if this looks like a module path
			if strings.Count(relPath, "/") >= 2 { // e.g., github.com/user/repo
				// Get the module root (first 3 path segments for github.com style)
				parts := strings.Split(relPath, "/")
				if len(parts) >= 3 {
					modulePath := strings.Join(parts[:3], "/")
					mc.addUsedModule(modulePath)
					moduleCount++
					return filepath.SkipDir // Don't descend into this module
				}
			}
		}

		return nil
	})

	if mc.config.Verbose {
		fmt.Printf("    Found %d modules from vendor directory\n", moduleCount)
	}

	return err // Return the walk error if any, but missing directory is not an error
}

// analyzeIndirectDependencies analyzes indirect dependencies (optimized version)
func (mc *ModuleCleaner) analyzeIndirectDependencies(projectDir string) error {
	absProjectDir, err := filepath.Abs(projectDir)
	if err != nil {
		absProjectDir = projectDir
	}

	// Protect analyzedProjects map access with mutex
	mc.mutex.Lock()
	if mc.analyzedProjects[absProjectDir] {
		mc.mutex.Unlock()
		if mc.config.Verbose {
			fmt.Printf("  Skipping already analyzed project: %s\n", projectDir)
		}
		return nil
	}
	mc.analyzedProjects[absProjectDir] = true
	mc.mutex.Unlock()

	if mc.config.Verbose {
		fmt.Printf("  Analyzing indirect dependencies for: %s\n", projectDir)
	}

	// Use configurable timeout from command line
	timeout := time.Duration(mc.config.Timeout) * time.Second

	// Allow environment variable to override if set
	if customTimeout := os.Getenv("GOCLEAN_TIMEOUT"); customTimeout != "" {
		if parsedTimeout, err := time.ParseDuration(customTimeout); err == nil {
			timeout = parsedTimeout
		}
	}

	// Create context with configurable timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-json", "all")
	cmd.Dir = projectDir

	// Always respect existing environment in enterprise settings
	cmd.Env = os.Environ()

	start := time.Now()
	output, err := cmd.Output()
	duration := time.Since(start)

	if mc.config.Verbose {
		fmt.Printf("    go list took: %v\n", duration)
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Printf("⚠️  Warning: go list timeout (%v) for %s - skipping indirect dependencies\n", timeout, projectDir)
			if mc.config.Verbose {
				fmt.Printf("    This is expected in enterprise environments with private repositories\n")
				fmt.Printf("    The tool will continue using go.mod + go.sum analysis only\n")
			}
		} else if mc.config.Verbose {
			fmt.Printf("    go list error for %s: %v\n", projectDir, err)
		}
		return nil // Not a fatal error, continue processing other projects
	}

	// Parse output
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	moduleCount := 0

	for decoder.More() {
		var mod struct {
			Path string `json:"Path"`
		}
		if err := decoder.Decode(&mod); err != nil {
			continue
		}
		if mod.Path != "" {
			mc.addUsedModule(mod.Path)
			moduleCount++
		}
	}

	if mc.config.Verbose {
		fmt.Printf("    Found %d modules via go list\n", moduleCount)
	}

	return nil
}

// addUsedModule adds module in use
func (mc *ModuleCleaner) addUsedModule(modPath string) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	mc.usedModules[modPath] = true
}

// isModuleUsed checks if module is in use
func (mc *ModuleCleaner) isModuleUsed(modPath string) bool {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()
	return mc.usedModules[modPath]
}

// FindUnusedModules finds unused modules with parallel processing
func (mc *ModuleCleaner) FindUnusedModules() ([]ModuleInfo, error) {
	if mc.config.Verbose {
		fmt.Println("Scanning module cache directory...")
	}

	// Use channels to collect results from parallel goroutines
	extractedChan := make(chan []ModuleInfo, 1)
	downloadedChan := make(chan []ModuleInfo, 1)
	errorChan := make(chan error, 2)

	// Start parallel scanning
	var wg sync.WaitGroup

	// Scan extracted modules in parallel
	wg.Add(1)
	go func() {
		defer wg.Done()
		if mc.config.Verbose {
			fmt.Println("  Scanning extracted modules...")
		}
		extractedModules, err := mc.findUnusedExtractedModules()
		if err != nil {
			errorChan <- fmt.Errorf("failed to scan extracted modules: %w", err)
			return
		}
		extractedChan <- extractedModules
		if mc.config.Verbose {
			fmt.Printf("  Found %d unused extracted modules\n", len(extractedModules))
		}
	}()

	// Scan downloaded modules in parallel
	wg.Add(1)
	go func() {
		defer wg.Done()
		if mc.config.Verbose {
			fmt.Println("  Scanning downloaded modules...")
		}
		downloadedModules, err := mc.findUnusedDownloadedModules()
		if err != nil {
			errorChan <- fmt.Errorf("failed to scan downloaded modules: %w", err)
			return
		}
		downloadedChan <- downloadedModules
		if mc.config.Verbose {
			fmt.Printf("  Found %d unused downloaded modules\n", len(downloadedModules))
		}
	}()

	// Wait for completion
	wg.Wait()
	close(errorChan)
	close(extractedChan)
	close(downloadedChan)

	// Check for errors
	for err := range errorChan {
		if err != nil {
			return nil, err
		}
	}

	// Combine results
	var unusedModules []ModuleInfo

	// Get extracted modules results
	if extractedModules := <-extractedChan; extractedModules != nil {
		unusedModules = append(unusedModules, extractedModules...)
	}

	// Get downloaded modules results
	if downloadedModules := <-downloadedChan; downloadedModules != nil {
		unusedModules = append(unusedModules, downloadedModules...)
	}

	// Sort by module path for consistent output
	sort.Slice(unusedModules, func(i, j int) bool {
		return unusedModules[i].Path < unusedModules[j].Path
	})

	if mc.config.Verbose {
		fmt.Printf("📊 Total unused modules found: %d\n", len(unusedModules))
	}

	return unusedModules, nil
}

// findUnusedExtractedModules finds unused extracted modules with concurrent size calculation
func (mc *ModuleCleaner) findUnusedExtractedModules() ([]ModuleInfo, error) {
	var modulesMutex sync.Mutex
	var modules []ModuleInfo
	cacheDir := mc.config.GoModCache

	// Channel to control concurrent size calculations
	sizeChan := make(chan ModuleInfo, 100) // Buffer for better performance
	var sizeWg sync.WaitGroup

	// Use configurable worker count, default to MaxWorkers for consistency
	sizeWorkers := mc.config.MaxWorkers
	if sizeWorkers > 10 {
		sizeWorkers = 10 // Cap at 10 for size calculations to avoid too much I/O contention
	}

	if mc.config.Verbose {
		fmt.Printf("    Using %d workers for size calculations\n", sizeWorkers)
	}

	// Worker pool for size calculations
	for i := 0; i < sizeWorkers; i++ {
		sizeWg.Add(1)
		go func() {
			defer sizeWg.Done()
			for moduleInfo := range sizeChan {
				// Calculate directory size
				size, _ := mc.calculateDirectorySize(moduleInfo.Path) // Path contains full directory path temporarily

				// Update the module info with calculated size and correct path
				modulesMutex.Lock()
				for i := range modules {
					if modules[i].Path == moduleInfo.Path { // Using full path as identifier temporarily
						modules[i].Size = size
						break
					}
				}
				modulesMutex.Unlock()
			}
		}()
	}

	err := filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip cache/download directory
		if strings.Contains(path, "cache/download") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() && strings.Contains(info.Name(), "@") {
			// This is a versioned module directory, e.g., github.com/gin-gonic/gin@v1.9.1
			modPath, version := mc.parseExtractedModulePath(path, cacheDir)
			if modPath != "" && !mc.isModuleUsed(modPath) {
				// Add module to list first (with placeholder size)
				moduleInfo := ModuleInfo{
					Path:    path, // Temporarily store full path for worker identification
					Version: version,
					Size:    0, // Will be calculated by workers
					Type:    "extracted",
				}

				modulesMutex.Lock()
				modules = append(modules, moduleInfo)
				modulesMutex.Unlock()

				// Send to size calculation workers
				sizeChan <- moduleInfo

				return filepath.SkipDir // Skip subdirectories
			}
		}

		return nil
	})

	// Close the channel and wait for all size calculations to complete
	close(sizeChan)
	sizeWg.Wait()

	// Fix the Path field to contain module path instead of full directory path
	modulesMutex.Lock()
	for i := range modules {
		modPath, _ := mc.parseExtractedModulePath(modules[i].Path, cacheDir)
		modules[i].Path = modPath
	}
	modulesMutex.Unlock()

	return modules, err
}

// findUnusedDownloadedModules finds unused downloaded modules with concurrent processing
func (mc *ModuleCleaner) findUnusedDownloadedModules() ([]ModuleInfo, error) {
	var modulesMutex sync.Mutex
	var modules []ModuleInfo
	downloadDir := filepath.Join(mc.config.GoModCache, "cache", "download")

	if _, err := os.Stat(downloadDir); os.IsNotExist(err) {
		return modules, nil
	}

	// Use map to track processed modules and avoid duplicates
	processedModules := make(map[string]*ModuleInfo) // key: modPath@version

	// Channel for concurrent size calculations
	sizeChan := make(chan string, 100) // Send module key for size calculation
	var sizeWg sync.WaitGroup

	// Use configurable worker count, but cap it for download size calculations
	sizeWorkers := mc.config.MaxWorkers / 2 // Use half of MaxWorkers for download calculations
	if sizeWorkers < 2 {
		sizeWorkers = 2 // Minimum 2 workers
	}
	if sizeWorkers > 6 {
		sizeWorkers = 6 // Cap at 6 for download calculations (less I/O intensive)
	}

	if mc.config.Verbose {
		fmt.Printf("    Using %d workers for download size calculations\n", sizeWorkers)
	}

	// Worker pool for size calculations
	for i := 0; i < sizeWorkers; i++ {
		sizeWg.Add(1)
		go func() {
			defer sizeWg.Done()
			for moduleKey := range sizeChan {
				modulesMutex.Lock()
				if moduleInfo, exists := processedModules[moduleKey]; exists {
					// Calculate size for this module
					modPath := moduleInfo.Path
					size := mc.calculateModuleDownloadSize("", downloadDir, modPath)
					moduleInfo.Size = size
				}
				modulesMutex.Unlock()
			}
		}()
	}

	err := filepath.Walk(downloadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && (strings.HasSuffix(info.Name(), ".mod") ||
			strings.HasSuffix(info.Name(), ".zip") ||
			strings.HasSuffix(info.Name(), ".info")) {

			modPath := mc.parseDownloadedModulePath(path, downloadDir)
			if modPath != "" && !mc.isModuleUsed(modPath) {
				version := mc.extractVersionFromPath(path)
				moduleKey := modPath + "@" + version

				modulesMutex.Lock()
				// Only add if not already processed
				if _, exists := processedModules[moduleKey]; !exists {
					moduleInfo := &ModuleInfo{
						Path:    modPath,
						Version: version,
						Size:    0, // Will be calculated by workers
						Type:    "download",
					}
					processedModules[moduleKey] = moduleInfo

					// Send for size calculation
					sizeChan <- moduleKey
				}
				modulesMutex.Unlock()
			}
		}

		return nil
	})

	// Close channel and wait for size calculations
	close(sizeChan)
	sizeWg.Wait()

	// Convert map to slice
	modulesMutex.Lock()
	for _, moduleInfo := range processedModules {
		modules = append(modules, *moduleInfo)
	}
	modulesMutex.Unlock()

	return modules, err
}

// parseExtractedModulePath parses extracted module path
func (mc *ModuleCleaner) parseExtractedModulePath(fullPath, cacheDir string) (modPath, version string) {
	relativePath, err := filepath.Rel(cacheDir, fullPath)
	if err != nil {
		return "", ""
	}

	// Decode module path
	parts := strings.Split(relativePath, "@")
	if len(parts) != 2 {
		return "", ""
	}

	modPath, err = module.UnescapePath(parts[0])
	if err != nil {
		return "", ""
	}

	version = parts[1]
	return modPath, version
}

// parseDownloadedModulePath parses downloaded module path
func (mc *ModuleCleaner) parseDownloadedModulePath(fullPath, downloadDir string) string {
	relativePath, err := filepath.Rel(downloadDir, fullPath)
	if err != nil {
		return ""
	}

	// The structure is typically .../module/path/@v/version.zip
	// We need to extract the 'module/path' part.
	dir := filepath.Dir(relativePath)

	// Remove the version part (e.g., @v)
	if strings.HasSuffix(dir, "@v") {
		dir = strings.TrimSuffix(dir, "@v")
		// Also trim any path separator that might be left
		dir = strings.TrimSuffix(dir, string(filepath.Separator))
	} else if strings.Contains(dir, "@") {
		// Handle cases where the structure might be different but still contains '@'
		// This is a safeguard, the primary logic targets the '@v' suffix.
		parts := strings.Split(dir, "@")
		dir = parts[0]
	}

	if dir == "." || dir == "" {
		return ""
	}

	modPath, err := module.UnescapePath(dir)
	if err != nil {
		return ""
	}

	return modPath
}

// extractVersionFromPath extracts version information from path
func (mc *ModuleCleaner) extractVersionFromPath(path string) string {
	fileName := filepath.Base(path)
	ext := filepath.Ext(fileName)
	nameWithoutExt := strings.TrimSuffix(fileName, ext)

	if strings.HasPrefix(nameWithoutExt, "v") {
		return nameWithoutExt
	}

	return ""
}

// calculateModuleDownloadSize calculates total size of downloaded module
func (mc *ModuleCleaner) calculateModuleDownloadSize(samplePath, downloadDir, modPath string) int64 {
	var totalSize int64

	escapedPath, err := module.EscapePath(modPath)
	if err != nil {
		return 0
	}

	modDir := filepath.Join(downloadDir, escapedPath)

	filepath.Walk(modDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	return totalSize
}

// RemoveUnusedModules removes unused modules
func (mc *ModuleCleaner) RemoveUnusedModules(modules []ModuleInfo) error {
	if mc.config.DryRun {
		fmt.Println("Dry run mode: The following modules would be deleted")

		var totalSizeToDelete int64
		for _, mod := range modules {
			fmt.Printf("  - %s@%s (%s)\n", mod.Path, mod.Version, FormatSize(mod.Size))
			totalSizeToDelete += mod.Size
		}

		// Calculate current cache size for dry-run statistics
		fmt.Println("\n📊 Calculating current cache statistics...")
		cacheSizeCurrent, err := CalculateCacheSize(mc.config.GoModCache)
		if err != nil {
			fmt.Printf("Warning: failed to calculate current cache size: %v\n", err)
		} else {
			fmt.Printf("\n🎯 Dry Run Summary:\n")
			fmt.Printf("  📦 Current cache size: %s\n", FormatSize(cacheSizeCurrent))
			fmt.Printf("  🧹 Would free space: %s\n", FormatSize(totalSizeToDelete))
			if cacheSizeCurrent > 0 {
				percentage := float64(totalSizeToDelete) / float64(cacheSizeCurrent) * 100
				fmt.Printf("  📊 Percentage of cache: %.1f%%\n", percentage)
			}
		}

		return nil
	}

	// Calculate cache size before deletion
	fmt.Println("📊 Calculating cache statistics...")
	cacheSizeBefore, err := CalculateCacheSize(mc.config.GoModCache)
	if err != nil {
		fmt.Printf("Warning: failed to calculate cache size before deletion: %v\n", err)
		cacheSizeBefore = 0
	}

	if mc.config.Verbose {
		fmt.Printf("Cache size before deletion: %s\n", FormatSize(cacheSizeBefore))
		fmt.Printf("Starting to delete %d unused modules...\n", len(modules))
	}

	// Start timing
	startTime := time.Now()

	var errors []string
	deletedCount := 0
	for _, mod := range modules {
		if err := mc.removeModule(mod); err != nil {
			errors = append(errors, fmt.Sprintf("failed to delete module %s: %v", mod.Path, err))
			if mc.config.Verbose {
				fmt.Printf("Failed to delete: %s@%s - %v\n", mod.Path, mod.Version, err)
			}
		} else {
			deletedCount++
			if mc.config.Verbose {
				fmt.Printf("Deleted: %s@%s (%s)\n", mod.Path, mod.Version, FormatSize(mod.Size))
			}
		}
	}

	// Calculate elapsed time
	elapsed := time.Since(startTime)

	// Calculate cache size after deletion
	cacheSizeAfter, err := CalculateCacheSize(mc.config.GoModCache)
	if err != nil {
		fmt.Printf("Warning: failed to calculate cache size after deletion: %v\n", err)
		cacheSizeAfter = cacheSizeBefore
	}

	// Display statistics
	fmt.Println("\n🎯 Deletion Summary:")
	fmt.Printf("  ✅ Modules deleted: %d/%d\n", deletedCount, len(modules))
	fmt.Printf("  ⏱️  Time taken: %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("  📦 Cache size before: %s\n", FormatSize(cacheSizeBefore))
	fmt.Printf("  📦 Cache size after: %s\n", FormatSize(cacheSizeAfter))

	if cacheSizeBefore > 0 {
		spaceFreed := cacheSizeBefore - cacheSizeAfter
		percentage := float64(spaceFreed) / float64(cacheSizeBefore) * 100
		fmt.Printf("  🧹 Space freed: %s (%.1f%%)\n", FormatSize(spaceFreed), percentage)
	}

	if len(errors) > 0 {
		fmt.Printf("  ❌ Errors: %d\n", len(errors))
		return fmt.Errorf("errors occurred during deletion:\n%s", strings.Join(errors, "\n"))
	}

	return nil
}

// removeModule removes single module
func (mc *ModuleCleaner) removeModule(mod ModuleInfo) error {
	switch mod.Type {
	case "extracted":
		return mc.removeExtractedModule(mod)
	case "download":
		return mc.removeDownloadedModule(mod)
	default:
		return fmt.Errorf("unknown module type: %s", mod.Type)
	}
}

// removeExtractedModule removes extracted module
func (mc *ModuleCleaner) removeExtractedModule(mod ModuleInfo) error {
	escapedPath, err := module.EscapePath(mod.Path)
	if err != nil {
		return err
	}

	modDir := filepath.Join(mc.config.GoModCache, escapedPath+"@"+mod.Version)

	// Make files writable before deletion to avoid permission errors
	if err := mc.makeDirectoryWritable(modDir); err != nil {
		if mc.config.Verbose {
			fmt.Printf("Warning: failed to make directory writable: %v\n", err)
		}
		// Continue with removal attempt even if chmod fails
	}

	return os.RemoveAll(modDir)
}

// removeDownloadedModule removes downloaded module
func (mc *ModuleCleaner) removeDownloadedModule(mod ModuleInfo) error {
	escapedPath, err := module.EscapePath(mod.Path)
	if err != nil {
		return err
	}

	downloadDir := filepath.Join(mc.config.GoModCache, "cache", "download", escapedPath)
	return os.RemoveAll(downloadDir)
}

// makeDirectoryWritable recursively makes directory and files writable for deletion
func (mc *ModuleCleaner) makeDirectoryWritable(dirPath string) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		// Make directories and files writable
		if info.IsDir() {
			return os.Chmod(path, 0o755) // rwxr-xr-x for directories
		} else {
			return os.Chmod(path, 0o644) // rw-r--r-- for files
		}
	})
}

// ShowInteractiveMenu displays interactive menu
func (mc *ModuleCleaner) ShowInteractiveMenu(unusedModules []ModuleInfo) error {
	if len(unusedModules) == 0 {
		fmt.Println("Great! No unused modules found.")
		return nil
	}

	totalSize := int64(0)
	for _, mod := range unusedModules {
		totalSize += mod.Size
	}

	viewedDetails := false

	// Loop until user chooses to exit or delete
	for {
		fmt.Printf("Found %d unused modules, occupying %s disk space.\n\n",
			len(unusedModules), FormatSize(totalSize))

		fmt.Println("You can:")
		if !viewedDetails {
			fmt.Println("(1) View details")
			fmt.Println("(2) Delete these modules (requires administrator privileges)")
			fmt.Println("(3) Exit")
		} else {
			fmt.Println("(1) Delete these modules (requires administrator privileges)")
			fmt.Println("(2) Exit")
		}
		fmt.Print("\nPlease enter the number in parentheses: ")

		reader := bufio.NewReader(os.Stdin)
		choice, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read user input: %w", err)
		}

		choice = strings.TrimSpace(choice)

		if !viewedDetails {
			// First time menu with view details option
			switch choice {
			case "1":
				if err := mc.showModuleDetails(unusedModules); err != nil {
					return err
				}
				viewedDetails = true
				fmt.Println()
			case "2":
				return mc.confirmAndRemove(unusedModules)
			case "3":
				fmt.Println("Exit.")
				return nil
			default:
				fmt.Println("Invalid choice, please try again.")
				fmt.Println()
			}
		} else {
			// After viewing details, simplified menu
			switch choice {
			case "1":
				return mc.confirmAndRemove(unusedModules)
			case "2":
				fmt.Println("Exit.")
				return nil
			default:
				fmt.Println("Invalid choice, please try again.")
				fmt.Println()
			}
		}
	}
}

// showModuleDetails displays module detailed information
func (mc *ModuleCleaner) showModuleDetails(modules []ModuleInfo) error {
	fmt.Println("\nUnused modules detailed information:")
	fmt.Println(strings.Repeat("-", 80))

	// Group modules by path
	moduleGroups := make(map[string][]ModuleInfo)
	for _, mod := range modules {
		moduleGroups[mod.Path] = append(moduleGroups[mod.Path], mod)
	}

	// Sort module paths
	var paths []string
	for path := range moduleGroups {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	// Display grouped modules
	for _, path := range paths {
		versions := moduleGroups[path]

		// Calculate total size for this module
		var totalSize int64
		for _, v := range versions {
			totalSize += v.Size
		}

		fmt.Printf("📦 Module: %s (Total: %s)\n", path, FormatSize(totalSize))

		// Show cache path for this module
		cachePath := mc.getModuleCachePath(path)
		fmt.Printf("   📁 Cache: %s\n", cachePath)

		// Sort versions for consistent display
		sort.Slice(versions, func(i, j int) bool {
			return versions[i].Version < versions[j].Version
		})

		for _, v := range versions {
			// Convert type to more intuitive description
			var typeDesc string
			switch v.Type {
			case "extracted":
				typeDesc = "removable"
			case "download":
				typeDesc = "removable"
			default:
				typeDesc = v.Type
			}

			fmt.Printf("   ├─ v%s (%s, %s)\n",
				strings.TrimPrefix(v.Version, "v"),
				FormatSize(v.Size),
				typeDesc)
		}
		fmt.Println()
	}

	return nil
}

// getModuleCachePath returns the cache directory path for a module
func (mc *ModuleCleaner) getModuleCachePath(modPath string) string {
	escapedPath, err := module.EscapePath(modPath)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	// Check both extracted and download paths
	extractedPath := filepath.Join(mc.config.GoModCache, escapedPath)
	downloadPath := filepath.Join(mc.config.GoModCache, "cache", "download", escapedPath)

	// Try to show relative path from GOMODCACHE for better readability
	gomodcache := mc.config.GoModCache

	// Check which type exists and show the primary location
	if PathExists(extractedPath) {
		relPath, err := filepath.Rel(gomodcache, extractedPath)
		if err == nil {
			return fmt.Sprintf("$GOMODCACHE/%s", relPath)
		}
		return extractedPath
	}

	if PathExists(downloadPath) {
		relPath, err := filepath.Rel(gomodcache, downloadPath)
		if err == nil {
			return fmt.Sprintf("$GOMODCACHE/%s", relPath)
		}
		return downloadPath
	}

	// If neither exists, show the expected extracted path
	relPath, err := filepath.Rel(gomodcache, extractedPath)
	if err == nil {
		return fmt.Sprintf("$GOMODCACHE/%s", relPath)
	}
	return extractedPath
}

// confirmAndRemove confirms and removes modules
func (mc *ModuleCleaner) confirmAndRemove(modules []ModuleInfo) error {
	fmt.Print("\nConfirm deletion of these modules? (y/N): ")

	reader := bufio.NewReader(os.Stdin)
	confirm, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read confirmation: %w", err)
	}

	confirm = strings.ToLower(strings.TrimSpace(confirm))
	if confirm != "y" && confirm != "yes" {
		fmt.Println("Deletion cancelled.")
		return nil
	}

	return mc.RemoveUnusedModules(modules)
}
