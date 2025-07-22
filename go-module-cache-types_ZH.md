# Go 模块缓存类型：Extracted 与 Download 详解

## 概述

Go 的模块缓存使用双层存储系统来优化网络效率和编译性能。理解这两种类型有助于您在清理模块缓存时做出明智的决策。

## 📦 双层缓存结构

### 🔽 Download 类型（下载缓存）
- **位置**: `$GOMODCACHE/cache/download/模块路径/`
- **内容**: 
  - `.mod` 文件 - 模块的 go.mod 内容
  - `.zip` 文件 - 压缩的源代码包
  - `.info` 文件 - 模块元数据信息
- **作用**: 存储从代理服务器下载的原始文件
- **特点**: 压缩存储，节省空间

**示例结构**:
```
$GOMODCACHE/cache/download/github.com/gin-gonic/gin/@v/
├── v1.9.1.mod     # go.mod 文件内容
├── v1.9.1.zip     # 源代码压缩包
├── v1.9.1.info    # 模块元数据
└── v1.9.1.ziphash # 校验和验证
```

### 📂 Extracted 类型（提取缓存）
- **位置**: `$GOMODCACHE/模块路径@版本/`
- **内容**: 完整的源代码文件和目录结构
- **作用**: Go 编译器直接使用的源代码
- **特点**: 未压缩，可直接编译的目录结构

**示例结构**:
```
$GOMODCACHE/github.com/gin-gonic/gin@v1.9.1/
├── binding/
├── render/
├── gin.go
├── context.go
├── go.mod
└── ... 所有源代码文件
```

## 🔄 工作流程

```
1. go get/build → 下载 .zip/.mod/.info → Download 类型
                        ↓
2. 首次编译 → 解压 .zip 到目录 → Extracted 类型
                        ↓  
3. 编译器使用 → 直接访问源代码文件
```

## 📊 大小对比示例

**yaml.v3@v3.0.1 的真实示例**:

| 类型 | 大小 | 存储方式 | 用途 |
|------|------|----------|------|
| **extracted** | **504K** | 完整源代码目录 | 编译器直接访问 |
| **download** | **124K** | 压缩文件+元数据 | 下载缓存 |

### **空间使用分析**:
- **Extracted** 由于未压缩文件使用约 4 倍空间
- **Download** 通过压缩节省空间
- **总计** 缓存实际上存储了每个模块的 2 份副本

## 🧹 清理安全性

### **为什么两种类型都可以安全删除？**

1. **Download 文件被删除**:
   - Go 会重新从代理服务器下载
   - 重新生成 `.mod`、`.zip`、`.info` 文件

2. **Extracted 目录被删除**:
   - Go 会重新解压现有的 `.zip` 文件
   - 或在需要时重新下载并解压

### **空间节省策略**:
- **仅删除 extracted**: 节省约 75% 空间（保留下载缓存）
- **全部删除**: 节省 100% 空间（需要完全重新下载）
- **智能清理**: goclean 等工具会移除未使用模块的两种类型

## 🔍 文件权限

### **Extracted 类型**:
- **只读权限** 应用于所有文件
- 防止意外修改缓存的模块
- 示例: `-r--r--r--` 权限

### **Download 类型**:
- **标准文件权限**
- 用于缓存和验证
- 示例: `-rw-r--r--` 权限

## 💡 最佳实践

1. **定期清理**: 使用 `goclean` 等工具移除未使用的模块
2. **监控使用**: 定期检查 `$GOMODCACHE` 大小
3. **网络意识**: 清理下载缓存时考虑网络成本
4. **构建性能**: 提取缓存加速编译过程

## 🛠️ 手动检查

您可以手动检查缓存：

```bash
# 检查已提取的模块
find $(go env GOMODCACHE) -maxdepth 2 -name "*@v*" -type d

# 检查下载缓存
ls $(go env GOMODCACHE)/cache/download/

# 比较大小
du -sh $(go env GOMODCACHE)/模块路径@版本
du -sh $(go env GOMODCACHE)/cache/download/模块路径
```

## ⚡ 性能影响

- **下载缓存**: 减少网络流量，加快 `go get` 速度
- **提取缓存**: 减少编译时间，无需解压开销
- **两者都缺失**: 需要完整的下载+解压周期

## 🔧 实际示例

以 `gopkg.in/yaml.v3@v3.0.1` 为例：

### **Extracted 类型内容**:
```bash
$ ls -la $GOMODCACHE/gopkg.in/yaml.v3@v3.0.1/
total 1000
dr-xr-xr-x@ 26 abboo  staff    832 Jul 17 22:53 .
-r--r--r--@  1 abboo  staff  21999 Jul 17 22:53 apic.go
-r--r--r--@  1 abboo  staff  42320 Jul 17 22:53 decode_test.go
-r--r--r--@  1 abboo  staff  24953 Jul 17 22:53 decode.go
...
```

### **Download 类型内容**:
```bash
$ ls -la $GOMODCACHE/cache/download/gopkg.in/yaml.v3/@v/
total 248
-rw-r--r--@ 1 abboo  staff      95 Jul 17 22:47 v3.0.1.mod
-rw-r--r--@ 1 abboo  staff  104623 Jul 17 22:53 v3.0.1.zip
-rw-r--r--@ 1 abboo  staff      50 Jul 17 23:23 v3.0.1.info
-rw-r--r--@ 1 abboo  staff      47 Jul 17 22:53 v3.0.1.ziphash
```

### **大小对比**:
- **Extracted**: 504K（完整源代码目录）
- **Download**: 124K（压缩文件+元数据）

理解这两种缓存类型有助于您在模块缓存管理和清理策略方面做出明智的决策。 