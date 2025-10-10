# 历史记录恢复逻辑检查报告

## 📋 期望的工作流程

```
记录下载链接与音质参数 
  → 添加新任务 
  → 对比历史记录  
    ├─ 无重复 → 进行下载  
    └─ 有重复 → 进入验重逻辑
```

## 🔍 当前实现状态

### ✅ 已实现部分

#### 1. 历史记录数据结构 (`internal/history/history.go`)

```go
// DownloadRecord 下载记录结构
type DownloadRecord struct {
    URL        string    `json:"url"`           // ✅ 下载链接
    AlbumID    string    `json:"album_id"`      // ✅ 专辑ID
    AlbumName  string    `json:"album_name"`    // ✅ 专辑名称
    ArtistName string    `json:"artist_name"`   // ✅ 歌手名称
    Status     string    `json:"status"`        // ✅ 状态（success/failed/skipped）
    DownloadAt time.Time `json:"download_at"`   // ✅ 下载时间
    ErrorMsg   string    `json:"error_msg"`     // ✅ 错误信息
}
```

**❌ 缺失**：音质参数未记录

#### 2. 历史记录对比 (`internal/history/history.go:129-166`)

```go
func GetCompletedURLs(taskFile string) (map[string]bool, error) {
    completedURLs := make(map[string]bool)
    
    // 读取所有历史记录文件
    entries, err := os.ReadDir(historyDir)
    
    // 只考虑相同txt文件的历史记录
    if filepath.Base(task.TaskFile) != baseTaskFile {
        continue
    }
    
    // 收集已成功的URL
    for _, record := range task.Records {
        if record.Status == "success" {
            completedURLs[record.URL] = true  // ✅ 只对比URL
        }
    }
    
    return completedURLs, nil
}
```

**✅ 实现**：读取历史记录，收集已完成的URL
**❌ 缺失**：未对比音质参数

#### 3. 历史记录过滤 (`main.go:314-343`)

```go
// 检查历史记录，获取已完成的URL
completedURLs, err = history.GetCompletedURLs(taskFile)

// 过滤已完成的URL
skippedCount := 0
var remainingUrls []string
for _, url := range finalUrls {
    if completedURLs[url] {  // ✅ 只对比URL
        skippedCount++
    } else {
        remainingUrls = append(remainingUrls, url)
    }
}

if skippedCount > 0 {
    core.SafePrintf("📜 历史记录检测: 发现 %d 个已完成的任务\n", skippedCount)
    core.SafePrintf("⏭️  已自动跳过，剩余 %d 个任务\n\n", len(remainingUrls))
    finalUrls = remainingUrls
}
```

**✅ 实现**：根据URL过滤已完成任务
**❌ 缺失**：未考虑音质参数变化

#### 4. 历史记录保存 (`main.go:374-391`)

```go
// 记录到历史
if task != nil && albumId != "" {
    status := "success"
    errorMsg := ""
    if err != nil {
        status = "failed"
        errorMsg = err.Error()
    }
    
    history.AddRecord(history.DownloadRecord{
        URL:        urlToProcess,    // ✅ 记录URL
        AlbumID:    albumId,          // ✅ 记录专辑ID
        AlbumName:  albumName,        // ✅ 记录专辑名称
        Status:     status,           // ✅ 记录状态
        DownloadAt: time.Now(),       // ✅ 记录时间
        ErrorMsg:   errorMsg,         // ✅ 记录错误
    })
}
```

**✅ 实现**：保存基本信息
**❌ 缺失**：未记录音质参数

### ❌ 未实现部分

#### 1. 音质参数记录

**当前状态**：DownloadRecord 结构体中**没有音质参数字段**

**音质相关配置**（`config.yaml`）：
```yaml
# 音频格式配置
get-m3u8-mode: "hires"           # M3U8 获取模式（all: 获取所有, hires: 仅探测 Hi-Res）
aac-type: "aac-lc"               # AAC 类型（aac-lc, aac, aac-binaural, aac-downmix）
alac-max: 192000                 # ALAC 最大采样率（192000, 96000, 48000, 44100）
atmos-max: 2768                  # Atmos 最大码率（2768, 2448）
```

**需要记录的音质参数**：
- `get-m3u8-mode`: M3U8 获取模式
- `aac-type`: AAC 类型
- `alac-max`: ALAC 最大采样率
- `atmos-max`: Atmos 最大码率
- `实际下载格式`: ALAC/Atmos/AAC（实际使用的格式）
- `实际采样率`: 44.1kHz/48kHz/96kHz/192kHz（实际使用的采样率）

#### 2. 音质参数对比逻辑

**当前状态**：GetCompletedURLs 只返回 `map[string]bool`，仅对比 URL

