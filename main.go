package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// Define command line parameters
	var (
		modulePaths = flag.String("modules", "", "Module paths to scan, comma-separated")
		verbose     = flag.Bool("verbose", false, "Enable verbose output mode")
		dryRun      = flag.Bool("dry-run", false, "Only simulate run, don't actually delete files")
		showHelp    = flag.Bool("help", false, "Show help information")
		showVersion = flag.Bool("version", false, "Show version information")
	)

	flag.Parse()

	if *showHelp {
		showUsage()
		return
	}

	if *showVersion {
		fmt.Println("goclean v1.0.0")
		fmt.Println("Go Module Cache Intelligent Cleaner")
		return
	}

	// Parse module paths
	var paths []string
	if *modulePaths != "" {
		rawPaths := strings.Split(*modulePaths, ",")
		for _, path := range rawPaths {
			paths = append(paths, strings.TrimSpace(path))
		}
	}

	// If no module paths specified, use default path
	if len(paths) == 0 {
		defaultPath, err := getDefaultModulePath()
		if err != nil {
			fmt.Printf("❌ Failed to get default module path: %v\n", err)
			os.Exit(1)
		}
		paths = []string{defaultPath}
		if *verbose {
			fmt.Printf("Using default module path: %s\n", defaultPath)
		}
	}

	// Create configuration
	config, err := NewConfig(paths, *verbose, *dryRun)
	if err != nil {
		fmt.Printf("❌ Failed to create configuration: %v\n", err)
		os.Exit(1)
	}

	if config.Verbose {
		fmt.Printf("🔧 Configuration:\n")
		fmt.Printf("  - Module cache directory: %s\n", config.GoModCache)
		fmt.Printf("  - Scan paths: %v\n", config.ModulePaths)
		fmt.Printf("  - Verbose mode: %t\n", config.Verbose)
		fmt.Printf("  - Dry run: %t\n", config.DryRun)
		fmt.Println()
	}

	// Create cleaner and execute cleaning
	cleaner := NewModuleCleaner(config)

	if err := runCleaner(cleaner); err != nil {
		fmt.Printf("❌ Cleaning process failed: %v\n", err)
		os.Exit(1)
	}
}

// runCleaner executes cleaning process
func runCleaner(cleaner *ModuleCleaner) error {
	fmt.Println("🚀 Starting Go module cache cleaning...")

	// 1. Analyze project dependencies
	fmt.Println("📊 Analyzing project dependencies...")
	if err := cleaner.AnalyzeDependencies(); err != nil {
		return fmt.Errorf("dependency analysis failed: %w", err)
	}

	// 2. Find unused modules
	fmt.Println("🔍 Finding unused modules...")
	unusedModules, err := cleaner.FindUnusedModules()
	if err != nil {
		return fmt.Errorf("finding unused modules failed: %w", err)
	}

	// 3. Show interactive menu
	return cleaner.ShowInteractiveMenu(unusedModules)
}

// getDefaultModulePath gets default module scan path
func getDefaultModulePath() (string, error) {
	// Try using $GOPATH/src
	gopath, err := GetGOPATH()
	if err == nil {
		srcPath := filepath.Join(gopath, "src")
		if PathExists(srcPath) {
			return srcPath, nil
		}
	}

	// If GOPATH/src doesn't exist, use common directories under user home
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Check common project directories
	commonDirs := []string{"go", "code", "projects", "workspace", "dev"}
	for _, dir := range commonDirs {
		path := filepath.Join(homeDir, dir)
		if PathExists(path) {
			return path, nil
		}
	}

	// Finally use user home directory
	return homeDir, nil
}

// showUsage displays usage instructions
func showUsage() {
	fmt.Print(`goclean - Go Module Cache Intelligent Cleaner

This is a Go module cache cleaning tool that can automatically identify
and clean unused modules in Go projects. The tool determines actual
project dependencies by analyzing go.mod files, then scans the module
cache directory to find unused modules.

Usage:
  goclean [options]

Options:
  -modules string    Module paths to scan, comma-separated
  -verbose           Enable verbose output mode
  -dry-run           Only simulate run, don't actually delete files
  -help              Show this help information
  -version           Show version information

Examples:
  # Use default settings
  goclean

  # Specify directory to scan
  goclean -modules ~/go

  # Enable verbose mode
  goclean -verbose

  # Dry run (don't actually delete)
  goclean -dry-run

Notes:
  - Deleting modules may require administrator privileges
  - Recommend using -dry-run parameter first to preview content
  - Default scan directory is $GOPATH/src
`)
}
