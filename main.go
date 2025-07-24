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
		fastMode    = flag.Bool("fast", false, "Fast mode: skip indirect dependencies analysis")
		maxWorkers  = flag.Int("workers", 8, "Maximum number of concurrent workers (default: 8)")
		showHelp    = flag.Bool("help", false, "Show help information")
		showVersion = flag.Bool("version", false, "Show version information")
	)

	flag.Parse()

	if *showHelp {
		showUsage()
		return
	}

	if *showVersion {
		fmt.Println("goclean v1.0.0") // Updated version
		fmt.Println("Go Module Cache Intelligent Cleaner")
		return
	}

	// New smart default behavior
	var paths []string
	if *modulePaths == "" {
		fmt.Println("🔎 No specific module paths provided, discovering projects automatically...")
		discoveredProjects, err := DiscoverGoProjects()
		if err != nil {
			fmt.Printf("❌ Error during project discovery: %v\n", err)
			os.Exit(1)
		}

		if len(discoveredProjects) > 0 {
			fmt.Printf("✅ Found %d projects. Analyzing their dependencies...\n", len(discoveredProjects))
			paths = discoveredProjects
			if *verbose {
				fmt.Println("   Projects to be scanned:")
				for _, p := range discoveredProjects {
					fmt.Printf("   - %s\n", p)
				}
			}
		} else {
			fmt.Println("⚠️ No Go projects found in standard locations (~/go, $GOPATH/src).")
			fmt.Println("   To analyze specific projects, use the -modules flag.")
			fmt.Println("   Proceeding to find all modules not used by any discovered project (which is none).")
			// An empty 'paths' slice will result in finding all modules as unused.
		}
		fmt.Println() // Add a newline for better readability
	} else {
		rawPaths := strings.Split(*modulePaths, ",")
		for _, path := range rawPaths {
			paths = append(paths, strings.TrimSpace(path))
		}
	}

	// Create configuration
	config, err := NewConfig(paths, *verbose, *dryRun, *fastMode, *maxWorkers)
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
		fmt.Printf("  - Fast mode: %t\n", config.FastMode)
		fmt.Printf("  - Max workers: %d\n", config.MaxWorkers)
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

	// Analyze dependencies
	if err := cleaner.AnalyzeDependencies(); err != nil {
		return err
	}

	// Find unused modules
	unusedModules, err := cleaner.FindUnusedModules()
	if err != nil {
		return err
	}

	// Show interactive menu for cleaning
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
  -fast              Fast mode: skip indirect dependencies analysis
  -workers int       Maximum number of concurrent workers (default: 8)
  -help              Show this help information
  -version           Show version information

Examples:
  # Use default settings (8 workers)
  goclean

  # High-performance system (16+ cores)
  goclean -workers 16 -verbose

  # Resource-constrained system
  goclean -workers 4 -fast

  # Enterprise environment with many projects
  goclean -workers 12 -fast -verbose

  # Dry run to preview (recommended first run)
  goclean -dry-run -verbose

Notes:
  - Deleting modules may require administrator privileges
  - Recommend using -dry-run parameter first to preview content
  - Default scan directory is $GOPATH/src
`)
}