**需要实现**：
```go
// 建议的新数据结构
type CompletedRecord struct {
    URL           string
    AlbumID       string
    QualityParams QualityParams  // 音质参数
    DownloadAt    time.Time
}

type QualityParams struct {
    M3U8Mode      string  // get-m3u8-mode
    AACType       string  // aac-type
    ALACMax       int     // alac-max
    AtmosMax      int     // atmos-max
    ActualFormat  string  // 实际格式（ALAC/Atmos/AAC）
    ActualBitrate string  // 实际码率/采样率
}
```

#### 3. 验重逻辑

**当前状态**：**完全未实现**

**需要实现的验重逻辑**：

```
新任务对比历史记录：
  ├─ URL 不同 → 正常下载
  └─ URL 相同
      ├─ 音质参数相同 → 跳过下载
      └─ 音质参数不同 → 进入验重流程
          ├─ 自动模式：
          │   ├─ 新音质 > 旧音质 → 下载并覆盖（记录升级）
          │   └─ 新音质 ≤ 旧音质 → 跳过下载
          └─ 交互模式（可选）：
              └─ 询问用户：
                  ├─ 重新下载（覆盖）
                  ├─ 跳过
                  └─ 保留两者（不同文件夹）
```

## 📊 实现进度

| 功能模块 | 期望 | 当前状态 | 完成度 |
|---------|------|---------|--------|
| **记录下载链接** | ✅ | ✅ 已实现 | 100% |
| **记录音质参数** | ✅ | ❌ 未实现 | 0% |
| **添加新任务** | ✅ | ✅ 已实现 | 100% |
| **对比历史记录** | ✅ | ⚠️ 仅对比URL | 30% |
| **URL对比** | ✅ | ✅ 已实现 | 100% |
| **音质参数对比** | ✅ | ❌ 未实现 | 0% |
| **无重复处理** | ✅ | ✅ 已实现 | 100% |
| **验重逻辑** | ✅ | ❌ 未实现 | 0% |

**总体完成度**: **约 40%**

## ⚠️ 当前存在的问题

### 问题1：音质参数未记录

**场景**：
```bash
# 第一次：使用 alac-max: 44100 下载专辑
$ ./apple-music-downloader https://music.apple.com/cn/album/...

# 修改配置：alac-max: 192000

# 第二次：尝试用更高音质下载同一专辑
$ ./apple-music-downloader same-album.txt

结果：❌ 系统检测到历史记录，直接跳过下载
期望：✅ 系统识别音质参数变化，提示用户或自动升级音质
```

### 问题2：无法区分音质升级和重复下载

**当前行为**：
- 只要 URL 在历史记录中存在，就跳过下载
- 不管音质参数是否变化

**用户困扰**：
1. **音质升级场景**：
   - 用户最初用标准音质下载
   - 后来想升级到 Hi-Res
   - 系统误判为重复下载，跳过
   
2. **配置错误场景**：
   - 用户错误配置了低音质
   - 修正配置后重新下载
   - 系统认为已完成，跳过

3. **格式切换场景**：
   - 用户想测试不同 AAC 类型
   - 系统无法识别格式差异

### 问题3：历史记录无法溯源音质

**当前历史记录示例**：
```json
{
  "task_id": "albums_1728558000",
  "records": [
    {
      "url": "https://music.apple.com/cn/album/...",
      "album_id": "1234567890",
      "album_name": "示例专辑",
      "status": "success",
      "download_at": "2025-10-10T10:00:00Z"
    }
  ]
}
```

**问题**：
- 无法得知当时使用的音质参数
- 无法判断是否需要重新下载
- 无法追溯音质历史

## 💡 建议的改进方案

### 方案1：最小改动方案（推荐）

#### 1.1 扩展 DownloadRecord 结构体

```go
// DownloadRecord 下载记录结构
type DownloadRecord struct {
    URL          string    `json:"url"`
    AlbumID      string    `json:"album_id"`
    AlbumName    string    `json:"album_name,omitempty"`
    ArtistName   string    `json:"artist_name,omitempty"`
    Status       string    `json:"status"`
    DownloadAt   time.Time `json:"download_at"`
    ErrorMsg     string    `json:"error_msg,omitempty"`
    
    // 新增：音质参数
    QualityHash  string    `json:"quality_hash"`          // 音质参数哈希（用于快速对比）
    M3U8Mode     string    `json:"m3u8_mode,omitempty"`   // get-m3u8-mode
    AACType      string    `json:"aac_type,omitempty"`    // aac-type
    ALACMax      int       `json:"alac_max,omitempty"`    // alac-max
    AtmosMax     int       `json:"atmos_max,omitempty"`   // atmos-max
    ActualFormat string    `json:"actual_format,omitempty"` // 实际下载格式
    ActualQuality string   `json:"actual_quality,omitempty"` // 实际音质（如 "24bit/192kHz"）
}
```

