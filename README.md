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

### Default Smart Scan

By default, `goclean` intelligently discovers all Go projects located in standard paths (`~/go` and `$GOPATH/src`) and analyzes their dependencies to find truly orphaned caches across your entire system.

Simply run:
```bash
goclean
```

The tool will automatically:
1.  **Discover** all `go.mod` files in common workspaces.
2.  **Analyze** the complete dependency graph of all found projects.
3.  **Identify** modules in `$GOMODCACHE` that are not used by any of these projects.
4.  **Present** an interactive menu to view details and confirm deletion.

### Specify Scan Paths

If you want to limit the dependency analysis to specific projects, use the `-modules` flag with a comma-separated list of paths.

```bash
# Analyze only a single project
goclean -modules /path/to/your/project

# Analyze multiple specific projects
goclean -modules /path/to/projectA,/path/to/projectB
```

### Dry Run

To see which modules would be deleted without actually removing any files, use the `-dry-run` flag. This is highly recommended for the first run.

```bash
goclean -dry-run
```

### Verbose Mode

For more detailed output during the scanning and analysis process, use the `-verbose` flag.

```bash
goclean -verbose
```

### Help and Version

To see all available options or check the current version, use the `-help` or `-version` flags.

```bash
goclean -help
goclean -version
```

## Example
For detailed examples of the output, please see [example.md](example.md).

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
