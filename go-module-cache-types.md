# Go Module Cache Types: Extracted vs Download

## Overview

Go's module cache uses a two-layer storage system to optimize both network efficiency and compilation performance. Understanding these two types helps you make informed decisions when cleaning module cache.

## 📦 Two-Layer Cache Structure

### 🔽 Download Type (Download Cache)
- **Location**: `$GOMODCACHE/cache/download/module-path/`
- **Contents**: 
  - `.mod` files - Module's go.mod content
  - `.zip` files - Compressed source code packages
  - `.info` files - Module metadata information
- **Purpose**: Store original files downloaded from proxy servers
- **Characteristics**: Compressed storage, space-efficient

**Example Structure**:
```
$GOMODCACHE/cache/download/github.com/gin-gonic/gin/@v/
├── v1.9.1.mod     # go.mod file content
├── v1.9.1.zip     # source code archive
├── v1.9.1.info    # module metadata
└── v1.9.1.ziphash # checksum verification
```

### 📂 Extracted Type (Extracted Cache)
- **Location**: `$GOMODCACHE/module-path@version/`
- **Contents**: Complete source code files and directory structure
- **Purpose**: Source code directly used by Go compiler
- **Characteristics**: Uncompressed, ready-to-compile directory structure

**Example Structure**:
```
$GOMODCACHE/github.com/gin-gonic/gin@v1.9.1/
├── binding/
├── render/
├── gin.go
├── context.go
├── go.mod
└── ... all source code files
```

## 🔄 Workflow

```
1. go get/build → Download .zip/.mod/.info → Download Type
                        ↓
2. First compile → Extract .zip to directory → Extracted Type
                        ↓  
3. Compiler usage → Direct access to source files
```

## 📊 Size Comparison Example

**Real example with yaml.v3@v3.0.1**:

| Type | Size | Storage Method | Usage |
|------|------|----------------|-------|
| **extracted** | **504K** | Complete source directory | Direct compiler access |
| **download** | **124K** | Compressed files + metadata | Download cache |

### **Space Usage Analysis**:
- **Extracted** uses ~4x more space due to uncompressed files
- **Download** is space-efficient with compression
- **Total** cache stores essentially 2 copies of each module

## 🧹 Cleaning Safety

### **Why Both Types Can Be Safely Deleted?**

1. **Download files deleted**:
   - Go will re-download from proxy servers
   - Regenerate `.mod`, `.zip`, `.info` files

2. **Extracted directories deleted**:
   - Go will re-extract existing `.zip` files
   - Or re-download and extract if needed

### **Space Savings Strategy**:
- **Delete extracted only**: Save ~75% space (keep download cache)
- **Delete both**: Save 100% space (full re-download needed)
- **Smart cleaning**: Tools like goclean remove both types for unused modules

## 🔍 File Permissions

### **Extracted Type**:
- **Read-only permissions** on all files
- Prevents accidental modification of cached modules
- Example: `-r--r--r--` permissions

### **Download Type**:
- **Standard file permissions**
- Used for caching and verification
- Example: `-rw-r--r--` permissions

## 💡 Best Practices

1. **Regular Cleaning**: Use tools like `goclean` to remove unused modules
2. **Monitor Usage**: Check `$GOMODCACHE` size periodically
3. **Network Awareness**: Consider network costs when cleaning download cache
4. **Build Performance**: Extracted cache speeds up compilation

## 🛠️ Manual Inspection

You can manually inspect your cache:

```bash
# Check extracted modules
find $(go env GOMODCACHE) -maxdepth 2 -name "*@v*" -type d

# Check download cache
ls $(go env GOMODCACHE)/cache/download/

# Compare sizes
du -sh $(go env GOMODCACHE)/module-path@version
du -sh $(go env GOMODCACHE)/cache/download/module-path
```

## ⚡ Performance Impact

- **Download cache**: Reduces network traffic, faster `go get`
- **Extracted cache**: Reduces compilation time, no extraction overhead
- **Missing both**: Requires full download + extraction cycle

Understanding these two cache types helps you make informed decisions about module cache management and cleanup strategies. 