#### 1.2 修改对比逻辑

```go
// 新增：生成音质参数哈希
func GetQualityHash() string {
    data := fmt.Sprintf("%s|%s|%d|%d",
        core.Config.GetM3u8Mode,
        core.Config.AacType,
        core.Config.AlacMax,
        core.Config.AtmosMax,
    )
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:8]) // 使用前8字节
}

// 修改：GetCompletedURLs 返回更多信息
func GetCompletedRecords(taskFile string) (map[string]*DownloadRecord, error) {
    completedRecords := make(map[string]*DownloadRecord)
    // ... 读取历史记录 ...
    for _, record := range task.Records {
        if record.Status == "success" {
            completedRecords[record.URL] = &record
        }
    }
    return completedRecords, nil
}

// 修改：过滤逻辑（在 main.go）
currentQualityHash := GetQualityHash()
completedRecords, err := history.GetCompletedRecords(taskFile)

for _, url := range finalUrls {
    if oldRecord, exists := completedRecords[url]; exists {
        if oldRecord.QualityHash == currentQualityHash {
            // 音质参数相同，跳过
            skippedCount++
        } else {
            // 音质参数不同，提示用户或自动处理
            core.SafePrintf("⚠️  检测到音质变化: %s\n", url)
            core.SafePrintf("   旧音质: %s (%s)\n", oldRecord.ActualFormat, oldRecord.ActualQuality)
            core.SafePrintf("   新音质: alac-max=%d, atmos-max=%d\n", 
                core.Config.AlacMax, core.Config.AtmosMax)
            
            // 可选：询问用户或根据配置自动决定
            if shouldUpgrade(oldRecord) {
                core.SafePrintf("   → 音质升级，重新下载\n")
                remainingUrls = append(remainingUrls, url)
            } else {
                core.SafePrintf("   → 跳过重新下载\n")
                skippedCount++
            }
        }
    } else {
        // 新链接
        remainingUrls = append(remainingUrls, url)
    }
}
```

#### 1.3 配置选项

在 `config.yaml` 中新增：

```yaml
# ========== 历史记录配置 ==========
history-quality-check: true                           # 是否检查音质参数变化
history-auto-upgrade: true                            # 音质升级时自动重新下载
history-quality-tolerance: "strict"                   # 音质对比严格度（strict: 严格, loose: 宽松）
```

### 方案2：完整验重系统（高级）

**包含**：
- 音质参数记录和对比
- 智能音质升级检测
- 文件指纹验证（防止手动删除）
- 多版本管理（同一专辑多个音质）
- 历史记录可视化工具

**复杂度**：较高，需要更多开发时间

## 📝 实现优先级建议

### P0（高优先级）

1. ✅ **记录音质参数**
   - 扩展 DownloadRecord 结构体
   - 添加 QualityHash 字段
   - 记录关键音质参数（alac-max, atmos-max, aac-type）

2. ✅ **音质参数对比**
   - 修改 GetCompletedURLs 为 GetCompletedRecords
   - 对比 QualityHash
   - 检测音质变化

### P1（中优先级）

3. ✅ **验重提示**
   - 检测到音质变化时显示信息
   - 询问用户是否重新下载
   - 或根据配置自动决定

4. ✅ **配置选项**
   - history-quality-check
   - history-auto-upgrade

### P2（低优先级）

5. ⭕ **音质升级逻辑**
   - 智能判断音质高低
   - 自动升级模式

6. ⭕ **历史记录工具**
   - 查看历史音质
   - 批量重新下载指定音质

## 🎯 总结

### ✅ 已实现

- ✅ 基本的历史记录系统
- ✅ URL级别的重复检测
- ✅ 任务执行记录和统计

### ❌ 未实现（关键缺失）

- ❌ **音质参数记录**
- ❌ **音质参数对比**
- ❌ **验重逻辑**

### 🚨 影响

**当前系统的局限性**：
1. **无法识别音质升级需求** - 用户想升级音质时会被误判为重复
2. **历史记录不完整** - 无法追溯当时的音质参数
3. **配置变化不生效** - 修改音质配置后无法重新下载

**建议**：
- 优先实现 P0 功能（记录和对比音质参数）
- 这是一个基础功能，对用户体验影响很大
- 实现难度中等，投入产出比高

## 📖 相关文件

- `/root/apple-music-downloader/internal/history/history.go` - 历史记录核心逻辑
- `/root/apple-music-downloader/main.go` (L314-343) - 历史记录过滤
- `/root/apple-music-downloader/config.yaml` - 音质参数配置
- `/root/apple-music-downloader/utils/structs/structs.go` - 配置结构体定义

---

**检查完成时间**: 2025-10-10
**项目状态**: ⚠️ 音质参数记录和验重逻辑**未实现**
**建议行动**: 优先实现音质参数记录功能

