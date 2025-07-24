# goclean 使用示例

## 完整工作流程演示

### 1. 安装工具

```bash
# 使用 go install 安装
go install github.com/foabo/goclean@latest

# 验证安装
goclean -version
```

### 2. 基本使用

```bash
# 智能扫描（自动发现 ~/go 和 $GOPATH/src 中的项目）
goclean

# 模拟运行，查看将要删除的模块
goclean -dry-run -verbose

# 企业环境（推荐）
goclean -fast -workers 12 -verbose
```

### 3. 高级使用

```bash
# 扫描特定项目
goclean -modules ~/go/src/myproject -verbose

# 扫描多个目录
goclean -modules ~/project1,~/project2,/opt/project3 -dry-run

# 高性能系统的性能调优
goclean -workers 16 -timeout 10 -verbose

# 严格网络环境的企业设置
goclean -fast -timeout 2 -workers 8 -verbose
```

### 4. 预期输出示例

#### 智能扫描输出
```
🔎 未提供特定模块路径，正在自动发现项目...
✅ 发现 3 个项目。正在分析其依赖关系...
   要扫描的项目:
   - /Users/user/go/src/github.com/mycompany/project1
   - /Users/user/go/src/github.com/mycompany/project2
   - /Users/user/go/pkg/mod/cache/download

🔧 配置:
  - 模块缓存目录: /Users/user/go/pkg/mod
  - 扫描路径: [/Users/user/go/src/github.com/mycompany/project1 ...]
  - 详细模式: true
  - 模拟运行: false
  - 快速模式: false
  - 最大工作线程: 16
  - 超时时间: 60s

🚀 开始Go模块缓存清理...
🔍 分析 3 个模块路径...
使用 8 个并发工作线程进行分析
[1/3] 处理中: /Users/user/go/src/github.com/mycompany/project1
    ✅ 成功使用 go.mod 分析
    ✅ 成功使用 go.sum 分析
    ✅ 成功使用 go list 分析
[2/3] 处理中: /Users/user/go/src/github.com/mycompany/project2
    ✅ 成功使用 go.mod 分析
    ⚠️  go.sum 分析失败: 未找到 go.sum
    ✅ 成功使用 go list 分析
[3/3] 处理中: /Users/user/go/pkg/mod/cache/download
...

🔍 发现 45 个唯一模块，共 67 个版本
📊 增强分析: 52 个模块具有完整元数据
  📦 直接依赖: 23
  🔗 间接依赖: 29
💡 智能版本清理: 发现 8 个模块有多个版本
   将仅保留每个模块的最新必需版本
```

#### 交互式菜单输出
```
清理摘要:
  📦 发现 23 个未使用的模块，占用 245.7 MB 磁盘空间
  🗂️  发现 12 个 VCS 缓存条目，占用 156.3 MB 磁盘空间
  🧹 总共可释放空间: 402.0 MB

您可以:
(1) 查看详细信息
(2) 仅删除未使用的模块 (245.7 MB)
(3) 仅删除 VCS 缓存 (156.3 MB)
(4) 删除模块和 VCS 缓存 (402.0 MB)
(5) 退出

请输入括号中的数字: 1
```

#### 详细信息输出
```
未使用模块详细信息:
--------------------------------------------------------------------------------
📦 模块: github.com/gin-gonic/gin (总计: 28.4 MB)
   📁 缓存: $GOMODCACHE/github.com/gin-gonic/gin
   ├─ v1.8.0 (14.2 MB, 可删除)
   ├─ v1.8.1 (14.2 MB, 可删除)

📦 模块: golang.org/x/crypto (总计: 15.6 MB)
   📁 缓存: $GOMODCACHE/golang.org/x/crypto
   ├─ v0.10.0 (7.8 MB, 可删除)
   ├─ v0.11.0 (7.8 MB, 可删除)
   
VCS 缓存详细信息:
--------------------------------------------------------------------------------
🗂️  哈希: a1b2c3d4e5f6
   📁 仓库: https://github.com/gin-gonic/gin.git
   💾 大小: 45.2 MB
   📅 最后使用: 2024-01-15
   📁 缓存路径: $GOMODCACHE/cache/vcs/a1b2c3d4e5f6
```

#### 模拟运行输出
```
模拟运行模式: 将要删除以下模块
  - github.com/gin-gonic/gin@v1.8.0 (14.2 MB)
  - github.com/gin-gonic/gin@v1.8.1 (14.2 MB)
  - golang.org/x/crypto@v0.10.0 (7.8 MB)
  - golang.org/x/crypto@v0.11.0 (7.8 MB)
  ...

📊 计算当前缓存统计信息...

🎯 模拟运行摘要:
  📦 当前缓存大小: 2.1 GB
  🧹 将释放空间: 245.7 MB
  📊 占缓存百分比: 11.7%
```

#### 实际删除输出
```
确认删除这些模块? (y/N): y
📊 计算缓存统计信息...
删除前缓存大小: 2.1 GB
开始删除 23 个未使用的模块...
已删除: github.com/gin-gonic/gin@v1.8.0 (14.2 MB)
已删除: github.com/gin-gonic/gin@v1.8.1 (14.2 MB)
已删除: golang.org/x/crypto@v0.10.0 (7.8 MB)
...

🎯 删除摘要:
  ✅ 已删除模块: 23/23
  ⏱️  用时: 3.2s
  📦 删除前缓存大小: 2.1 GB
  📦 删除后缓存大小: 1.9 GB
  🧹 释放空间: 245.7 MB (11.7%)
```

