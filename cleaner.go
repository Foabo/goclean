package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

// ModuleCleaner Go module cache cleaner
type ModuleCleaner struct {
	config      *Config
	usedModules map[string]bool // Set of modules in use
	mutex       sync.RWMutex    // Protect concurrent access
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
		config:      config,
		usedModules: make(map[string]bool),
	}
}

// AnalyzeDependencies analyzes project dependencies
func (mc *ModuleCleaner) AnalyzeDependencies() error {
	if mc.config.Verbose {
		fmt.Println("Starting to analyze project dependencies...")
	}

	for _, modPath := range mc.config.ModulePaths {
		if err := mc.analyzeModulePath(modPath); err != nil {
			return fmt.Errorf("failed to analyze module path %s: %w", modPath, err)
		}
	}

	if mc.config.Verbose {
		fmt.Printf("Found %d modules in use\n", len(mc.usedModules))
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
		if !require.Indirect {
			mc.addUsedModule(require.Mod.Path)
		}
	}

	// Get complete dependency graph (including indirect dependencies)
	return mc.analyzeIndirectDependencies(filepath.Dir(goModPath))
}

// analyzeIndirectDependencies uses go list to get indirect dependencies
func (mc *ModuleCleaner) analyzeIndirectDependencies(projectDir string) error {
	cmd := exec.Command("go", "list", "-m", "-json", "all")
	cmd.Dir = projectDir

	output, err := cmd.Output()
	if err != nil {
		if mc.config.Verbose {
			fmt.Printf("Failed to get indirect dependencies (may not be a valid Go module): %s\n", projectDir)
		}
		return nil // Not a fatal error, continue processing other projects
	}

	decoder := json.NewDecoder(strings.NewReader(string(output)))
	for decoder.More() {
		var mod struct {
			Path string `json:"Path"`
		}
		if err := decoder.Decode(&mod); err != nil {
			continue
		}
		if mod.Path != "" {
			mc.addUsedModule(mod.Path)
		}
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

// FindUnusedModules finds unused modules
func (mc *ModuleCleaner) FindUnusedModules() ([]ModuleInfo, error) {
	if mc.config.Verbose {
		fmt.Println("Scanning module cache directory...")
	}

	var unusedModules []ModuleInfo

	// Scan extracted modules
	extractedModules, err := mc.findUnusedExtractedModules()
	if err != nil {
		return nil, fmt.Errorf("failed to scan extracted modules: %w", err)
	}
	unusedModules = append(unusedModules, extractedModules...)

	// Scan downloaded modules
	downloadedModules, err := mc.findUnusedDownloadedModules()
	if err != nil {
		return nil, fmt.Errorf("failed to scan downloaded modules: %w", err)
	}
	unusedModules = append(unusedModules, downloadedModules...)

	// Sort by module path
	sort.Slice(unusedModules, func(i, j int) bool {
		return unusedModules[i].Path < unusedModules[j].Path
	})

	return unusedModules, nil
}

// findUnusedExtractedModules finds unused extracted modules
func (mc *ModuleCleaner) findUnusedExtractedModules() ([]ModuleInfo, error) {
	var modules []ModuleInfo
	cacheDir := mc.config.GoModCache

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
				size, _ := mc.calculateDirectorySize(path)
				modules = append(modules, ModuleInfo{
					Path:    modPath,
					Version: version,
					Size:    size,
					Type:    "extracted",
				})
				return filepath.SkipDir // Skip subdirectories
			}
		}

		return nil
	})

	return modules, err
}

// findUnusedDownloadedModules finds unused downloaded modules
func (mc *ModuleCleaner) findUnusedDownloadedModules() ([]ModuleInfo, error) {
	var modules []ModuleInfo
	downloadDir := filepath.Join(mc.config.GoModCache, "cache", "download")

	if _, err := os.Stat(downloadDir); os.IsNotExist(err) {
		return modules, nil
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
				// Only add once, avoid duplication (.mod, .zip, .info all correspond to same module)
				exists := false
				for _, mod := range modules {
					if mod.Path == modPath {
						exists = true
						break
					}
				}

				if !exists {
					size := mc.calculateModuleDownloadSize(path, downloadDir, modPath)
					modules = append(modules, ModuleInfo{
						Path:    modPath,
						Version: mc.extractVersionFromPath(path),
						Size:    size,
						Type:    "download",
					})
				}
			}
		}

		return nil
	})

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

	// Remove filename, get module path
	dir := filepath.Dir(relativePath)
	if dir == "." {
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
