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
# Dry run to see what modules would be deleted
goclean -dry-run -verbose

# Actual cleaning (requires confirmation)
goclean -verbose
```

### 3. Specify Scan Paths

```bash
# Scan Go workspace
goclean -modules ~/go -verbose

# Scan multiple directories if needed
goclean -modules ~/go,~/legacy-projects -dry-run
```

### 4. Expected Output Example

#### Dry Run Output
```
🚀 Starting Go module cache cleaning...
📊 Analyzing project dependencies...
🔍 Finding unused modules...
Found 237 unused modules, occupying 455.7 MB disk space.

You can:
(1) View details
(2) Delete these modules (requires administrator privileges)
(3) Exit

Please enter the number in parentheses: 2

Confirm deletion of these modules? (y/N): y
Dry run mode: The following modules would be deleted
  - google.golang.org/protobuf@v1.36.5 (14.8 MB)
  - github.com/golang/protobuf@v1.5.3 (787.1 KB)
  ...

📊 Calculating current cache statistics...

🎯 Dry Run Summary:
  📦 Current cache size: 2.1 GB
  🧹 Would free space: 455.7 MB
  📊 Percentage of cache: 21.7%
```

#### Actual Deletion Output
```
Found 237 unused modules, occupying 455.7 MB disk space.

Please enter the number in parentheses: 2

Confirm deletion of these modules? (y/N): y
📊 Calculating cache statistics...
Cache size before deletion: 2.1 GB
Starting to delete 237 unused modules...
Deleted: google.golang.org/protobuf@v1.36.5 (14.8 MB)
Deleted: github.com/golang/protobuf@v1.5.3 (787.1 KB)
...

🎯 Deletion Summary:
  ✅ Modules deleted: 237/237
  ⏱️  Time taken: 2.3s
  📦 Cache size before: 2.1 GB
  📦 Cache size after: 1.7 GB
  🧹 Space freed: 455.7 MB (21.7%)
```

## Frequently Asked Questions

### Q: Why are no unused modules found?
A: Possible reasons:
- Your projects use most modules in the cache
- Scan paths are incorrect
- Module cache directory is empty

### Q: Will deleting modules affect existing projects?
A: No. The tool only deletes modules not used by any scanned project. If projects need them, Go will re-download automatically.

### Q: Can deleted modules be recovered?
A: Yes. When projects need them, Go will automatically re-download the deleted modules.

### Q: Permission denied error when deleting modules?
A: This happens because Go module cache files are read-only. Solutions:

**Option 1 (Recommended)**: The tool now automatically handles permissions
```bash
goclean -verbose
```

**Option 2**: If you still get permission errors, use sudo
```bash
sudo goclean -verbose
```

**Option 3**: Manual permission fix
```bash
chmod -R +w $(go env GOMODCACHE)
goclean -verbose
```

The tool now automatically attempts to make files writable before deletion, but some systems may still require elevated privileges. 