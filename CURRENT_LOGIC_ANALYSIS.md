# 当前下载逻辑分析报告

## 📋 用户期望的逻辑

### 下载逻辑
下载数据 → 缓存文件夹 → 完成加工 → 移动至存储路径 → 清空缓存

### 验重逻辑

#### 无差异情况
读取数据 → 查找存储路径 → 存在文件 → **跳过下载与编码验证**

#### 有差异情况（增量下载）
读取数据 → 查找存储路径 → 存在部分文件（一致性<100%） → **只下载差异数据** → 缓存到缓存路径 → 完成加工 → 移动至存储路径（差异位置） → 清空缓存

## 🔍 当前实现分析

### 1. 文件存在检查（downloadTrackSilently 函数，第413-436行）

```go
// 检查文件是否存在：如果使用缓存，检查最终目标路径；否则检查当前路径
checkPath := trackPath
if finalSaveFolder != baseSaveFolder {
    // 使用缓存时，检查最终目标路径是否已存在文件
    targetAlbumFolder := filepath.Join(targetSingerFolder, finalAlbumDir)
    checkPath = filepath.Join(targetAlbumFolder, finalFilename)
}

exists, err := utils.FileExists(checkPath)
if exists {
    core.OkDict[albumId] = append(core.OkDict[albumId], trackNum)
    return returnPath, nil // 返回实际存在文件的路径，不下载
}
```

**状态**：✅ **正确实现**
- 使用缓存时，检查最终目标路径
- 文件已存在则跳过下载
- 单曲级别的去重

### 2. 全部文件存在检查（Rip 函数，第776-853行）

```go
allFilesExist := true
for _, trackNum := range selected {
    // ... 构建文件路径 ...
    exists, _ := utils.FileExists(checkFilePath)
    if !exists {
        allFilesExist = false
        break
    }
}

// 如果所有文件都已存在，直接跳过
if allFilesExist && len(selected) > 0 {
    core.SafePrintln(green("✅ 跳过下载（所有文件已存在），任务完成！"))
    // 标记所有文件为已完成
    return nil
}
```

**状态**：✅ **正确实现**
- 检查所有选中曲目
- 全部存在时跳过整个专辑
- 专辑级别的去重

### 3. 单曲下载流程（第954-1035行）

```go
trackPath, err := downloadTrackWithFallback(...)
// ↓ trackPath 指向缓存路径
if err != nil {
    return // 下载失败
}

// Step 2: Re-encode if necessary (在缓存路径)
if core.Config.FfmpegFix {
    wasFixed, fixErr = checkAndReEncodeTrack(trackPath, statusIndex)
}

// Step 3: Write tags (在缓存路径)
tagErr := metadata.WriteMP4Tags(trackPath, finalLrc, meta, ...)
```

**状态**：✅ **正确实现**
- 下载到缓存路径
- 在缓存路径完成加工（FFmpeg、标签）
- 等待批次完成后转移

### 4. 批次转移逻辑（第1078-1131行）

```go
// 如果使用了缓存，批次完成后立即转移文件（多批次且不是最后一批）
if usingCache && batch.TotalBatches > 1 && !batch.IsLast {
    // 检查缓存hash目录中是否有新文件需要转移
    cacheHashDir := baseSaveFolder
    
    hasFilesToMove := false
    filepath.Walk(cacheHashDir, func(path string, info os.FileInfo, err error) error {
        if !info.IsDir() && strings.HasSuffix(path, ".m4a") {
            hasFilesToMove = true
            return filepath.SkipDir
        }
        return nil
    })

    if hasFilesToMove {
        // 递归转移所有文件
        filepath.Walk(cacheHashDir, func(cachePath string, info os.FileInfo, walkErr error) error {
            // ... 转移逻辑 ...
            if err := utils.SafeMoveFile(cachePath, targetPath); err == nil {
                moveCount++
            }
            return nil
        })
    }
}
```

**状态**：⚠️ **部分问题**
- 转移时没有检查目标文件是否已存在
- 可能覆盖已有文件
- 没有增量转移的概念

### 5. 最终转移逻辑（第1150-1205行）

```go
// 如果使用了缓存，转移所有缓存文件到目标位置
if usingCache {
    // 递归扫描整个缓存hash目录，查找所有需要转移的文件
    cacheHashDir := baseSaveFolder
    
    hasFilesToMove := false
    err := filepath.Walk(cacheHashDir, func(path string, info os.FileInfo, err error) error {
        if !info.IsDir() && (strings.HasSuffix(path, ".m4a") || strings.HasSuffix(path, ".jpg")) {
            hasFilesToMove = true
            return filepath.SkipDir
        }
        return nil
    })
    
    if err == nil && hasFilesToMove {
        // 递归转移所有子目录
        filepath.Walk(cacheHashDir, func(cachePath string, info os.FileInfo, walkErr error) error {
            // ... 构建目标路径 ...
            targetPath := filepath.Join(finalSaveFolder, relPath)
            
            if info.IsDir() {
                return os.MkdirAll(targetPath, info.Mode())
            }
            
            // 转移文件
            if err := utils.SafeMoveFile(cachePath, targetPath); err != nil {
                fmt.Printf("警告: 转移文件失败 %s: %v\n", relPath, err)
            }
            return nil
        })
    }
}
```

**状态**：❌ **存在问题**
- **没有检查目标文件是否已存在**
- 直接调用 `SafeMoveFile`，可能覆盖
- 无差异化处理

## 🚨 发现的问题

### 问题1：部分存在时的行为不明确

**场景**：专辑有10首歌，其中5首已存在

**当前行为**：
1. 检查 allFilesExist → false（因为有5首不存在）
2. 进入下载循环
3. 对于每首歌：
   - 已存在的5首：`downloadTrackSilently` 检查后跳过 ✅
   - 不存在的5首：下载到缓存 ✅
4. 批次转移/最终转移：
   - 转移缓存中的5首新文件
   - **问题**：没有检查目标路径是否已有文件
   - **可能结果**：如果目标路径有同名文件，会被覆盖

**影响**：
- 理论上问题不大，因为已存在的5首没有被重新下载到缓存
- 但如果缓存中有残留文件，可能导致问题

### 问题2：缓存转移时未检查目标文件存在性

**问题代码**（第1190行）：
```go
// 转移文件
if err := utils.SafeMoveFile(cachePath, targetPath); err != nil {
    fmt.Printf("警告: 转移文件失败 %s: %v\n", relPath, err)
}
```

让我检查 `SafeMoveFile` 的实现：

