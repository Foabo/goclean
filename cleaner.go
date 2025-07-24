package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/module"
)

// SemanticVersion represents a semantic version
type SemanticVersion struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string
	Build      string
	Original   string
}

// ParseSemanticVersion parses a semantic version string
func ParseSemanticVersion(version string) (*SemanticVersion, error) {
	if version == "" {
		return nil, fmt.Errorf("empty version string")
	}

	original := version
	// Remove 'v' prefix if present
	version = strings.TrimPrefix(version, "v")

	// Split pre-release and build metadata
	var preRelease, build string

	// Handle build metadata (+)
	if plusIndex := strings.Index(version, "+"); plusIndex != -1 {
		build = version[plusIndex+1:]
		version = version[:plusIndex]
	}

	// Handle pre-release (-)
	if dashIndex := strings.Index(version, "-"); dashIndex != -1 {
		preRelease = version[dashIndex+1:]
		version = version[:dashIndex]
	}

	// Parse major.minor.patch
	parts := strings.Split(version, ".")
	if len(parts) < 1 || len(parts) > 3 {
		return nil, fmt.Errorf("invalid version format: %s", original)
	}

	// Ensure we have 3 parts (major.minor.patch)
	for len(parts) < 3 {
		parts = append(parts, "0")
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %s", parts[0])
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version: %s", parts[1])
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid patch version: %s", parts[2])
	}

	return &SemanticVersion{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		PreRelease: preRelease,
		Build:      build,
		Original:   original,
	}, nil
}

// Compare compares two semantic versions
// Returns: -1 if v < other, 0 if v == other, 1 if v > other
func (v *SemanticVersion) Compare(other *SemanticVersion) int {
	// Compare major
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}

	// Compare minor
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}

	// Compare patch
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}

	// Compare pre-release
	if v.PreRelease == "" && other.PreRelease != "" {
		return 1 // Release > pre-release
	}
	if v.PreRelease != "" && other.PreRelease == "" {
		return -1 // Pre-release < release
	}
	if v.PreRelease != other.PreRelease {
		if v.PreRelease < other.PreRelease {
			return -1
		}
		return 1
	}

	return 0 // Equal
}

// String returns the string representation of the version
func (v *SemanticVersion) String() string {
	return v.Original
}

// ModuleEntry represents a module with complete metadata from go list
type ModuleEntry struct {
	Path      string `json:"Path"`      // Module path
	Version   string `json:"Version"`   // Module version
	Time      string `json:"Time"`      // Module timestamp (RFC3339 string)
	Indirect  bool   `json:"Indirect"`  // Whether this is an indirect dependency
	Dir       string `json:"Dir"`       // Path to extracted module directory
	GoMod     string `json:"GoMod"`     // Path to .mod file in download cache
	GoVersion string `json:"GoVersion"` // Required Go version
	Sum       string `json:"Sum"`       // Module content checksum
	GoModSum  string `json:"GoModSum"`  // go.mod file checksum
}

// ModuleCleaner simplified module cache cleaner
type ModuleCleaner struct {
	config   *Config
	analyzer *DependencyAnalyzer
}

// NewModuleCleaner creates new cleaner instance
func NewModuleCleaner(config *Config) *ModuleCleaner {
	return &ModuleCleaner{
		config:   config,
		analyzer: NewDependencyAnalyzer(config),
	}
}

// AnalyzeDependencies analyzes dependencies using the dependency analyzer
func (mc *ModuleCleaner) AnalyzeDependencies() error {
	_, err := mc.analyzer.AnalyzeAllProjects()
	return err
}

// shouldKeepVersion determines if a specific version should be kept (simplified version)
func (mc *ModuleCleaner) shouldKeepVersion(modPath, version string) bool {
	usedModules := mc.analyzer.GetUsedModules()
	moduleEntries := mc.analyzer.GetModuleEntries()

	// Strategy 1: Always keep if explicitly used
	if versions, exists := usedModules[modPath]; exists {
		if versions[version] {
			return true
		}
	}

	// Strategy 2: Enhanced logic using ModuleEntry metadata
	moduleKey := fmt.Sprintf("%s@%s", modPath, version)
	if entry, hasMetadata := moduleEntries[moduleKey]; hasMetadata {
		// Use intelligent cleaning based on direct/indirect status
		if entry.Indirect {
			return mc.shouldKeepIndirectDependency(modPath, version, usedModules)
		} else {
			return mc.shouldKeepDirectDependency(modPath, version, usedModules)
		}
	}

	// Strategy 3: Fallback to basic version logic
	return mc.shouldKeepVersionBasic(modPath, version, usedModules)
}