## 高级应用场景

### 1. 企业环境

```bash
# 针对网络访问受限的环境
goclean -fast -workers 12 -timeout 2 -verbose

# 预期输出:
🚀 开始Go模块缓存清理...
🔍 分析 5 个模块路径...
[1/5] 处理中: /enterprise/project1
    ✅ 成功使用 go.mod 分析
    ✅ 成功使用 go.sum 分析
    快速模式: 跳过 /enterprise/project1 的间接依赖
[2/5] 处理中: /enterprise/project2
    ✅ 成功使用 go.mod 分析
    ⚠️  go.sum 分析失败: 未找到 go.sum
    快速模式: 跳过 /enterprise/project2 的间接依赖
...
```

### 2. 性能优化

```bash
# 针对高性能开发机器
goclean -workers 16 -verbose

# 预期输出显示优化的工作线程使用:
使用 16 个并发工作线程进行分析
    使用 10 个工作线程进行大小计算
    使用 8 个工作线程进行下载大小计算
```

### 3. 版本感知清理

当存在多个版本时，goclean 使用智能策略:

```bash
# 示例: gin 模块有多个版本
📦 模块: github.com/gin-gonic/gin (总计: 42.6 MB)
   📁 缓存: $GOMODCACHE/github.com/gin-gonic/gin
   ├─ v1.8.0 (14.2 MB, 可删除)    # 旧版本 - 将被清理
   ├─ v1.9.1 (14.2 MB, 保留)     # 最新必需版本 - 保留
   ├─ v1.10.0 (14.2 MB, 保留)    # 更新版本 - 为兼容性保留

# 直接依赖: 保守清理（保留 v1.9.1 和 v1.10.0）
# 间接依赖: 激进清理（仅保留 v1.9.1）
```

## 常见问题

### Q: 为什么没有找到未使用的模块？
A: 可能的原因：
- 您的项目使用了缓存中的大部分模块
- 指定的扫描路径不正确
- 模块缓存目录为空
- 所有缓存的模块都是最新的且必需的

### Q: 工具如何确定保留哪些版本？
A: goclean 使用语义版本控制和不同策略：
- **直接依赖**: 保守策略 - 保留最新必需版本及更新版本
- **间接依赖**: 激进策略 - 仅保留确切的最新必需版本
- **版本比较**: 使用语义版本规则（major.minor.patch）

### Q: 如果我删除了实际需要的模块会怎样？
A: 没问题！Go 会在您构建项目时自动重新下载任何需要的模块。

### Q: 提取式和下载式缓存有什么区别？
A: 
- **提取式缓存**: `$GOMODCACHE/path@version/` 中的解压源代码
- **下载式缓存**: `$GOMODCACHE/cache/download/` 中的压缩文件（.zip、.mod、.info）
- goclean 清理两种类型以最大化空间回收

### Q: 为什么 goclean 需要网络访问？
A: `go list -m -json all` 命令需要查询模块代理和校验和数据库来构建完整的依赖图。使用 `-fast` 模式可以避免网络需求。

### Q: 如何提高性能？
A: 
- 使用 `-workers N` 增加并发性（默认: 16）
- 使用 `-fast` 模式跳过网络依赖分析
- 使用 `-timeout N` 控制 go list 超时时间（默认: 60s）

### Q: 在企业环境中运行安全吗？
A: 是的！使用这些推荐设置:
```bash
goclean -fast -workers 12 -timeout 2 -verbose
```

### Q: 什么是 VCS 缓存，我应该清理它吗？
A: VCS 缓存在 `$GOMODCACHE/cache/vcs/` 中存储 git 仓库信息。清理是安全的，因为 Go 会在需要时重新克隆仓库。

## 故障排除

### 网络超时
```bash
# 问题: 企业环境中 go list 超时
⚠️  警告: /project 的 go list 超时 (60s) - 跳过间接依赖

# 解决方案: 使用快速模式
goclean -fast -verbose
```

### 权限被拒绝
```bash
# 问题: 删除时权限被拒绝
删除失败: github.com/gin-gonic/gin@v1.8.0 - 权限被拒绝

# 解决方案: 确保文件可写（goclean 自动处理）
# 或使用适当权限运行
```

### 未找到项目
```bash
# 问题: 在标准位置未找到 Go 项目
⚠️ 在标准位置（~/go, $GOPATH/src）未找到 Go 项目。

# 解决方案: 手动指定项目路径
goclean -modules /path/to/your/projects
```

## 性能基准

不同系统的典型性能:

| 系统 | 项目数 | 模块数 | 时间 | 工作线程 |
|------|--------|--------|------|----------|
| 笔记本电脑 (8核) | 5 | 150 | 2.3s | 8 |
| 工作站 (16核) | 20 | 500 | 4.1s | 16 |
| 企业 (12核, -fast) | 50 | 800 | 3.7s | 12 |

## 最佳实践

1. **首次运行**: 始终使用 `-dry-run` 预览
2. **企业环境**: 使用 `-fast` 模式避免网络问题
3. **性能**: 根据您的 CPU 核心数调整 `-workers`
4. **安全**: 定期备份重要项目
5. **监控**: 使用 `-verbose` 获取详细日志 