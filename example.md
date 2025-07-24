# goclean Usage Examples

## Complete Workflow Demonstration

### 1. Install the Tool

```bash
# Install using go install
go install github.com/foabo/goclean@latest

# Verify installation
goclean -version
```

### 2. Basic Usage

```bash
# Smart scan (automatically discovers projects in ~/go and $GOPATH/src)
goclean

# Dry run to see what modules would be deleted
goclean -dry-run -verbose

# Enterprise environment (recommended)
goclean -fast -workers 12 -verbose
```

### 3. Advanced Usage

```bash
# Scan specific projects
goclean -modules ~/go/src/myproject -verbose

# Scan multiple directories
goclean -modules ~/project1,~/project2,/opt/project3 -dry-run

# Performance tuning for powerful systems
goclean -workers 16 -timeout 10 -verbose

# Enterprise environment with strict network
goclean -fast -timeout 2 -workers 8 -verbose
```

### 4. Expected Output Examples

#### Smart Scan Output
```
🔎 No specific module paths provided, discovering projects automatically...
✅ Found 3 projects. Analyzing their dependencies...
   Projects to be scanned:
   - /Users/user/go/src/github.com/mycompany/project1
   - /Users/user/go/src/github.com/mycompany/project2
   - /Users/user/go/pkg/mod/cache/download

🔧 Configuration:
  - Module cache directory: /Users/user/go/pkg/mod
  - Scan paths: [/Users/user/go/src/github.com/mycompany/project1 ...]
  - Verbose mode: true
  - Dry run: false
  - Fast mode: false
  - Max workers: 16
  - Timeout: 60s

🚀 Starting Go module cache cleaning...
🔍 Analyzing 3 module paths...
Using 8 concurrent workers for analysis
[1/3] Processing: /Users/user/go/src/github.com/mycompany/project1
    ✅ Successfully analyzed with go.mod
    ✅ Successfully analyzed with go.sum
    ✅ Successfully analyzed with go list
[2/3] Processing: /Users/user/go/src/github.com/mycompany/project2
    ✅ Successfully analyzed with go.mod
    ⚠️  go.sum analysis failed: go.sum not found
    ✅ Successfully analyzed with go list
[3/3] Processing: /Users/user/go/pkg/mod/cache/download
...

🔍 Found 45 unique modules with 67 total versions
📊 Enhanced analysis: 52 modules with complete metadata
  📦 Direct dependencies: 23
  🔗 Indirect dependencies: 29
💡 Smart version cleaning: Found 8 modules with multiple versions
   Will keep only the latest required version for each module
```

#### Interactive Menu Output
```
Cleanup Summary:
  📦 Found 23 unused modules, occupying 245.7 MB disk space
  🗂️  Found 12 VCS cache entries, occupying 156.3 MB disk space
  🧹 Total space that can be freed: 402.0 MB

You can:
(1) View details
(2) Delete unused modules only (245.7 MB)
(3) Delete VCS cache only (156.3 MB)
(4) Delete both modules and VCS cache (402.0 MB)
(5) Exit

Please enter the number in parentheses: 1
```

#### Detailed View Output
```
Unused modules detailed information:
--------------------------------------------------------------------------------
📦 Module: github.com/gin-gonic/gin (Total: 28.4 MB)
   📁 Cache: $GOMODCACHE/github.com/gin-gonic/gin
   ├─ v1.8.0 (14.2 MB, removable)
   ├─ v1.8.1 (14.2 MB, removable)

📦 Module: golang.org/x/crypto (Total: 15.6 MB)
   📁 Cache: $GOMODCACHE/golang.org/x/crypto
   ├─ v0.10.0 (7.8 MB, removable)
   ├─ v0.11.0 (7.8 MB, removable)
   
VCS Cache detailed information:
--------------------------------------------------------------------------------
🗂️  Hash: a1b2c3d4e5f6
   📁 Repository: https://github.com/gin-gonic/gin.git
   💾 Size: 45.2 MB
   📅 Last used: 2024-01-15
   📁 Cache path: $GOMODCACHE/cache/vcs/a1b2c3d4e5f6
```

#### Dry Run Output
```
Dry run mode: The following modules would be deleted
  - github.com/gin-gonic/gin@v1.8.0 (14.2 MB)
  - github.com/gin-gonic/gin@v1.8.1 (14.2 MB)
  - golang.org/x/crypto@v0.10.0 (7.8 MB)
  - golang.org/x/crypto@v0.11.0 (7.8 MB)
  ...

📊 Calculating current cache statistics...

🎯 Dry Run Summary:
  📦 Current cache size: 2.1 GB
  🧹 Would free space: 245.7 MB
  📊 Percentage of cache: 11.7%
```

#### Actual Deletion Output
```
Confirm deletion of these modules? (y/N): y
📊 Calculating cache statistics...
Cache size before deletion: 2.1 GB
Starting to delete 23 unused modules...
Deleted: github.com/gin-gonic/gin@v1.8.0 (14.2 MB)
Deleted: github.com/gin-gonic/gin@v1.8.1 (14.2 MB)
Deleted: golang.org/x/crypto@v0.10.0 (7.8 MB)
...

🎯 Deletion Summary:
  ✅ Modules deleted: 23/23
  ⏱️  Time taken: 3.2s
  📦 Cache size before: 2.1 GB
  📦 Cache size after: 1.9 GB
  🧹 Space freed: 245.7 MB (11.7%)
```

