package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	// Define command line parameters
	var (
		modulePaths = flag.String("modules", "", "Module paths to scan, comma-separated")
		verbose     = flag.Bool("verbose", false, "Enable verbose output mode")
		dryRun      = flag.Bool("dry-run", false, "Only simulate run, don't actually delete files")
		fastMode    = flag.Bool("fast", false, "Fast mode: skip indirect dependencies analysis")
		maxWorkers  = flag.Int("workers", 16, "Maximum number of concurrent workers (default: 16)")
		timeout     = flag.Int("timeout", 60, "Timeout for go list commands in seconds (default: 60)")
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
		expandedPaths, err := ParseModulePaths(*modulePaths)
		if err != nil {
			fmt.Printf("❌ Error parsing module paths: %v\n", err)
			os.Exit(1)
		}
		paths = expandedPaths

		if *verbose && len(paths) > 0 {
			fmt.Printf("📂 Expanded module paths:\n")
			for i, path := range paths {
				fmt.Printf("   [%d] %s\n", i+1, path)
			}
			fmt.Println()
		}
	}

	// Create configuration
	config, err := NewConfig(paths, *verbose, *dryRun, *fastMode, *maxWorkers, *timeout)
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
		fmt.Printf("  - Timeout: %ds\n", config.Timeout)
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
  -timeout int       Timeout for go list commands in seconds (default: 5)
  -help              Show this help information
  -version           Show version information

Examples:
  # Use default settings (smart project discovery)
  goclean

  # High-performance system (16+ cores)
  goclean -workers 16 -verbose

  # Resource-constrained system
  goclean -workers 4 -fast

  # Enterprise environment (recommended)
  goclean -fast -workers 12 -verbose

  # Custom timeout for slow networks
  goclean -timeout 10 -verbose

  # Very aggressive timeout for enterprise
  goclean -timeout 2 -fast -verbose

  # Dry run to preview (recommended first run)
  goclean -dry-run -verbose

  # Specify module paths (supports various formats):
  goclean -modules .                                    # Current directory
  goclean -modules ./myproject                          # Relative path
  goclean -modules ~/go/src/myproject                   # Home directory
  goclean -modules $GOPATH/src/company.com/project      # Environment variables
  goclean -modules /absolute/path/to/project            # Absolute path
  goclean -modules ".,~/other-project,$GOPATH/src/old"  # Multiple paths (comma-separated)

Notes:
  - Deleting modules may require administrator privileges
  - Recommend using -dry-run parameter first to preview content
  - Enterprise environments: Use -fast mode to avoid network timeouts
  - Use -timeout 2 for very restrictive networks
  - Use -timeout 10 for slow but working networks
  - Default behavior: auto-discover projects in ~/go and $GOPATH/src
`)
}
