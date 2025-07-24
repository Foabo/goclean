# goclean - Go Module Cache Intelligent Cleaner

English | [中文](README_ZH.md)

## Description

goclean is an intelligent Go module cache cleaning tool that automatically identifies and removes unused modules from your Go projects. It analyzes go.mod files to determine actual dependencies, scans the module cache directory, and provides an interactive interface to safely clean up unused modules and free disk space.

**Key Features:**
- 🔍 Smart dependency analysis across multiple projects  
- 🧹 Safe cleaning of extracted and downloaded module cache
- 🗂️ VCS (Version Control System) cache cleaning
- 💾 Disk space calculation and recovery
- 🔒 Interactive confirmation to prevent accidental deletion
- ⚡ Concurrent processing for better performance
- 🎯 Intelligent version-aware cleaning strategies

## Installation

### Using go install (Recommended)

```bash
# Install latest version
go install github.com/foabo/goclean@latest

# Ensure $GOPATH/bin is in your PATH
export PATH=$PATH:$(go env GOPATH)/bin

# Use directly after installation
goclean -help
goclean -version
goclean -dry-run
```

**Note**: Ensure your `$GOPATH/bin` directory is in the system PATH environment variable so you can run `goclean` command directly.

### Build and Install

```bash
# Clone the code
git clone https://github.com/foabo/goclean.git
cd goclean

# Build
go build -o goclean

# Move to PATH directory (optional)
sudo mv goclean /usr/local/bin/
```

### Run Directly

```bash
# In project directory
go run .
```

## Usage

### Default Smart Scan

By default, `goclean` intelligently discovers all Go projects located in standard paths (`~/go` and `$GOPATH/src`) and analyzes their dependencies to find truly orphaned caches across your entire system.

Simply run:
```bash
goclean
```

The tool will automatically:
1. **Discover** all `go.mod` files in common workspaces
2. **Analyze** the complete dependency graph of all found projects  
3. **Identify** modules in `$GOMODCACHE` that are not used by any of these projects
4. **Provide** an interactive menu to view details and confirm deletion

### Specify Scan Paths

If you want to limit dependency analysis to specific projects, use the `-modules` flag with a comma-separated list of paths.

```bash
# Scan specific project
goclean -modules /path/to/your/go/project

# Scan multiple projects
goclean -modules ~/project1,~/project2,/opt/project3

# Use dry-run to preview what would be deleted
goclean -modules ~/myproject -dry-run -verbose
```

### Command Line Options

```bash
# Basic usage
goclean                              # Smart scan of all projects
goclean -dry-run                     # Preview mode, no actual deletion
goclean -verbose                     # Enable detailed output
goclean -fast                        # Skip indirect dependency analysis (recommended for enterprise)

# Performance tuning
goclean -workers 16                  # Use 16 concurrent workers (default: 8)
goclean -timeout 10                  # Set 10s timeout for go list (default: 5s)

# Enterprise environment (recommended settings)
goclean -fast -workers 12 -verbose  # Fast, concurrent, with detailed logs
goclean -fast -timeout 2 -verbose   # Very aggressive timeout for restricted networks

# Specific projects
goclean -modules ~/go/src/myproject  # Scan only specific project
goclean -modules /tmp/empty_test     # Find all modules as unused (advanced)
```

### Interactive Menu

After analysis, goclean provides an interactive menu:

```
Cleanup Summary:
  📦 Found 15 unused modules, occupying 156.7 MB disk space
  🗂️  Found 8 VCS cache entries, occupying 89.3 MB disk space  
  🧹 Total space that can be freed: 246.0 MB

You can:
(1) View details
(2) Delete unused modules only (156.7 MB)
(3) Delete VCS cache only (89.3 MB)
(4) Delete both modules and VCS cache (246.0 MB)
(5) Exit
```

## Architecture

### Core Components

1. **DependencyAnalyzer** (`dependency_analyzer.go`)
   - Handles all dependency analysis logic
   - Supports 4 analysis methods: go.mod, go.sum, vendor, go list
   - Provides thread-safe access to analysis results

2. **ModuleCleaner** (`cleaner.go`)  
   - Manages cache scanning and cleaning operations
   - Implements intelligent version-aware cleaning strategies
   - Handles VCS cache detection and removal

3. **Utils** (`utils.go`)
   - Utility functions for file operations and project discovery
   - Automatic Go project detection in standard locations

### Dependency Analysis Methods

goclean uses multiple complementary methods to ensure comprehensive dependency analysis:

1. **go.mod parsing**: Direct dependencies from require statements
2. **go.sum analysis**: Additional version information from checksums  
3. **vendor scanning**: Dependencies from vendor directories
4. **go list -m -json all**: Complete dependency graph (can be skipped with `-fast`)

### Intelligent Cleaning Strategies

The tool implements layered cleaning strategies based on dependency metadata:

- **Direct Dependencies**: Conservative cleaning - keeps latest required version and newer
- **Indirect Dependencies**: Aggressive cleaning - keeps only the exact latest required version
- **Version-aware**: Semantic version comparison to determine which versions to keep
- **Dual Cache**: Cleans both extracted (`$GOMODCACHE/path@version/`) and download (`$GOMODCACHE/cache/download/`) caches

