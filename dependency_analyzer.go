package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/modfile"
)

// DependencyAnalyzer handles all dependency analysis logic
type DependencyAnalyzer struct {
	config           *Config
	usedModules      map[string]map[string]bool // module path -> versions
	moduleEntries    map[string]*ModuleEntry    // moduleKey -> complete metadata
	mutex            sync.RWMutex
	analyzedProjects map[string]bool
}

// AnalysisResult contains the result of dependency analysis
type AnalysisResult struct {
	TotalModules  int
	TotalVersions int
	DirectCount   int
	IndirectCount int
	MultiVersions int
}

// NewDependencyAnalyzer creates a new dependency analyzer
func NewDependencyAnalyzer(config *Config) *DependencyAnalyzer {
	return &DependencyAnalyzer{
		config:           config,
		usedModules:      make(map[string]map[string]bool),
		moduleEntries:    make(map[string]*ModuleEntry),
		analyzedProjects: make(map[string]bool),
	}
}

// AnalyzeAllProjects analyzes all specified project paths
func (da *DependencyAnalyzer) AnalyzeAllProjects() (*AnalysisResult, error) {
	if da.config.Verbose {
		fmt.Printf("🔍 Analyzing %d module paths...\n", len(da.config.ModulePaths))
	}

	// Use worker pool for concurrent processing
	maxWorkers := da.config.MaxWorkers
	semaphore := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	var errors []error
	var errorMutex sync.Mutex

	if da.config.Verbose {
		fmt.Printf("Using %d concurrent workers for analysis\n", maxWorkers)
	}

	// Process all projects concurrently
	for i, modPath := range da.config.ModulePaths {
		wg.Add(1)
		go func(index int, path string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if da.config.Verbose {
				fmt.Printf("[%d/%d] Processing: %s\n", index+1, len(da.config.ModulePaths), path)
			}

			if err := da.analyzeProject(path); err != nil {
				errorMutex.Lock()
				errors = append(errors, fmt.Errorf("failed to analyze %s: %w", path, err))
				errorMutex.Unlock()
			}

			if da.config.Verbose {
				fmt.Printf("✅ [%d/%d] Completed: %s\n", index+1, len(da.config.ModulePaths), path)
			}
		}(i, modPath)
	}

	wg.Wait()

	// Calculate and return results
	result := da.calculateAnalysisResult()

	if da.config.Verbose {
		da.printAnalysisStatistics(result)
	}

	return result, nil
}

// analyzeProject analyzes a single project
func (da *DependencyAnalyzer) analyzeProject(projectPath string) error {
	// Expand home directory if needed
	if strings.HasPrefix(projectPath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		projectPath = filepath.Join(homeDir, projectPath[2:])
	}

	// Check if path exists
	info, err := os.Stat(projectPath)
	if err != nil {
		if da.config.Verbose {
			fmt.Printf("Skipping non-existent path: %s\n", projectPath)
		}
		return nil
	}

	if info.IsDir() {
		return da.analyzeDirectory(projectPath)
	} else {
		return da.analyzeGoModFile(projectPath)
	}
}

// analyzeDirectory recursively finds and analyzes go.mod files
func (da *DependencyAnalyzer) analyzeDirectory(dirPath string) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Name() == "go.mod" {
			if da.config.Verbose {
				fmt.Printf("Analyzing go.mod file: %s\n", path)
			}
			return da.analyzeGoModFile(path)
		}

		return nil
	})
}

// analyzeGoModFile analyzes a single go.mod file using all available methods
func (da *DependencyAnalyzer) analyzeGoModFile(goModPath string) error {
	// Parse go.mod file
	if err := da.parseGoModFile(goModPath); err != nil {
		return err
	}

	// Skip additional analysis in fast mode
	if da.config.FastMode {
		if da.config.Verbose {
			fmt.Printf("    Fast mode: skipping indirect dependencies for %s\n", filepath.Dir(goModPath))
		}
		return nil
	}

	projectDir := filepath.Dir(goModPath)

	// Try all analysis methods
	methods := []AnalysisMethod{
		{name: "go.sum", fn: da.analyzeGoSumFile},
		{name: "vendor", fn: da.analyzeVendorDirectory},
		{name: "go list", fn: da.analyzeIndirectDependencies},
	}

	successfulMethods := 0
	for _, method := range methods {
		if err := method.fn(projectDir); err == nil {
			successfulMethods++
			if da.config.Verbose {
				fmt.Printf("    ✅ Successfully analyzed with %s\n", method.name)
			}
		} else if da.config.Verbose {
			fmt.Printf("    ⚠️  %s analysis failed: %v\n", method.name, err)
		}
	}

	if da.config.Verbose {
		fmt.Printf("    📊 Successfully used %d/%d analysis methods\n", successfulMethods, len(methods))
	}

	return nil
}

