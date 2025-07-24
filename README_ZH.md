# goclean - Go模块缓存智能清理工具

中文 | [English](README.md)

## 描述

goclean 是一个智能的 Go 模块缓存清理工具，能够自动识别并清理 Go 项目中未使用的模块。它通过分析 go.mod 文件确定实际依赖关系，扫描模块缓存目录，并提供交互式界面来安全清理未使用的模块，释放磁盘空间。

**核心特性：**
- 🔍 跨多个项目的智能依赖分析
- �� 安全清理已提取和已下载的模块缓存
- 🗂️ VCS（版本控制系统）缓存清理
- 💾 磁盘空间计算和回收
- 🔒 交互式确认防止误删除
- ⚡ 并发处理提升性能
- 🎯 智能版本感知清理策略

## 安装

### 使用 go install （推荐）

```bash
# 安装最新版本
go install github.com/foabo/goclean@latest

# 确保 $GOPATH/bin 在你的 PATH 中
export PATH=$PATH:$(go env GOPATH)/bin

# 安装完成后直接使用
goclean -help
goclean -version
goclean -dry-run
```

**注意**: 确保你的 `$GOPATH/bin` 目录在系统 PATH 环境变量中，这样才能直接运行 `goclean` 命令。

### 编译安装

```bash
# 克隆代码
git clone https://github.com/foabo/goclean.git
cd goclean

# 编译
go build -o goclean

# 移动到PATH目录（可选）
sudo mv goclean /usr/local/bin/
```

### 直接运行

```bash
# 在项目目录中
go run .
```

## 用法

### 默认智能扫描

默认情况下，`goclean` 会智能地发现位于标准路径（`~/go` 和 `$GOPATH/src`）下的所有 Go 项目，并分析它们的依赖关系，以找出您整个系统中真正的孤儿缓存。

只需运行：
```bash
goclean
```

该工具将自动：
1. **发现** 常见工作区中的所有 `go.mod` 文件
2. **分析** 所有已发现项目的完整依赖图
3. **识别** `$GOMODCACHE` 中未被任何这些项目使用的模块
4. **提供** 一个交互式菜单来查看详情并确认删除

### 指定扫描路径

如果您想将依赖分析限制在特定的项目，请使用 `-modules` 标志，并提供一个用逗号分隔的路径列表。

```bash
# 扫描特定项目
goclean -modules /path/to/your/go/project

# 扫描多个项目
goclean -modules ~/project1,~/project2,/opt/project3

# 使用模拟运行预览将要删除的内容
goclean -modules ~/myproject -dry-run -verbose
```

### 命令行选项

```bash
# 基本用法
goclean                              # 智能扫描所有项目
goclean -dry-run                     # 预览模式，不实际删除
goclean -verbose                     # 启用详细输出
goclean -fast                        # 跳过间接依赖分析（企业环境推荐）

# 性能调优
goclean -workers 16                  # 使用16个并发工作线程（默认：8）
goclean -timeout 10                  # 设置10秒go list超时（默认：5秒）

# 企业环境（推荐设置）
goclean -fast -workers 12 -verbose  # 快速、并发、详细日志
goclean -fast -timeout 2 -verbose   # 严格网络环境的超短超时

# 特定项目
goclean -modules ~/go/src/myproject  # 仅扫描特定项目
goclean -modules /tmp/empty_test     # 将所有模块视为未使用（高级用法）
```

### 交互式菜单

分析完成后，goclean 提供交互式菜单：

```
清理摘要:
  📦 发现15个未使用的模块，占用156.7 MB磁盘空间
  🗂️  发现8个VCS缓存条目，占用89.3 MB磁盘空间
  🧹 总共可释放空间: 246.0 MB

您可以:
(1) 查看详细信息
(2) 仅删除未使用的模块 (156.7 MB)
(3) 仅删除VCS缓存 (89.3 MB)
(4) 删除模块和VCS缓存 (246.0 MB)
(5) 退出
```

## 架构

### 核心组件

1. **DependencyAnalyzer** (`dependency_analyzer.go`)
   - 处理所有依赖分析逻辑
   - 支持4种分析方法：go.mod、go.sum、vendor、go list
   - 提供线程安全的分析结果访问

2. **ModuleCleaner** (`cleaner.go`)
   - 管理缓存扫描和清理操作
   - 实现智能版本感知清理策略
   - 处理VCS缓存检测和删除

3. **Utils** (`utils.go`)
   - 文件操作和项目发现的实用功能
   - 在标准位置自动检测Go项目

### 依赖分析方法

goclean 使用多种互补方法确保全面的依赖分析：

1. **go.mod解析**: 从require语句获取直接依赖
2. **go.sum分析**: 从校验和获取额外版本信息  
3. **vendor扫描**: 从vendor目录获取依赖
4. **go list -m -json all**: 完整依赖图（可通过`-fast`跳过）

### 智能清理策略

工具基于依赖元数据实现分层清理策略：

- **直接依赖**: 保守清理 - 保留最新必需版本及更新版本
- **间接依赖**: 激进清理 - 仅保留确切的最新必需版本
- **版本感知**: 语义版本比较确定保留哪些版本
- **双重缓存**: 清理提取式（`$GOMODCACHE/path@version/`）和下载式（`$GOMODCACHE/cache/download/`）缓存

## 工作原理

