# 版本管理说明

## 📋 版本管理系统

本项目使用简单但有效的版本管理系统，确保每次编译的二进制文件都包含明确的版本信息。

## 🏷️ 版本号规则

版本号遵循语义化版本 (Semantic Versioning)：

```
v主版本号.次版本号.修订号
```

- **主版本号**：重大功能变更或不兼容的API修改
- **次版本号**：新增功能且向后兼容
- **修订号**：Bug修复和小改进

### 当前版本
```bash
$ cat VERSION
v2.5.0
```

## 🔧 编译方法

### 1. 使用编译脚本（推荐）

```bash
./build.sh
```

**脚本功能**：
- ✅ 自动读取 `VERSION` 文件
- ✅ 自动获取编译时间（北京时间）
- ✅ 自动获取 Git 提交哈希和分支
- ✅ 检测未提交的修改（标记为 `-dirty`）
- ✅ 注入版本信息到二进制文件
- ✅ 显示编译进度和结果

**输出示例**：
```
========================================
🔨 开始编译 Apple Music Downloader
========================================
📦 版本号: v2.5.0 (experiment/improve-txt-batch-tasks)
📅 编译时间: 2025-10-10 12:03:07 CST
🔖 Git提交: f569aa4-dirty
========================================

⚙️  正在编译...

✅ 编译成功！

📄 输出文件: apple-music-downloader
💾 文件大小: 26M

运行方式:
  ./apple-music-downloader
```

### 2. 手动编译（不推荐）

如果手动编译，版本信息将显示为默认值：

```bash
go build -o apple-music-downloader
```

**结果**：
- 版本号：`dev`
- 编译时间：`unknown`
- Git提交：`unknown`

### 3. 带版本信息的手动编译

```bash
VERSION=$(cat VERSION | tr -d '\n')
BUILD_TIME=$(TZ='Asia/Shanghai' date '+%Y-%m-%d %H:%M:%S CST')
GIT_COMMIT=$(git rev-parse --short HEAD)

go build -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.BuildTime=${BUILD_TIME}' -X 'main.GitCommit=${GIT_COMMIT}'" -o apple-music-downloader
```

## 📺 版本信息显示

程序启动时会自动显示版本信息：

```
================================================================================
🎵 Apple Music Downloader v2.5.0 (experiment/improve-txt-batch-tasks)
📅 编译时间: 2025-10-10 12:03:07 CST
🔖 Git提交: f569aa4-dirty
================================================================================

📌 配置文件中未设置 'txtDownloadThreads'，自动设为默认值 5
📌 从配置文件强制使用最大路径长度限制: 255
📌 缓存中转机制已启用，缓存路径: ./Cache
...
```

### 版本信息说明

- **版本号**：来自 `VERSION` 文件
- **分支名**：当前 Git 分支（括号内）
- **编译时间**：中国标准时间（CST）
- **Git提交**：
  - `f569aa4`：提交哈希的前7位
  - `-dirty`：表示有未提交的修改

## 🔄 版本更新流程

### 1. 修复Bug或小改进
```bash
# 更新VERSION文件
echo "v2.5.1" > VERSION

# 提交更改
git add VERSION
git commit -m "chore: bump version to v2.5.1"

# 创建标签
git tag v2.5.1

# 编译
./build.sh
```

### 2. 新功能（向后兼容）
```bash
echo "v2.6.0" > VERSION
git add VERSION
git commit -m "chore: bump version to v2.6.0"
git tag v2.6.0
./build.sh
```

### 3. 重大版本更新
```bash
echo "v3.0.0" > VERSION
git add VERSION
git commit -m "chore: bump version to v3.0.0"
git tag v3.0.0
./build.sh
```

## 📦 发布流程

### 1. 确认版本号
```bash
cat VERSION
# 输出: v2.5.0
```

### 2. 确保代码已提交
```bash
git status
# 应该显示 "nothing to commit, working tree clean"
```

### 3. 编译发布版本
```bash
./build.sh
```

### 4. 创建Git标签
```bash
VERSION=$(cat VERSION | tr -d '\n')
git tag -a ${VERSION} -m "Release ${VERSION}"
```

### 5. 推送标签（如果需要）
```bash
git push origin ${VERSION}
```

## 🔍 查看版本信息

### 查看当前版本文件
```bash
cat VERSION
```

### 查看已编译程序的版本
```bash
./apple-music-downloader --help 2>&1 | head -10
```

### 查看所有Git标签
```bash
git tag -l
```

### 查看特定版本的变更
```bash
git log v2.4.0..v2.5.0 --oneline
```

## 📝 版本历史

### v2.5.0 (2025-10-10)
- ✨ 添加版本管理系统
- 🐛 修复缓存文件转移问题（49个文件滞留）
- 🐛 移除危险的文件校验模式（避免删除用户文件）
- 🎨 优化标签写入失败的错误处理
- 🎨 减小专辑内分批任务之间的视觉间隔

### v2.4.0 (之前)
- ✨ 支持TXT文件单行多链接（空格分隔）
- ✨ 建立历史记录系统和断点续传
- ✨ 优化批量下载任务执行流程
- ✨ 简化配置文件排版，添加中文注释

## 🛠️ 开发者注意事项

### 在开发分支编译
开发分支编译时，版本信息会包含分支名和 `-dirty` 标记：

```
v2.5.0 (experiment/improve-txt-batch-tasks)
Git提交: f569aa4-dirty
```

这可以帮助你确认运行的是哪个版本。

### 版本号含义
- `v2.5.0`：基础版本号（来自 VERSION 文件）
- `(experiment/...)`：当前 Git 分支
- `f569aa4`：Git 提交哈希
- `-dirty`：有未提交的修改

### 清理未提交标记
如果你想要一个"干净"的版本：

```bash
# 提交所有修改
git add .
git commit -m "your commit message"

# 重新编译
./build.sh
```

## 🎯 最佳实践

1. **每次编译都使用 `./build.sh`**
   - 确保版本信息正确
   - 便于追踪问题

2. **发布前更新 VERSION 文件**
   - 提交版本号变更
   - 创建对应的 Git 标签

3. **重大修改后更新版本号**
   - Bug修复：修订号+1 (v2.5.0 → v2.5.1)
   - 新功能：次版本号+1 (v2.5.0 → v2.6.0)
   - 破坏性更改：主版本号+1 (v2.5.0 → v3.0.0)

4. **保持 Git 历史清晰**
   - 使用语义化的提交信息
   - 为每个版本创建标签
   - 记录重要变更

## 📚 相关文件

- **VERSION**：版本号文件
- **build.sh**：编译脚本
- **main.go**：版本信息定义（第25-30行）
- **VERSION_MANAGEMENT.md**：本文档

---

**提示**：从现在开始，请始终使用 `./build.sh` 进行编译，以确保版本信息正确！