// AnalysisMethod represents a dependency analysis method
type AnalysisMethod struct {
	name string
	fn   func(string) error
}

// Parse go.mod file for direct dependencies
func (da *DependencyAnalyzer) parseGoModFile(goModPath string) error {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return fmt.Errorf("failed to read go.mod: %w", err)
	}

	modFile, err := modfile.Parse(goModPath, content, nil)
	if err != nil {
		return fmt.Errorf("failed to parse go.mod: %w", err)
	}

	// Add main module
	if modFile.Module != nil {
		da.addUsedModule(modFile.Module.Mod.Path, modFile.Module.Mod.Version)
	}

	// Add direct dependencies
	for _, require := range modFile.Require {
		da.addUsedModule(require.Mod.Path, require.Mod.Version)
	}

	return nil
}

// analyzeGoSumFile parses go.sum for additional dependencies
func (da *DependencyAnalyzer) analyzeGoSumFile(projectDir string) error {
	goSumPath := filepath.Join(projectDir, "go.sum")
	if !PathExists(goSumPath) {
		return fmt.Errorf("go.sum not found")
	}

	content, err := os.ReadFile(goSumPath)
	if err != nil {
		return fmt.Errorf("failed to read go.sum: %w", err)
	}

	moduleCount := 0
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 3 {
			modulePath := parts[0]
			version := strings.TrimSuffix(parts[1], "/go.mod")

			da.addUsedModule(modulePath, version)
			moduleCount++
		}
	}

	if da.config.Verbose {
		fmt.Printf("    Found %d modules from go.sum\n", moduleCount)
	}

	return nil
}

// analyzeVendorDirectory scans vendor directory for dependencies
func (da *DependencyAnalyzer) analyzeVendorDirectory(projectDir string) error {
	vendorPath := filepath.Join(projectDir, "vendor")
	if !PathExists(vendorPath) {
		return fmt.Errorf("vendor directory not found")
	}

	moduleCount := 0
	err := filepath.Walk(vendorPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(vendorPath, path)
		if err != nil || relPath == "." || strings.Contains(relPath, "@") {
			return nil
		}

		// Check if this looks like a module path (e.g., github.com/user/repo)
		if strings.Count(relPath, "/") >= 2 {
			parts := strings.Split(relPath, "/")
			if len(parts) >= 3 {
				modulePath := strings.Join(parts[:3], "/")
				da.addUsedModule(modulePath, "")
				moduleCount++
				return filepath.SkipDir
			}
		}

		return nil
	})

	if da.config.Verbose {
		fmt.Printf("    Found %d modules from vendor\n", moduleCount)
	}

	return err
}

// analyzeIndirectDependencies uses go list to get complete dependency graph
func (da *DependencyAnalyzer) analyzeIndirectDependencies(projectDir string) error {
	// Check if already analyzed
	absPath, _ := filepath.Abs(projectDir)
	da.mutex.Lock()
	if da.analyzedProjects[absPath] {
		da.mutex.Unlock()
		return nil
	}
	da.analyzedProjects[absPath] = true
	da.mutex.Unlock()

	// Execute go list command
	timeout := time.Duration(da.config.Timeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-json", "all")
	cmd.Dir = projectDir
	cmd.Env = os.Environ()

	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("go list timeout (%v)", timeout)
		}
		return fmt.Errorf("go list failed: %w", err)
	}

	// Parse JSON output
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	moduleCount := 0

	for decoder.More() {
		var mod ModuleEntry
		if err := decoder.Decode(&mod); err != nil {
			continue
		}

		if mod.Path != "" {
			version := mod.Version
			if version == "" {
				version = "unknown"
			}

			da.addUsedModule(mod.Path, version)
			da.storeModuleEntry(mod.Path, version, &mod)
			moduleCount++
		}
	}

	if da.config.Verbose {
		fmt.Printf("    Found %d modules via go list\n", moduleCount)
	}

	return nil
}