1. **项目发现阶段**:
   - 自动扫描 `~/go` 和 `$GOPATH/src` 寻找Go项目
   - 查找所有 `go.mod` 文件，跳过 `vendor`、`pkg`、`.git` 目录

2. **依赖分析阶段**:
   - 解析 go.mod 文件获取直接依赖
   - 分析 go.sum 文件获取版本信息
   - 扫描 vendor 目录获取额外依赖
   - 使用 `go list -m -json all` 获取完整依赖图（除非使用`-fast`模式）

3. **缓存扫描阶段**:
   - 扫描 `$GOMODCACHE` 目录中的所有缓存模块
   - 识别提取式和下载式模块缓存
   - 扫描 `$GOMODCACHE/cache/vcs/` 中的VCS缓存

4. **智能清理阶段**:
   - 应用版本感知清理策略
   - 区分直接和间接依赖
   - 使用语义版本比较进行智能决策

5. **交互确认阶段**:
   - 显示发现的未使用模块和VCS缓存
   - 提供详细信息和删除确认
   - 安全删除选定的缓存条目并跟踪进度

## 性能优化

- **并发处理**: 可配置的工作池进行并行分析和大小计算
- **快速模式**: 跳过网络依赖的 `go list`，适用于企业环境
- **智能缓存**: 避免重复分析相同项目
- **优化I/O**: 批量操作和高效文件系统访问

## 企业环境支持

对于有网络限制的企业环境：

```bash
# 企业环境推荐设置
goclean -fast -workers 12 -timeout 2 -verbose

# 环境变量
export GOCLEAN_TIMEOUT="2s"           # 覆盖默认超时
export GOCLEAN_OVERRIDE_PROXY="true"  # 强制代理覆盖（如需要）
```

### 企业环境 Go 代理配置

在企业环境中使用 goclean 之前，请确保正确配置 Go 代理：

```bash
# 企业环境必需的 Go 环境变量
export GOPROXY="https://your-company-proxy.com,direct"     # 公司代理
export GOPRIVATE="*.internal.com,gitlab.company.com"       # 私有模块
export GOSUMDB="off"                                        # 禁用校验和数据库
export GONOPROXY="*.internal.com"                          # 内部模块跳过代理
export GONOSUMDB="*.internal.com"                          # 内部模块跳过校验和
export GOINSECURE="*.internal.com"                         # 内部模块允许不安全连接

# 测试您的配置
cd /path/to/your/project
go list -m -json all
```

**如果 `go list` 失败或超时：**
- 使用 `-fast` 模式跳过网络依赖分析
- 验证您的 GOPROXY 和 GOPRIVATE 设置
- 检查企业防火墙/VPN 连接
- 考虑增加 `-timeout` 或在严格网络环境下使用 `-timeout 2`

`-fast` 模式通过跳过 `go list -m -json all` 并仅依赖本地文件分析来避免网络超时。

## 技术实现

- **语言**: Go 1.18+
- **依赖**: 
  - `golang.org/x/mod` - 模块路径解析和编码
- **并发**: 使用可配置工作池和信号量的goroutine
- **错误处理**: 全面的错误处理和用户友好的消息
- **缓存类型**: 支持提取式和下载式缓存格式
- **版本管理**: 具有智能比较逻辑的语义版本控制

## 示例

完整的使用示例和FAQ请参见 [example_ZH.md](example_ZH.md)。

## 理解Go模块缓存

关于Go模块缓存结构的详细说明：
- [Go Module Cache Types (English)](go-module-cache-types.md) - 提取式和下载式缓存类型的详细说明
- [Go 模块缓存类型 (中文)](go-module-cache-types_ZH.md) - 提取式和下载式缓存类型的详细说明

## 发布说明

### 如何发布新版本

1. 确保所有更改都已提交到主分支
2. 创建版本标签:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```
3. 用户可以安装特定版本:
   ```bash
   go install github.com/foabo/goclean@v1.1.0
   ```

### 版本历史

- **v1.1.0**: 增强的路径解析支持
  - 支持当前目录（`.`）和相对路径（`./project`）
  - 环境变量扩展（`$GOPATH`、`$GOMODCACHE`、`$HOME`）
  - 改进的错误处理和用户反馈
  - 增强的企业环境配置检测
  - 修复异步进度显示，使用原子计数器
  - 更好的路径验证和扩展

- **v1.0.0**: 初始版本，包含智能依赖分析、缓存扫描和清理、交互式界面以及并发性能优化

## 贡献

1. Fork 这个仓库
2. 创建您的功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交您的更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 打开一个 Pull Request

## 许可证

本项目使用 MIT 许可证 - 详细信息请参见 [LICENSE](LICENSE) 文件。

## 常见问题

**问: 为什么goclean需要网络访问？**  
答: `go list -m -json all` 命令需要查询模块代理和校验和数据库来构建完整的依赖图。使用 `-fast` 模式可以避免网络需求。

**问: 删除模块安全吗？**  
答: 是的，Go 会在您构建项目时重新下载任何需要的模块。goclean 只删除没有被任何发现的Go项目引用的模块。

**问: 提取式和下载式缓存有什么区别？**  
答: 提取式缓存包含解压的模块源代码，而下载式缓存包含 .zip、.mod 和 .info 文件。goclean 会清理两种类型。

**问: 版本感知清理是如何工作的？**  
答: goclean 分析实际使用的版本，并根据语义版本规则仅保留必要的版本，对直接依赖和间接依赖采用不同的策略。