// shouldKeepVersionBasic implements basic version keeping logic
func (mc *ModuleCleaner) shouldKeepVersionBasic(modPath, version string, usedModules map[string]map[string]bool) bool {
	usedVersions := mc.getUsedVersionsFromMap(modPath, usedModules)
	if len(usedVersions) == 0 {
		return false // No versions in use, can remove
	}

	currentVersion, err := ParseSemanticVersion(version)
	if err != nil {
		return true // Keep if can't parse version for safety
	}

	latestRequired, err := mc.findLatestRequiredVersion(modPath, usedVersions)
	if err != nil {
		return true // Keep if can't determine latest for safety
	}

	// Keep only the latest required version and newer
	return currentVersion.Compare(latestRequired) >= 0
}

// shouldKeepIndirectDependency implements aggressive cleaning for indirect dependencies
func (mc *ModuleCleaner) shouldKeepIndirectDependency(modPath, version string, usedModules map[string]map[string]bool) bool {
	// For indirect dependencies, be more aggressive
	if _, exists := usedModules[modPath]; !exists {
		return false // Module not used anywhere
	}

	usedVersions := mc.getUsedVersionsFromMap(modPath, usedModules)
	if len(usedVersions) == 0 {
		return false
	}

	currentVersion, err := ParseSemanticVersion(version)
	if err != nil {
		return true // Keep if can't parse for safety
	}

	latestRequired, err := mc.findLatestRequiredVersion(modPath, usedVersions)
	if err != nil {
		return true // Keep if can't determine latest for safety
	}

	// Aggressive: Only keep if this IS the latest required version
	return currentVersion.Compare(latestRequired) == 0
}

// shouldKeepDirectDependency implements conservative cleaning for direct dependencies
func (mc *ModuleCleaner) shouldKeepDirectDependency(modPath, version string, usedModules map[string]map[string]bool) bool {
	// For direct dependencies, be more conservative
	if versions, exists := usedModules[modPath]; exists {
		if versions[version] {
			return true // Always keep if explicitly listed
		}
	}

	if _, exists := usedModules[modPath]; !exists {
		return false // Module not used anywhere
	}

	usedVersions := mc.getUsedVersionsFromMap(modPath, usedModules)
	if len(usedVersions) == 0 {
		return false
	}

	currentVersion, err := ParseSemanticVersion(version)
	if err != nil {
		return true // Keep if can't parse for safety
	}

	latestRequired, err := mc.findLatestRequiredVersion(modPath, usedVersions)
	if err != nil {
		return true // Keep if can't determine latest for safety
	}

	// Conservative: Keep latest required version and newer (for compatibility)
	return currentVersion.Compare(latestRequired) >= 0
}

// getUsedVersionsFromMap gets all versions in use for a module from the provided map
func (mc *ModuleCleaner) getUsedVersionsFromMap(modPath string, usedModules map[string]map[string]bool) []string {
	var versions []string
	if moduleVersions, exists := usedModules[modPath]; exists {
		for version, used := range moduleVersions {
			if used && version != "" {
				versions = append(versions, version)
			}
		}
	}
	return versions
}

// findLatestRequiredVersion finds the latest version among all used versions
func (mc *ModuleCleaner) findLatestRequiredVersion(modPath string, usedVersions []string) (*SemanticVersion, error) {
	var latestVersion *SemanticVersion

	for _, version := range usedVersions {
		parsedVersion, err := ParseSemanticVersion(version)
		if err != nil {
			continue // Skip unparseable versions
		}

		if latestVersion == nil || parsedVersion.Compare(latestVersion) > 0 {
			latestVersion = parsedVersion
		}
	}

	if latestVersion == nil {
		return nil, fmt.Errorf("no valid semantic versions found for module %s", modPath)
	}

	return latestVersion, nil
}

// ModuleInfo module information
type ModuleInfo struct {
	Path    string // Module path
	Version string // Version number
	Size    int64  // Size in bytes
	Type    string // Type: extracted or download
}

// VCSCacheInfo VCS cache information
type VCSCacheInfo struct {
	Hash     string // VCS cache hash
	RepoURL  string // Repository URL
	Size     int64  // Size in bytes
	LastUsed string // Last access time (if available)
}

