# 🚀 goo 命令别名配置说明

## 📋 基本信息

| 项目 | 内容 |
|------|------|
| **命令别名** | `goo` |
| **指向文件** | apple-music-downloader |
| **配置文件** | `~/.zshrc` |
| **二进制路径** | `/root/apple-music-downloader/apple-music-downloader` |
| **文件大小** | 26MB |
| **编译时间** | 2025-10-09 07:11 |
| **版本特性** | 包含中文帮助、emoji美化、MV质量显示、所有最新功能 |

---

## ✨ 特性说明

### 🔧 完整功能集成

✅ **日志UI治理** - OutputMutex + SafePrintf  
✅ **中文帮助菜单** - 所有参数说明已汉化  
✅ **Emoji美化** - 终端输出更直观美观  
✅ **--no-ui 模式** - 纯日志输出支持  
✅ **缓存机制** - NFS性能优化  
✅ **所有最新功能** - 包含MV质量显示等新特性

### 📤 Emoji 输出示例

```bash
📌 配置文件中未设置 'txtDownloadThreads'，自动设为默认值 5
📌 缓存中转机制已启用，缓存路径: ./Cache

📋 开始下载任务
📝 总数: 2, 并发数: 1

🧾 [1/2] 开始处理: https://music.apple.com/...
🎤 歌手: Anaïs Reno
💽 专辑: Lovesome Thing
🔬 正在进行版权预检，请稍候...
📡 音源: Lossless | 5 线程 | CN | 1 个账户并行下载

📤 正在从缓存转移文件到目标位置...
📥 文件转移完成！

📦 已完成: 25/25 | 警告: 0 | 错误: 0
```

---

## 🚀 使用方法

### 基本命令

```bash
# 查看帮助（完整中文说明）
goo --help

# 下载专辑
goo https://music.apple.com/cn/album/...

# 下载杜比全景声
goo --atmos https://music.apple.com/cn/album/...

# 下载单曲
goo --song https://music.apple.com/cn/album/...?i=...

# 下载播放列表
goo https://music.apple.com/cn/playlist/...

# 批量下载
goo url1 url2 url3
```

### 高级选项

```bash
# 纯日志模式（无动态UI）
goo --no-ui <url>

# 选择性下载
goo --select <album-url>

# 交互式搜索
goo --search song "歌曲名"
goo --search album "专辑名"
goo --search artist "歌手名"

# 下载歌手所有专辑
goo --all-album <artist-url>

# 调试模式（查看音质信息）
goo --debug <url>
```

### 音质选项

```bash
# ALAC 高解析度
goo --alac-max 192000 <url>

# Dolby Atmos
goo --atmos --atmos-max 2768 <url>

# AAC 格式
goo --aac --aac-type aac-lc <url>

# MV 下载
goo --mv-max 2160 <mv-url>
```

---

## ⚙️ 配置详情

### Shell 配置

**文件位置**: `~/.zshrc`

**配置内容**:
```bash
alias goo='/root/apple-music-downloader/apple-music-downloader'
```

### 生效方式

#### 方式1: 重新加载配置（当前终端）
```bash
source ~/.zshrc
```

#### 方式2: 新开终端
新开的终端会自动加载配置，`goo` 命令直接可用

#### 方式3: 手动测试
```bash
alias goo='/root/apple-music-downloader/apple-music-downloader'
```

---

## 🔄 版本对比

### 传统方式 vs goo 命令

| 对比项 | 传统方式 | goo 命令 |
|--------|---------|----------|
| **启动方式** | `go run main.go` | `goo` |
| **启动速度** | 慢（每次编译） | 快（直接运行） |
| **命令长度** | 15个字符 | 3个字符 |
| **版本** | 开发版 | 最新稳定编译版 |
| **包含功能** | 当前代码 | 所有最新功能 |

### 性能提升

- ⚡ **启动时间**: 从 ~2秒 → <0.1秒
- 📝 **命令简化**: 减少 80% 字符输入
- 🎯 **版本稳定**: 使用编译版本，避免代码变动影响

---

## 📝 维护说明

### 更新到最新版本

当有新代码时，重新编译即可（无需修改别名）：

```bash
# 1. 重新编译（覆盖原文件）
cd /root/apple-music-downloader
go build -ldflags "-s -w" -o apple-music-downloader main.go

# 2. 验证更新
goo --help

# 说明：使用统一的二进制文件名，无需每次修改别名配置
```

