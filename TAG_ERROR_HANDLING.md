# 标签写入失败错误处理优化

## 📋 问题背景

### 错误来源
- **模块**: `internal/metadata/writer.go` - `WriteMP4Tags()` 函数
- **第三方库**: `github.com/zhaarey/go-mp4tag`
- **错误类型**: `ilst box not present, implement`
  - MP4文件缺少 `ilst box`（iTunes元数据容器）
  - 该库无法处理没有此元数据容器的文件

### 原问题
1. **刷屏问题**: 错误信息过长导致UI换行，破坏原地刷新效果
2. **计数问题**: 重试失败后计入错误统计，影响成功率显示
3. **重试体验**: 等待时间较长（2秒），影响下载流程连贯性

## ✨ 优化方案

### 1. 原地刷新（不刷屏）
```go
// 截断过长的错误信息，避免换行刷屏
errorMsg := postDownloadError.Error()
if len(errorMsg) > 50 {
    errorMsg = errorMsg[:47] + "..."
}
```

**效果**:
- 错误信息限制在50字符以内
- 避免终端换行，保持UI整洁
- 通过 `ui.UpdateStatus()` 原地更新状态

### 2. 重试三次后自动跳过
```go
if attempt < PostDownloadMaxRetries {
    // 显示重试信息（原地更新，不刷屏）
    ui.UpdateStatus(statusIndex, fmt.Sprintf("重试 %d/%d: %s", attempt, PostDownloadMaxRetries, errorMsg), yellow)
    time.Sleep(1500 * time.Millisecond)
    continue
} else {
    // 所有重试失败，跳过该曲目（不计入错误计数）
    ui.UpdateStatus(statusIndex, "已跳过 (标签失败)", color.New(color.FgYellow).SprintFunc())
    core.Counter.Total++
    // 不增加 Error 计数
    return
}
```

**效果**:
- 自动重试3次，每次间隔1.5秒
- 重试状态原地更新: `重试 1/3: 标签写入失败: ilst box not present...`
- 失败后标记为"已跳过"，**不计入错误统计**
- 不影响整体下载成功率显示

## 📊 优化对比

### 优化前
```
Track 61 of 64: Goldberg Variations, BWV 988 (19... (24bit/44.1kHz) - 标签写入失败: ilst box not present, implement meack
Track 61 of 64: Goldberg Variations, BWV 988 (19... (24bit/44.1kHz) - 第 2/3 次重试...
Track 61 of 64: Goldberg Variations, BWV 988 (19... (24bit/44.1kHz) - 标签写入失败: ilst box not present, implement meack
Track 61 of 64: Goldberg Variations, BWV 988 (19... (24bit/44.1kHz) - 第 3/3 次重试...
Track 61 of 64: Goldberg Variations, BWV 988 (19... (24bit/44.1kHz) - 所有重试均失败: 标签写入失败: ilst box not present, implement
```
- ❌ 错误信息过长，产生换行
- ❌ 多次显示导致刷屏
- ❌ 计入错误统计（Error++）

### 优化后
```
Track 61 of 64: Goldberg Variations, BWV 988 (19... (24bit/44.1kHz) - 重试 1/3: 标签写入失败: ilst box not present...
Track 61 of 64: Goldberg Variations, BWV 988 (19... (24bit/44.1kHz) - 重试 2/3: 标签写入失败: ilst box not present...
Track 61 of 64: Goldberg Variations, BWV 988 (19... (24bit/44.1kHz) - 重试 3/3: 标签写入失败: ilst box not present...
Track 61 of 64: Goldberg Variations, BWV 988 (19... (24bit/44.1kHz) - 已跳过 (标签失败)
```
- ✅ 错误信息截断，不换行
- ✅ 原地更新，不刷屏
- ✅ 自动跳过，不计入错误
- ✅ 重试间隔缩短至1.5秒

## 🔧 技术细节

### 修改位置
- **文件**: `internal/downloader/downloader.go`
- **行数**: 999-1023
- **函数**: `Rip()` - 批次下载主循环的 goroutine 内

### 重试机制
- **最大重试次数**: 3次（`PostDownloadMaxRetries = 3`）
- **重试间隔**: 1.5秒（从2秒优化）
- **重试范围**: 
  - FFmpeg重新编码失败
  - MP4标签写入失败
- **不重试**: 下载失败（`downloadTrackWithFallback` 有自己的重试机制）

### UI更新机制
- **原地更新**: 使用 `ui.UpdateStatus()` 更新指定索引的状态
- **动态刷新**: `RenderUI()` 每300ms刷新一次，通过 ANSI 转义序列控制光标位置
- **颜色标识**: 
  - 黄色: 重试中
  - 黄色: 已跳过
  - 绿色: 下载完成
  - 红色: 下载失败

## 💡 使用说明

### 何时触发
1. 下载的MP4文件缺少 `ilst box` 元数据容器
2. FFmpeg重新编码失败（如启用了 `ffmpeg-fix`）
3. 其他标签写入相关错误

### 预期行为
1. **第1-3次**: 显示 `重试 X/3: [错误信息]`（黄色）
2. **第3次后**: 显示 `已跳过 (标签失败)`（黄色）
3. **统计影响**: 
   - Total 计数 +1
   - Success 计数 **不变**
   - Error 计数 **不变**（跳过不算错误）

### 潜在影响
- 跳过的曲目**不会有元数据标签**（艺术家、专辑、封面等）
- 音频文件本身是完整的，可以正常播放
- 如需元数据，可使用其他工具手动添加（如 MusicBrainz Picard）

## 🐛 根本原因分析

`ilst box not present` 错误通常由以下原因引起：

1. **源文件问题**: Apple Music 服务器返回的文件本身没有元数据容器
2. **FFmpeg处理**: 某些FFmpeg版本在转码时可能移除或未创建 `ilst` atom
3. **库限制**: `go-mp4tag` 库不支持创建 `ilst box`，只能修改已存在的

### 可能的解决方向（未实现）
1. 使用支持创建 `ilst box` 的库（需更换第三方库）
2. 使用 FFmpeg 命令行工具添加元数据容器
3. 放弃使用有问题的源文件，尝试其他音质或区域

## 📝 相关代码

### 错误处理入口
```go:internal/downloader/downloader.go
tagErr := metadata.WriteMP4Tags(trackPath, finalLrc, meta, trackIndexInMeta, len(meta.Data[0].Relationships.Tracks.Data))
if tagErr != nil {
    postDownloadError = fmt.Errorf("标签写入失败: %w", tagErr)
}
```

### 标签写入实现
```go:internal/metadata/writer.go
mp4, err := mp4tag.Open(trackPath)
if err != nil {
    return err  // 如果文件没有 ilst box，这里会报错
}
defer mp4.Close()
err = mp4.Write(t, []string{})
if err != nil {
    return err
}
```

## 🎯 总结

此次优化提升了批量下载时遇到标签写入错误的用户体验：
- ✅ UI更整洁（不刷屏）
- ✅ 响应更快（1.5秒重试间隔）
- ✅ 统计更准确（跳过不算错误）
- ✅ 流程更连贯（自动跳过继续下载）

**注意**: 跳过的曲目音频完整但缺少元数据标签，建议后续使用专业工具补充。

