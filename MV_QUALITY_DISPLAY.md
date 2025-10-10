# MV 质量信息显示功能

## 📋 概述

v2.2.1 版本新增了 MV 下载时自动显示视频质量信息的功能，让用户清楚了解下载的 MV 分辨率。

## ✨ 功能特性

### 支持的质量标识
- **4K**: 2160P 及以上
- **1080P**: 1920x1080
- **720P**: 1280x720  
- **480P**: 640x480
- **其他**: 自动显示实际分辨率

### 显示格式
```
📺 视频质量: 宽x高 (质量标签)
```

例如：
- `📺 视频质量: 1920x1080 (1080P)`
- `📺 视频质量: 3840x2160 (4K)`
- `📺 视频质量: 1280x720 (720P)`

## 📊 完整下载流程示例

```bash
$ goo https://music.apple.com/cn/music-video/...

🎤 歌手: 薛之谦
🎬 MV: 挚友
📅 发行年份: 2021
🎥 开始下载MV...
📺 视频质量: 1920x1080 (1080P)  ← 新增显示
📤 正在从缓存转移MV文件到目标位置...
📥 MV文件转移完成！
💾 保存路径: /media/Music/.../挚友 (2021).mp4
```

## 🔧 技术实现

### 修改的文件

1. **internal/parser/m3u8.go**
   - `ExtractVideo` 函数签名变更：
     - 旧: `func ExtractVideo(c string) (string, error)`
     - 新: `func ExtractVideo(c string) (string, string, error)`
   - 新增分辨率提取逻辑
   - 根据高度自动判断质量等级

2. **internal/downloader/downloader.go**
   - `MvDownloader` 函数签名变更：
     - 旧: `func MvDownloader(...) (string, error)`
     - 新: `func MvDownloader(...) (string, string, error)`
   - 传递分辨率信息

3. **main.go**
   - `handleSingleMV` 函数中新增质量信息显示
   - 使用 emoji 📺 标识质量信息

### 分辨率提取算法

```go
// 从 m3u8 URL 中提取分辨率
re := regexp.MustCompile(`_(\d+)x(\d+)`)
matches := re.FindStringSubmatch(variant.URI)

// 根据高度判断质量等级
if height >= 2160 {
    qualityLabel = "4K"
} else if height >= 1080 {
    qualityLabel = "1080P"
} else if height >= 720 {
    qualityLabel = "720P"
} else if height >= 480 {
    qualityLabel = "480P"
} else {
    qualityLabel = fmt.Sprintf("%dP", height)
}

// 格式化输出
resolution = fmt.Sprintf("%dx%d (%s)", width, height, qualityLabel)
```

## 📈 优势

1. **信息透明**: 用户在下载前即可了解 MV 质量
2. **决策辅助**: 帮助用户判断是否需要调整质量设置
3. **体验优化**: 通过 emoji 图标提升视觉识别度
4. **无侵入性**: 仅在实际下载时显示，不影响已存在文件的处理

## 🚀 版本信息

- **引入版本**: v2.2.1
- **发布日期**: 2025-10-09
- **兼容性**: 向后兼容所有旧版本

## 💡 使用提示

1. 质量信息仅在**新下载**时显示
2. 如果 MV 文件已存在，将显示 "已存在" 而非质量信息
3. 质量标签基于视频高度自动判断，准确反映实际分辨率
4. 可通过 `--mv-max` 参数限制最大下载分辨率

## 📝 相关配置

```yaml
# 限制最大 MV 分辨率（默认 2160，即 4K）
mv_max: 2160  # 或 1080、720 等
```

---

**注意**: 此功能与之前的 MV 日志修复功能配合使用，共同提供完整的 MV 下载信息反馈。