// FindVCSCache finds VCS cache entries
func (mc *ModuleCleaner) FindVCSCache() ([]VCSCacheInfo, error) {
	vcsDir := filepath.Join(mc.config.GoModCache, "cache", "vcs")

	// Check if VCS cache directory exists
	if _, err := os.Stat(vcsDir); os.IsNotExist(err) {
		if mc.config.Verbose {
			fmt.Println("📂 No VCS cache directory found")
		}
		return []VCSCacheInfo{}, nil
	}

	if mc.config.Verbose {
		fmt.Println("🔍 Scanning VCS cache directory...")
	}

	var vcsCache []VCSCacheInfo
	var vcsMutex sync.Mutex

	err := filepath.Walk(vcsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Look for .info files that contain repository information
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".info") {
			hash := strings.TrimSuffix(info.Name(), ".info")
			repoDir := filepath.Join(vcsDir, hash)

			// Check if corresponding repository directory exists
			if _, err := os.Stat(repoDir); err == nil {
				size, _ := mc.calculateDirectorySize(repoDir)

				// Try to read repository URL from .info file
				repoURL := mc.readVCSRepoURL(path)

				// Get last access time
				lastUsed := info.ModTime().Format("2006-01-02")

				vcsCacheInfo := VCSCacheInfo{
					Hash:     hash,
					RepoURL:  repoURL,
					Size:     size,
					LastUsed: lastUsed,
				}

				vcsMutex.Lock()
				vcsCache = append(vcsCache, vcsCacheInfo)
				vcsMutex.Unlock()
			}
		}

		return nil
	})

	if mc.config.Verbose {
		fmt.Printf("📊 Found %d VCS cache entries\n", len(vcsCache))
	}

	return vcsCache, err
}

// readVCSRepoURL reads repository URL from VCS .info file
func (mc *ModuleCleaner) readVCSRepoURL(infoPath string) string {
	content, err := os.ReadFile(infoPath)
	if err != nil {
		return "unknown"
	}

	// The .info file typically contains the repository URL
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "http") || strings.Contains(line, "github.com") {
			return line
		}
	}

	return "unknown"
}