## Advanced Scenarios

### 1. Enterprise Environment

```bash
# For environments with restricted network access
goclean -fast -workers 12 -timeout 2 -verbose

# Expected output:
🚀 Starting Go module cache cleaning...
🔍 Analyzing 5 module paths...
[1/5] Processing: /enterprise/project1
    ✅ Successfully analyzed with go.mod
    ✅ Successfully analyzed with go.sum
    Fast mode: skipping indirect dependencies for /enterprise/project1
[2/5] Processing: /enterprise/project2
    ✅ Successfully analyzed with go.mod
    ⚠️  go.sum analysis failed: go.sum not found
    Fast mode: skipping indirect dependencies for /enterprise/project2
...
```

### 2. Performance Optimization

```bash
# For powerful development machines
goclean -workers 16 -verbose

# Expected output shows optimized worker usage:
Using 16 concurrent workers for analysis
    Using 10 workers for size calculations
    Using 8 workers for download size calculations
```

### 3. Version-Aware Cleaning

When multiple versions exist, goclean uses intelligent strategies:

```bash
# Example: gin module with multiple versions
📦 Module: github.com/gin-gonic/gin (Total: 42.6 MB)
   📁 Cache: $GOMODCACHE/github.com/gin-gonic/gin
   ├─ v1.8.0 (14.2 MB, removable)    # Old version - will be cleaned
   ├─ v1.9.1 (14.2 MB, kept)         # Latest required - kept
   ├─ v1.10.0 (14.2 MB, kept)        # Newer version - kept for compatibility

# Direct dependency: Conservative cleaning (keeps v1.9.1 and v1.10.0)
# Indirect dependency: Aggressive cleaning (keeps only v1.9.1)
```

## Frequently Asked Questions

### Q: Why are no unused modules found?
A: Possible reasons:
- Your projects use most modules in the cache
- Incorrect scan paths specified
- Module cache directory is empty
- All cached modules are recent and required

### Q: How does the tool determine which versions to keep?
A: goclean uses semantic versioning with different strategies:
- **Direct dependencies**: Conservative - keeps latest required + newer versions
- **Indirect dependencies**: Aggressive - keeps only the exact latest required version
- **Version comparison**: Uses semantic versioning rules (major.minor.patch)

### Q: What happens if I delete a module I actually need?
A: No problem! Go will automatically re-download any needed modules when you build your project.

### Q: What's the difference between extracted and download cache?
A: 
- **Extracted cache**: Unpacked source code in `$GOMODCACHE/path@version/`
- **Download cache**: Compressed files (.zip, .mod, .info) in `$GOMODCACHE/cache/download/`
- goclean cleans both types for maximum space recovery

### Q: Why does goclean need network access?
A: The `go list -m -json all` command queries module proxies and checksum databases to build the complete dependency graph. Use `-fast` mode to avoid network requirements.

### Q: How can I improve performance?
A: 
- Use `-workers N` to increase concurrency (default: 16)
- Use `-fast` mode to skip network-dependent analysis
- Use `-timeout N` to control go list timeout (default: 60s)

### Q: Is it safe to run in enterprise environments?
A: Yes! Use these recommended settings:
```bash
goclean -fast -workers 12 -timeout 2 -verbose
```

### Q: What is VCS cache and should I clean it?
A: VCS cache stores git repository information in `$GOMODCACHE/cache/vcs/`. It's safe to clean as Go will re-clone repositories when needed.

## Troubleshooting

### Network Timeouts
```bash
# Problem: go list timeout in enterprise
⚠️  Warning: go list timeout (60s) for /project - skipping indirect dependencies

# Solution: Use fast mode
goclean -fast -verbose
```

### Permission Denied
```bash
# Problem: Permission denied when deleting
Failed to delete: github.com/gin-gonic/gin@v1.8.0 - permission denied

# Solution: Make sure files are writable (goclean handles this automatically)
# Or run with appropriate permissions
```

### No Projects Found
```bash
# Problem: No Go projects found in standard locations
⚠️ No Go projects found in standard locations (~/go, $GOPATH/src).

# Solution: Specify project paths manually
goclean -modules /path/to/your/projects
```

## Performance Benchmarks

Typical performance on different systems:

| System | Projects | Modules | Time | Workers |
|--------|----------|---------|------|---------|
| Laptop (8 cores) | 5 | 150 | 2.3s | 8 |
| Workstation (16 cores) | 20 | 500 | 4.1s | 16 |
| Enterprise (12 cores, -fast) | 50 | 800 | 3.7s | 12 |

## Best Practices

1. **First Run**: Always use `-dry-run` to preview
2. **Enterprise**: Use `-fast` mode to avoid network issues
3. **Performance**: Adjust `-workers` based on your CPU cores
4. **Safety**: Regular backups of important projects
5. **Monitoring**: Use `-verbose` for detailed logging 