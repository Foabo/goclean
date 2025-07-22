# goclean - Go Module Cache Intelligent Cleaner

English | [中文](README_ZH.md)

## Description

goclean is an intelligent Go module cache cleaning tool that automatically identifies and removes unused modules from your Go projects. It analyzes go.mod files to determine actual dependencies, scans the module cache directory, and provides an interactive interface to safely clean up unused modules and free disk space.

**Key Features:**
- 🔍 Smart dependency analysis across multiple projects
- 🧹 Safe cleaning of extracted and downloaded module cache
- 💾 Disk space calculation and recovery
- 🔒 Interactive confirmation to prevent accidental deletion
- ⚡ Concurrent processing for better performance

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

### Basic Usage

```bash
# Use default settings
goclean

# Enable verbose output mode
goclean -verbose

# Dry run (don't actually delete files)
goclean -dry-run

# Show help information
goclean -help

# Show version information
goclean -version
```

### Specify Scan Paths

```bash
# Specify Go workspace directory
goclean -modules ~/go

# Specify multiple directories if needed (comma-separated)
goclean -modules ~/go,~/legacy-projects

# Specify specific go.mod files
goclean -modules ~/go/myproject/go.mod,~/go/anotherproject/go.mod
```

### Interactive Operation

When the tool finds unused modules, it will display an interactive interface similar to the following:

```
Found 147 unused modules, occupying 435.2 MB disk space.

You can:
(1) View details
(2) Delete these modules (requires administrator privileges)
(3) Exit

Please enter the number in parentheses:
```

Option selection:
- **1**: Display detailed information for all unused modules (module path, version, size, type)
- **2**: Delete all unused modules after confirmation
- **3**: Exit the program

## Command Line Options

| Option | Description |
|--------|-------------|
| `-modules string` | Module paths to scan, comma-separated |
| `-verbose` | Enable verbose output mode |
| `-dry-run` | Only simulate run, don't actually delete files |
| `-help` | Show help information |
| `-version` | Show version information |

## Important Notes

1. **Permission Requirements**: Deleting modules may require administrator privileges, especially with system-level Go installations
2. **Safety Prevention**: Recommend using `-dry-run` parameter first to preview content to be deleted
3. **Default Path**: If no scan path is specified, the tool uses `$GOPATH/src` as the default scan directory
4. **Environment Variables**: The tool depends on `GOMODCACHE` environment variable to locate module cache directory
5. **Backup Recommendation**: Recommend backing up important projects before large-scale cleaning

## How It Works

1. **Dependency Analysis Phase**:
   - Scans all go.mod files in specified paths
   - Parses direct dependencies in each go.mod file
   - Uses `go list -m -json all` to get complete dependency graph

2. **Cache Scanning Phase**:
   - Scans all modules in `$GOMODCACHE` directory
   - Identifies extracted module directories (format: `path@version`)
   - Identifies downloaded module files (`.mod`, `.zip`, `.info` files)

3. **Comparison Analysis Phase**:
   - Compares modules in cache with modules used in projects
   - Identifies modules not used by any project
   - Calculates disk space occupied by each unused module

4. **Interactive Cleaning Phase**:
   - Shows discovered unused modules to user
   - Provides detailed information viewing and deletion confirmation options
   - Safely deletes selected module files and directories

## Technical Implementation

- **Language**: Go 1.18+
- **Dependencies**: 
  - `golang.org/x/mod` - Module path parsing and encoding
- **Concurrency**: Uses goroutines and semaphores to control concurrent computation
- **Error Handling**: Comprehensive error handling and user-friendly error messages

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
   go install github.com/foabo/goclean@v1.0.0
   ```

### Version History

- **v1.0.0**: Initial release
  - Intelligent dependency analysis
  - Cache scanning and cleaning
  - Interactive interface
  - Concurrent performance optimization

## Examples

See [example.md](example.md) for complete usage examples and FAQ.

## Understanding Go Module Cache Types

For detailed explanation of Go module cache structure:
- [Go Module Cache Types (English)](go-module-cache-types.md) - Detailed explanation of extracted vs download cache types
- [Go 模块缓存类型 (中文)](go-module-cache-types_ZH.md) - 中文版本的详细说明

## References

You can refer to the implementation of [go-mod-clean](https://github.com/fosmjo/go-mod-clean/blob/main/cleaner.go).

## License

MIT License