### 回退到开发模式

如需使用 `go run` 方式测试代码：

```bash
# 临时回退
alias goo='go run main.go'

# 永久回退（修改 ~/.zshrc）
sed -i "s|/root/apple-music-downloader/apple-music-downloader|go run main.go|g" ~/.zshrc
source ~/.zshrc
```

### 维护策略

**⚠️ 重要：统一二进制文件管理**

项目采用**单一二进制文件**策略，避免版本混乱：

```bash
# ✅ 正确：只保留一个二进制文件
/root/apple-music-downloader/apple-music-downloader

# ❌ 错误：不要创建多个版本
# apple-music-downloader-v2.2.0
# apple-music-downloader-v2.2.1
# apple-music-downloader.baseline

# 清理旧版本文件（如有）
rm -f /root/apple-music-downloader/apple-music-downloader-v*
rm -f /root/apple-music-downloader/apple-music-downloader.baseline

# 验证只有一个文件
ls -lh /root/apple-music-downloader/apple-music-downloader
```

**优势：**
- 🎯 版本清晰，易于识别
- 🔧 维护简单，无需修改别名
- 📦 节省空间，避免冗余
- ✅ 更新方便，直接覆盖编译

---

## 🎯 快速参考

### 常用命令速查

```bash
# 帮助
goo --help

# 标准下载
goo <url>

# Atmos
goo --atmos <url>

# 纯日志
goo --no-ui <url>

# 搜索
goo --search song "关键词"
```

### 配置文件

```bash
# 编辑配置
nano ~/apple-music-downloader/config.yaml

# 查看示例
cat ~/apple-music-downloader/config.yaml.example
```

### 日志输出

```bash
# 保存日志
goo --no-ui <url> > download.log 2>&1

# 实时查看日志
goo <url> | tee download.log
```

---

## 💡 提示和技巧

### 1. 命令补全
```bash
# zsh 通常支持别名补全
goo --[Tab][Tab]  # 显示所有可用选项
```

### 2. 多任务下载
```bash
# 并行下载多个专辑
goo url1 url2 url3 url4 url5

# 配置文件设置并发数
# config.yaml: txtDownloadThreads: 5
```

### 3. CI/CD 集成
```bash
# Jenkins / GitLab CI
goo --no-ui <url> > output.log
if [ $? -eq 0 ]; then
  echo "下载成功"
else
  echo "下载失败"
  exit 1
fi
```

### 4. 快捷函数
```bash
# 添加到 ~/.zshrc
function goo-atmos() {
  goo --atmos "$@"
}

function goo-search() {
  goo --search song "$@"
}

# 使用
goo-atmos <url>
goo-search "歌名"
```

---

## 🔗 相关资源

### 文档

- [README.md](./README.md) - 主文档
- [README-CN.md](./README-CN.md) - 中文文档  
- [CHANGELOG.md](./CHANGELOG.md) - 版本历史
- [RELEASE_v2.2.0.md](./RELEASE_v2.2.0.md) - 发布说明
- [EMOJI_DEMO.md](./EMOJI_DEMO.md) - Emoji 演示

### 工具

- [wrapper](https://github.com/zhaarey/wrapper) - 解密服务
- [MP4Box](https://gpac.io/downloads/gpac-nightly-builds/) - 必需工具
- [mp4decrypt](https://www.bento4.com/downloads/) - MV 下载

---

## ✅ 验证清单

使用前请确认：

- [ ] MP4Box 已安装并在 PATH 中
- [ ] wrapper 解密服务正在运行
- [ ] config.yaml 已正确配置
- [ ] media-user-token 已填写（如需歌词/MV）
- [ ] goo 命令可正常执行
- [ ] 帮助菜单显示中文
- [ ] Emoji 正常显示

---

## 🎉 总结

**goo 命令别名配置成功！**

现在您可以：
- ✅ 使用简短的 `goo` 命令启动下载器
- ✅ 享受快速的启动速度（<0.1秒）
- ✅ 体验完整的中文帮助菜单
- ✅ 查看美观的 Emoji 输出
- ✅ 使用所有 v2.2.0 新功能

**开始您的高品质音乐下载之旅吧！** 🎵

---

**配置时间**: 2025-10-09  
**最后更新**: 2025-10-09 07:11  
**状态**: ✅ 已生效