## How It Works

1. **Project Discovery Phase**:
   - Automatically scans `~/go` and `$GOPATH/src` for Go projects
   - Finds all `go.mod` files while skipping `vendor`, `pkg`, `.git` directories

2. **Dependency Analysis Phase**:
   - Parses go.mod files for direct dependencies
   - Analyzes go.sum files for version information
   - Scans vendor directories for additional dependencies
   - Uses `go list -m -json all` for complete dependency graph (unless `-fast` mode)

3. **Cache Scanning Phase**:
   - Scans `$GOMODCACHE` directory for all cached modules
   - Identifies both extracted and downloaded module caches
   - Scans VCS cache in `$GOMODCACHE/cache/vcs/`

4. **Intelligent Cleaning Phase**:
   - Applies version-aware cleaning strategies
   - Differentiates between direct and indirect dependencies
   - Uses semantic version comparison for intelligent decisions

5. **Interactive Confirmation Phase**:
   - Shows discovered unused modules and VCS cache
   - Provides detailed information and deletion confirmation
   - Safely deletes selected cache entries with progress tracking

## Performance Optimization

- **Concurrent Processing**: Configurable worker pools for parallel analysis and size calculations
- **Fast Mode**: Skip network-dependent `go list` for enterprise environments  
- **Intelligent Caching**: Avoid re-analyzing the same projects
- **Optimized I/O**: Batch operations and efficient file system access

## Enterprise Environment Support

For enterprise environments with network restrictions:

```bash
# Recommended settings for enterprise
goclean -fast -workers 12 -timeout 2 -verbose

# Environment variables
export GOCLEAN_TIMEOUT="2s"           # Override default timeout
export GOCLEAN_OVERRIDE_PROXY="true"  # Force proxy override (if needed)
```

### Go Proxy Configuration for Enterprise

Before using goclean in enterprise environments, ensure your Go proxy is properly configured:

```bash
# Essential Go environment variables for enterprise
export GOPROXY="https://your-company-proxy.com,direct"     # Company proxy
export GOPRIVATE="*.internal.com,gitlab.company.com"       # Private modules
export GOSUMDB="off"                                        # Disable checksum database
export GONOPROXY="*.internal.com"                          # Skip proxy for internal
export GONOSUMDB="*.internal.com"                          # Skip checksum for internal
export GOINSECURE="*.internal.com"                         # Allow insecure for internal

# Test your configuration
cd /path/to/your/project
go list -m -json all
```

**If `go list` fails or times out:**
- Use `-fast` mode to skip network-dependent analysis
- Verify your GOPROXY and GOPRIVATE settings
- Check corporate firewall/VPN connectivity
- Consider increasing `-timeout` or use `-timeout 2` for very restrictive networks

The `-fast` mode avoids network timeouts by skipping `go list -m -json all` and relying on local file analysis only.

## Technical Implementation

- **Language**: Go 1.18+
- **Dependencies**: 
  - `golang.org/x/mod` - Module path parsing and encoding
- **Concurrency**: Goroutines with configurable worker pools and semaphores
- **Error Handling**: Comprehensive error handling with user-friendly messages
- **Cache Types**: Supports both extracted and download cache formats
- **Version Management**: Semantic versioning with intelligent comparison logic

## Examples

See [example.md](example.md) for complete usage examples and FAQ.

## Understanding Go Module Cache

For detailed explanation of Go module cache structure:
- [Go Module Cache Types (English)](go-module-cache-types.md) - Detailed explanation of extracted vs download cache types
- [Go Module Cache Types (中文)](go-module-cache-types_ZH.md) - 提取式和下载式缓存类型的详细说明

## Release Notes

### How to Release New Version

1. Ensure all changes are committed to main branch
2. Create version tag:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```
3. Users can install specific version:
   ```bash
   go install github.com/foabo/goclean@v1.1.0
   ```

### Version History

- **v1.1.0**: Enhanced path resolution support
  - Support for current directory (`.`) and relative paths (`./project`)
  - Environment variable expansion (`$GOPATH`, `$GOMODCACHE`, `$HOME`)
  - Improved error handling and user feedback
  - Enhanced enterprise environment configuration detection
  - Fixed async progress display with atomic counters
  - Better path validation and expansion

- **v1.0.0**: Initial release with intelligent dependency analysis, cache scanning and cleaning, interactive interface, and concurrent performance optimization

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## FAQ

**Q: Why does goclean need network access?**  
A: The `go list -m -json all` command queries module proxies and checksum databases to build the complete dependency graph. Use `-fast` mode to avoid network requirements.

**Q: Is it safe to delete modules?**  
A: Yes, Go will re-download any needed modules when you build your projects. goclean only removes modules that are not referenced by any discovered Go projects.

**Q: What's the difference between extracted and download cache?**  
A: Extracted cache contains unpacked module source code, while download cache contains .zip, .mod, and .info files. goclean cleans both types.

**Q: How does version-aware cleaning work?**  
A: goclean analyzes which versions are actually used and keeps only the necessary versions based on semantic versioning rules, with different strategies for direct vs indirect dependencies.