// addUsedModule adds a module to the used modules map
func (da *DependencyAnalyzer) addUsedModule(modPath, version string) {
	da.mutex.Lock()
	defer da.mutex.Unlock()

	if _, exists := da.usedModules[modPath]; !exists {
		da.usedModules[modPath] = make(map[string]bool)
	}
	da.usedModules[modPath][version] = true
}

// storeModuleEntry stores complete module metadata
func (da *DependencyAnalyzer) storeModuleEntry(modPath, version string, mod *ModuleEntry) {
	moduleKey := fmt.Sprintf("%s@%s", modPath, version)

	da.mutex.Lock()
	da.moduleEntries[moduleKey] = &ModuleEntry{
		Path:      mod.Path,
		Version:   version,
		Time:      mod.Time,
		Indirect:  mod.Indirect,
		Dir:       mod.Dir,
		GoMod:     mod.GoMod,
		GoVersion: mod.GoVersion,
		Sum:       mod.Sum,
		GoModSum:  mod.GoModSum,
	}
	da.mutex.Unlock()
}

// calculateAnalysisResult calculates statistics from the analysis
func (da *DependencyAnalyzer) calculateAnalysisResult() *AnalysisResult {
	da.mutex.RLock()
	defer da.mutex.RUnlock()

	result := &AnalysisResult{}
	result.TotalModules = len(da.usedModules)

	// Count versions and multi-version modules
	for _, versions := range da.usedModules {
		result.TotalVersions += len(versions)
		if len(versions) > 1 {
			result.MultiVersions++
		}
	}

	// Count direct vs indirect dependencies
	for _, entry := range da.moduleEntries {
		if entry.Indirect {
			result.IndirectCount++
		} else {
			result.DirectCount++
		}
	}

	return result
}

// printAnalysisStatistics prints detailed analysis statistics
func (da *DependencyAnalyzer) printAnalysisStatistics(result *AnalysisResult) {
	fmt.Printf("🔍 Found %d unique modules with %d total versions\n",
		result.TotalModules, result.TotalVersions)

	if len(da.moduleEntries) > 0 {
		fmt.Printf("📊 Enhanced analysis: %d modules with complete metadata\n", len(da.moduleEntries))
		fmt.Printf("  📦 Direct dependencies: %d\n", result.DirectCount)
		fmt.Printf("  🔗 Indirect dependencies: %d\n", result.IndirectCount)
	}

	if result.MultiVersions > 0 {
		fmt.Printf("💡 Smart version cleaning: Found %d modules with multiple versions\n", result.MultiVersions)
		fmt.Printf("   Will keep only the latest required version for each module\n")
	}
}

// GetUsedModules returns the map of used modules (thread-safe)
func (da *DependencyAnalyzer) GetUsedModules() map[string]map[string]bool {
	da.mutex.RLock()
	defer da.mutex.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string]map[string]bool)
	for modPath, versions := range da.usedModules {
		result[modPath] = make(map[string]bool)
		for version, used := range versions {
			result[modPath][version] = used
		}
	}
	return result
}

// GetModuleEntries returns the map of module entries (thread-safe)
func (da *DependencyAnalyzer) GetModuleEntries() map[string]*ModuleEntry {
	da.mutex.RLock()
	defer da.mutex.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string]*ModuleEntry)
	for key, entry := range da.moduleEntries {
		result[key] = &ModuleEntry{
			Path:      entry.Path,
			Version:   entry.Version,
			Time:      entry.Time,
			Indirect:  entry.Indirect,
			Dir:       entry.Dir,
			GoMod:     entry.GoMod,
			GoVersion: entry.GoVersion,
			Sum:       entry.Sum,
			GoModSum:  entry.GoModSum,
		}
	}
	return result
}