// RemoveVCSCache removes VCS cache entries
func (mc *ModuleCleaner) RemoveVCSCache(vcsCache []VCSCacheInfo) error {
	if mc.config.DryRun {
		fmt.Println("Dry run mode: The following VCS cache entries would be deleted")

		var totalSizeToDelete int64
		for _, cache := range vcsCache {
			fmt.Printf("  - %s (%s) [%s]\n", cache.Hash[:12], FormatSize(cache.Size), cache.RepoURL)
			totalSizeToDelete += cache.Size
		}

		fmt.Printf("\n🎯 VCS Cache Dry Run Summary:\n")
		fmt.Printf("  📦 VCS entries to delete: %d\n", len(vcsCache))
		fmt.Printf("  🧹 Would free space: %s\n", FormatSize(totalSizeToDelete))

		return nil
	}

	if mc.config.Verbose {
		fmt.Printf("Starting to delete %d VCS cache entries...\n", len(vcsCache))
	}

	// Start timing
	startTime := time.Now()

	var errors []string
	deletedCount := 0
	var totalSizeDeleted int64

	vcsDir := filepath.Join(mc.config.GoModCache, "cache", "vcs")

	for _, cache := range vcsCache {
		// Remove the repository directory
		repoDir := filepath.Join(vcsDir, cache.Hash)
		infoFile := filepath.Join(vcsDir, cache.Hash+".info")
		lockFile := filepath.Join(vcsDir, cache.Hash+".lock")

		// Remove repository directory
		if err := os.RemoveAll(repoDir); err != nil {
			errors = append(errors, fmt.Sprintf("failed to delete VCS repo %s: %v", cache.Hash[:12], err))
			continue
		}

		// Remove info file
		if err := os.Remove(infoFile); err != nil && !os.IsNotExist(err) {
			if mc.config.Verbose {
				fmt.Printf("Warning: failed to remove info file %s: %v\n", infoFile, err)
			}
		}

		// Remove lock file
		if err := os.Remove(lockFile); err != nil && !os.IsNotExist(err) {
			if mc.config.Verbose {
				fmt.Printf("Warning: failed to remove lock file %s: %v\n", lockFile, err)
			}
		}

		deletedCount++
		totalSizeDeleted += cache.Size

		if mc.config.Verbose {
			fmt.Printf("Deleted VCS cache: %s (%s) [%s]\n", cache.Hash[:12], FormatSize(cache.Size), cache.RepoURL)
		}
	}

	// Calculate elapsed time
	elapsed := time.Since(startTime)

	// Display statistics
	fmt.Println("\n🎯 VCS Cache Deletion Summary:")
	fmt.Printf("  ✅ VCS entries deleted: %d/%d\n", deletedCount, len(vcsCache))
	fmt.Printf("  ⏱️  Time taken: %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("  🧹 Space freed: %s\n", FormatSize(totalSizeDeleted))

	if len(errors) > 0 {
		fmt.Printf("  ❌ Errors: %d\n", len(errors))
		return fmt.Errorf("errors occurred during VCS cache deletion:\n%s", strings.Join(errors, "\n"))
	}

	return nil
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
			if modPath != "" && !mc.shouldKeepVersion(modPath, version) {
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
					size := mc.calculateModuleDownloadSize(downloadDir, modPath)
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
			if modPath != "" {
				version := mc.extractVersionFromPath(path)
				if !mc.shouldKeepVersion(modPath, version) {
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
func (mc *ModuleCleaner) calculateModuleDownloadSize(downloadDir, modPath string) int64 {
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

// removeModule removes single module using intelligent method
func (mc *ModuleCleaner) removeModule(mod ModuleInfo) error {
	// Use traditional type-based removal
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
	// Also scan for VCS cache
	vcsCache, err := mc.FindVCSCache()
	if err != nil {
		if mc.config.Verbose {
			fmt.Printf("Warning: failed to scan VCS cache: %v\n", err)
		}
		vcsCache = []VCSCacheInfo{} // Continue without VCS cache
	}

	if len(unusedModules) == 0 && len(vcsCache) == 0 {
		fmt.Println("Great! No unused modules or VCS cache found.")
		return nil
	}

	totalModuleSize := int64(0)
	for _, mod := range unusedModules {
		totalModuleSize += mod.Size
	}

	totalVCSSize := int64(0)
	for _, cache := range vcsCache {
		totalVCSSize += cache.Size
	}

	viewedDetails := false

	// Loop until user chooses to exit or delete
	for {
		fmt.Printf("Cleanup Summary:\n")
		if len(unusedModules) > 0 {
			fmt.Printf("  📦 Found %d unused modules, occupying %s disk space\n",
				len(unusedModules), FormatSize(totalModuleSize))
		}
		if len(vcsCache) > 0 {
			fmt.Printf("  🗂️  Found %d VCS cache entries, occupying %s disk space\n",
				len(vcsCache), FormatSize(totalVCSSize))
		}
		fmt.Printf("  🧹 Total space that can be freed: %s\n\n",
			FormatSize(totalModuleSize+totalVCSSize))

		fmt.Println("You can:")
		optionNum := 1

		if !viewedDetails {
			fmt.Printf("(%d) View details\n", optionNum)
			optionNum++
		}

		if len(unusedModules) > 0 {
			fmt.Printf("(%d) Delete unused modules only (%s)\n", optionNum, FormatSize(totalModuleSize))
			optionNum++
		}

		if len(vcsCache) > 0 {
			fmt.Printf("(%d) Delete VCS cache only (%s)\n", optionNum, FormatSize(totalVCSSize))
			optionNum++
		}

		if len(unusedModules) > 0 && len(vcsCache) > 0 {
			fmt.Printf("(%d) Delete both modules and VCS cache (%s)\n", optionNum, FormatSize(totalModuleSize+totalVCSSize))
			optionNum++
		}

		fmt.Printf("(%d) Exit\n", optionNum)

		fmt.Print("\nPlease enter the number in parentheses: ")

		reader := bufio.NewReader(os.Stdin)
		choice, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read user input: %w", err)
		}

		choice = strings.TrimSpace(choice)

		err = mc.handleMenuChoice(choice, unusedModules, vcsCache, &viewedDetails)
		if err != nil {
			// Check if it's a continue signal
			if _, isContinue := err.(continueMenuError); isContinue {
				continue // Continue the menu loop
			}
			return err // Return actual errors
		}

		// If we reach here, it means user chose exit or deletion (both should exit)
		return nil
	}
}

// handleMenuChoice handles user menu choice
func (mc *ModuleCleaner) handleMenuChoice(choice string, unusedModules []ModuleInfo, vcsCache []VCSCacheInfo, viewedDetails *bool) error {
	optionNum := 1

	// View details option
	if !*viewedDetails {
		if choice == fmt.Sprintf("%d", optionNum) {
			if err := mc.showModuleDetails(unusedModules); err != nil {
				return err
			}
			if len(vcsCache) > 0 {
				mc.showVCSDetails(vcsCache)
			}
			*viewedDetails = true
			fmt.Println()
			return continueMenuError{}
		}
		optionNum++
	}

	// Delete unused modules only
	if len(unusedModules) > 0 {
		if choice == fmt.Sprintf("%d", optionNum) {
			return mc.confirmAndRemove(unusedModules)
		}
		optionNum++
	}

	// Delete VCS cache only
	if len(vcsCache) > 0 {
		if choice == fmt.Sprintf("%d", optionNum) {
			return mc.confirmAndRemoveVCSCache(vcsCache)
		}
		optionNum++
	}

	// Delete both
	if len(unusedModules) > 0 && len(vcsCache) > 0 {
		if choice == fmt.Sprintf("%d", optionNum) {
			return mc.confirmAndRemoveBoth(unusedModules, vcsCache)
		}
		optionNum++
	}

	// Exit option
	if choice == fmt.Sprintf("%d", optionNum) {
		fmt.Println("Exit.")
		return nil
	}

	fmt.Println("Invalid choice, please try again.")
	fmt.Println()
	return nil
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

// showVCSDetails displays VCS cache detailed information
func (mc *ModuleCleaner) showVCSDetails(vcsCache []VCSCacheInfo) {
	if len(vcsCache) == 0 {
		return
	}

	fmt.Println("\nVCS Cache detailed information:")
	fmt.Println(strings.Repeat("-", 80))

	// Sort by size for better display
	sort.Slice(vcsCache, func(i, j int) bool {
		return vcsCache[i].Size > vcsCache[j].Size
	})

	for _, cache := range vcsCache {
		fmt.Printf("🗂️  Hash: %s\n", cache.Hash[:12])
		fmt.Printf("   📁 Repository: %s\n", cache.RepoURL)
		fmt.Printf("   💾 Size: %s\n", FormatSize(cache.Size))
		fmt.Printf("   📅 Last used: %s\n", cache.LastUsed)

		// Show cache path
		vcsDir := filepath.Join(mc.config.GoModCache, "cache", "vcs")
		fmt.Printf("   📁 Cache path: %s/%s\n", vcsDir, cache.Hash[:12])
		fmt.Println()
	}
}

// confirmAndRemoveVCSCache confirms and removes VCS cache
func (mc *ModuleCleaner) confirmAndRemoveVCSCache(vcsCache []VCSCacheInfo) error {
	fmt.Printf("\nConfirm deletion of %d VCS cache entries? (y/N): ", len(vcsCache))

	reader := bufio.NewReader(os.Stdin)
	confirm, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read confirmation: %w", err)
	}

	confirm = strings.ToLower(strings.TrimSpace(confirm))
	if confirm != "y" && confirm != "yes" {
		fmt.Println("VCS cache deletion cancelled.")
		return nil
	}

	return mc.RemoveVCSCache(vcsCache)
}

// confirmAndRemoveBoth confirms and removes both modules and VCS cache
func (mc *ModuleCleaner) confirmAndRemoveBoth(modules []ModuleInfo, vcsCache []VCSCacheInfo) error {
	totalModuleSize := int64(0)
	for _, mod := range modules {
		totalModuleSize += mod.Size
	}

	totalVCSSize := int64(0)
	for _, cache := range vcsCache {
		totalVCSSize += cache.Size
	}

	fmt.Printf("\nConfirm deletion of:\n")
	fmt.Printf("  - %d unused modules (%s)\n", len(modules), FormatSize(totalModuleSize))
	fmt.Printf("  - %d VCS cache entries (%s)\n", len(vcsCache), FormatSize(totalVCSSize))
	fmt.Printf("  - Total space to free: %s\n", FormatSize(totalModuleSize+totalVCSSize))
	fmt.Printf("\nProceed? (y/N): ")

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

	// Remove modules first, then VCS cache
	if len(modules) > 0 {
		if err := mc.RemoveUnusedModules(modules); err != nil {
			return fmt.Errorf("failed to remove modules: %w", err)
		}
	}

	if len(vcsCache) > 0 {
		if err := mc.RemoveVCSCache(vcsCache); err != nil {
			return fmt.Errorf("failed to remove VCS cache: %w", err)
		}
	}

	return nil
}

// continueMenuError is a special error type to indicate menu should continue
type continueMenuError struct{}

func (e continueMenuError) Error() string {
	return "continue menu"
}
