# goclean - Go模块缓存智能清理工具

中文 | [English](README.md)

## 描述

goclean 是一个智能的 Go 模块缓存清理工具，能够自动识别并清理 Go 项目中未使用的模块。它通过分析 go.mod 文件确定实际依赖关系，扫描模块缓存目录，并提供交互式界面来安全清理未使用的模块，释放磁盘空间。

**核心特性：**
- 🔍 跨多个项目的智能依赖分析
- 🧹 安全清理已提取和已下载的模块缓存
- 💾 磁盘空间计算和回收
- 🔒 交互式确认防止误删除
- ⚡ 并发处理提升性能

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
1.  **发现** 常见工作区中的所有 `go.mod` 文件。
2.  **分析** 所有已发现项目的完整依赖图。
3.  **识别** `$GOMODCACHE` 中未被任何这些项目使用的模块。
4.  **提供** 一个交互式菜单来查看详情并确认删除。

### 指定扫描路径

如果您想将依赖分析限制在特定的项目，请使用 `-modules` 标志，并提供一个用逗号分隔的路径列表。

```bash
# 仅分析单个项目
goclean -modules /path/to/your/project

# 分析多个指定的项目
goclean -modules /path/to/projectA,/path/to/projectB
```

### 模拟运行 (Dry Run)

如果您想查看哪些模块将被删除，而无需实际删除任何文件，请使用 `-dry-run` 标志。强烈建议在首次运行时使用此标志。

```bash
goclean -dry-run
```

### 详细模式 (Verbose)

要在扫描和分析过程中获得更详细的输出，请使用 `-verbose` 标志。

```bash
goclean -verbose
```

### 帮助与版本

要查看所有可用选项或检查当前版本，请使用 `-help` 或 `-version` 标志。

```bash
goclean -help
goclean -version
```

## 示例
关于输出的详细示例，请参阅 [example_ZH.md](example_ZH.md)。

## 命令行选项

| 选项 | 描述 |
|------|------|
| `-modules string` | 要扫描的模块路径，用逗号分隔 |
| `-verbose` | 启用详细输出模式 |
| `-dry-run` | 只模拟运行，不实际删除文件 |
| `-help` | 显示帮助信息 |
| `-version` | 显示版本信息 |

## 注意事项

1. **权限要求**: 删除模块可能需要管理员权限，特别是在系统级Go安装的情况下
2. **安全预防**: 建议先使用 `-dry-run` 参数预览要删除的内容
3. **默认路径**: 如果未指定扫描路径，工具会使用 `$GOPATH/src` 作为默认扫描目录
4. **环境变量**: 工具依赖 `GOMODCACHE` 环境变量定位模块缓存目录
5. **备份建议**: 在大规模清理前建议备份重要项目

## 工作原理

1. **依赖分析阶段**:
   - 扫描指定路径中的所有go.mod文件
   - 解析每个go.mod文件中的直接依赖
   - 使用`go list -m -json all`获取完整的依赖关系图

2. **缓存扫描阶段**:
   - 扫描`$GOMODCACHE`目录中的所有模块
   - 识别已提取的模块目录（格式：`path@version`）
   - 识别已下载的模块文件（`.mod`, `.zip`, `.info`文件）

3. **比较分析阶段**:
   - 比较缓存中的模块与项目中使用的模块
   - 标识未被任何项目使用的模块
   - 计算每个未使用模块占用的磁盘空间

4. **交互清理阶段**:
   - 向用户展示发现的未使用模块
   - 提供详细信息查看和确认删除选项
   - 安全删除选定的模块文件和目录

## 技术实现

- **语言**: Go 1.18+
- **依赖**: 
  - `golang.org/x/mod` - 模块路径解析和编码
- **并发**: 使用goroutine和信号量控制并发计算
- **错误处理**: 完善的错误处理和用户友好的错误信息

## 发布说明

### 如何发布新版本

1. 确保所有更改都已提交到主分支
2. 创建版本标签：
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```
3. 用户可以安装特定版本：
   ```bash
   go install github.com/foabo/goclean@v1.0.0
   ```

### 版本历史

- **v1.0.0**: 初始版本
  - 智能依赖分析
  - 缓存扫描与清理
  - 交互式界面
  - 并发性能优化

## 使用示例

查看 [example_ZH.md](example_ZH.md) 获取完整的使用示例和常见问题解答。

## 理解 Go 模块缓存类型

关于 Go 模块缓存结构的详细说明：
- [Go 模块缓存类型 (中文)](go-module-cache-types_ZH.md) - Extracted 与 Download 类型的详细解释
- [Go Module Cache Types (English)](go-module-cache-types.md) - English version of detailed explanation

## 参考资料

可以参考 [go-mod-clean](https://github.com/fosmjo/go-mod-clean/blob/main/cleaner.go) 的实现。

## 许可证

MIT License
