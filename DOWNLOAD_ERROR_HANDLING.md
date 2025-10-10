# 下载失败错误处理优化

## 🚨 问题描述

用户报告的错误信息：

```
下载失败: 所有可用账户均尝试失败: failed to run v14 with account CN: dial tcp 192.168.1.9:10020: connect: connection refused
```

### 问题分析

1. **错误信息过长**：
   - 包含完整的错误链：账户 → 服务 → 网络错误
   - 超过终端宽度，导致换行刷屏
   - 难以阅读和定位问题

2. **重复刷屏**：
   - 每次重试都显示完整错误
   - 切换账户时也显示提示
   - 大量信息滚动，影响观看体验

3. **无效重试**：
   - `connection refused` 错误通常是服务未启动
   - 反复重试相同错误，浪费时间
   - 没有智能跳过机制

## 🔍 错误来源

### 调用链

```
Rip() 主下载循环 (第919行)
  └─> downloadTrackWithFallback() (第95-154行)
       └─> downloadTrackSilently() (第156行)
            └─> runv14.Run() (第416行) ← 这里抛出 connection refused
```

### 错误构建过程

1. **第418行**：`runv14.Run` 失败
   ```go
   return "", fmt.Errorf("failed to run v14 with account %s: %w", account.Name, err)
   // → "failed to run v14 with account CN: dial tcp 192.168.1.9:10020: connect: connection refused"
   ```

2. **第153行**：`downloadTrackWithFallback` 返回
   ```go
   return "", fmt.Errorf("所有账户失败: %s", errorMsg)
   // → "所有可用账户均尝试失败: ..."
   ```

3. **第973行**：主循环显示
   ```go
   ui.UpdateStatus(statusIndex, fmt.Sprintf("下载失败: %s", errorMsg), red)
   // → "下载失败: 所有可用账户均尝试失败: ..."
   ```

## ✨ 优化方案

### 1. 原地刷新（不刷屏）

**实现**：
```go
// 截断过长错误信息
errorMsg := err.Error()
if len(errorMsg) > 50 {
    errorMsg = errorMsg[:47] + "..."
}

// 使用 ui.UpdateStatus 原地更新
ui.UpdateStatus(statusIndex, errorMsg, color)
```

**效果**：
- ✅ 错误信息限制在50字符内
- ✅ 原地刷新，不产生新行
- ✅ UI保持整洁，易于观看

### 2. 智能连接失败处理

**实现**：
```go
// 连接失败计数器
connectionRefusedCount := 0
const maxConnectionRefusedRetries = 3

// 检测连接错误
if strings.Contains(err.Error(), "connection refused") {
    connectionRefusedCount++
    updateStatus(statusIndex, 
        fmt.Sprintf("连接失败 %d/%d: 正在重试...", connectionRefusedCount, maxConnectionRefusedRetries), 
        yellow)
    
    // 超过最大重试次数，直接跳过
    if connectionRefusedCount >= maxConnectionRefusedRetries {
        updateStatus(statusIndex, "连接服务失败，已跳过", red)
        return "", fmt.Errorf("连接服务持续失败，已跳过此曲目")
    }
}
```

**特点**：
- ✅ 检测 `connection refused` 错误
- ✅ 最多重试3次
- ✅ 自动跳过，不浪费时间
- ✅ 跳过的曲目不计入错误统计

### 3. 优化重试提示

```go
// 非连接错误，显示简短提示
errorMsg := err.Error()
if len(errorMsg) > 40 {
    errorMsg = errorMsg[:37] + "..."
}
updateStatus(statusIndex, 
    fmt.Sprintf("重试 %d/%d: %s", attempt+1, maxRetries+1, errorMsg), 
    yellow)
```

### 4. 缩短等待时间

```go
// 重试间隔：2秒 → 1.5秒
time.Sleep(1500 * time.Millisecond)

// 切换账户：1秒 → 0.5秒
time.Sleep(500 * time.Millisecond)
```

## 📊 优化前后对比

### 优化前 ❌

```
Track 15: 下载失败: 所有可用账户均尝试失败: failed to run v14 with account CN: dial tcp 192.168.1.9:10020: connect: connection refused
Track 15: 账户 CN 失败, 尝试下一个...
Track 15: 下载失败: 所有可用账户均尝试失败: failed to run v14 with account CN: dial tcp 192.168.1.9:10020: connect: connection refused
Track 15: 账户 CN 失败, 尝试下一个...
[反复刷屏，浪费时间]
```

**问题**：
- ❌ 错误信息过长，换行
- ❌ 反复显示相同错误
- ❌ 滚屏严重，难以观看
- ❌ 浪费时间重试无效错误

### 优化后 ✅

```
Track 15: 连接失败 1/3: 正在重试...  [原地更新，不换行]
Track 15: 连接失败 2/3: 正在重试...  [原地更新]
Track 15: 连接失败 3/3: 正在重试...  [原地更新]
Track 15: 连接服务失败，已跳过      [自动跳过]
```

**优势**：
- ✅ 简洁明了，一目了然
- ✅ 原地刷新，不刷屏
- ✅ 智能跳过，节省时间
- ✅ 跳过不计入错误统计

## 🎯 各种错误的显示效果

### 1. 连接失败（connection refused）

```
Track 15: 连接失败 1/3: 正在重试...
Track 15: 连接失败 2/3: 正在重试...
Track 15: 连接失败 3/3: 正在重试...
Track 15: 连接服务失败，已跳过
```

### 2. 其他下载错误

```
Track 15: 重试 1/4: failed to extract info from...
Track 15: 重试 2/4: failed to extract info from...
Track 15: 账户 CN 失败，切换中...
Track 15: 重试 1/4: failed to extract info from...
Track 15: 下载失败: 所有账户失败: failed to extract...
```

### 3. 标签写入失败

```
Track 15: 重试 1/3: 标签写入失败: ilst box not present...
Track 15: 重试 2/3: 标签写入失败: ilst box not present...
Track 15: 重试 3/3: 标签写入失败: ilst box not present...
Track 15: 已跳过 (标签失败)
```

## 🔧 技术细节

### 错误信息截断

```go
// 方法1：简单截断
errorMsg := err.Error()
if len(errorMsg) > 50 {
    errorMsg = errorMsg[:47] + "..."
}

// 方法2：保留关键信息
// "所有可用账户均尝试失败: failed to run v14 with account CN: dial tcp ..."
// → "所有账户失败: dial tcp 192.168.1.9:10020..."
```

### 连接错误检测

```go
// 检测多种连接错误
if strings.Contains(err.Error(), "connection refused") ||
   strings.Contains(err.Error(), "connection reset") ||
   strings.Contains(err.Error(), "timeout") {
    // 智能处理
}
```

### 跳过逻辑

```go
// 检查是否是跳过类型的错误
if strings.Contains(err.Error(), "已跳过") {
    ui.UpdateStatus(statusIndex, errorMsg, yellow)
    // 跳过不计入错误统计
} else {
    ui.UpdateStatus(statusIndex, fmt.Sprintf("下载失败: %s", errorMsg), red)
    core.Counter.Error++
}
```

## 💡 使用建议

### 处理 connection refused

**问题原因**：解密服务未启动或配置错误

**解决方法**：
1. 检查 `config.yaml` 中的端口配置
   ```yaml
   decrypt-m3u8-port: "192.168.1.9:10020"
   get-m3u8-port: "192.168.1.9:10021"
   ```

2. 确认解密服务已启动
   ```bash
   # 检查端口是否监听
   nc -zv 192.168.1.9 10020
   ```

3. 如果服务在本机，使用 `127.0.0.1` 而非 `192.168.1.9`

### 减少错误

1. **网络稳定性**：
   - 使用有线网络
   - 检查防火墙设置
   - 避免高峰时段

2. **服务可用性**：
   - 确保解密服务正常运行
   - 定期检查服务状态
   - 配置服务自动重启

3. **账户有效性**：
   - 检查 media-user-token 是否过期
   - 确认账户有权限访问内容
   - 使用多个账户做备份

## 📋 相关文件

- **主要修改**: `/root/apple-music-downloader/internal/downloader/downloader.go`
  - 第95-154行：`downloadTrackWithFallback` - 智能重试和跳过逻辑
  - 第957-978行：主下载循环 - 错误信息截断和显示

- **相关模块**:
  - `runv14.Run`: M3U8 下载和解密
  - `ui.UpdateStatus`: UI 原地刷新
  - `core.Counter`: 错误统计

## 🎓 经验总结

1. **用户友好**：错误信息要简洁明了，不要技术细节堆砌
2. **智能处理**：识别常见错误，自动跳过而不是死循环
3. **原地刷新**：使用 UI 更新而不是打印新行，保持整洁
4. **性能优先**：缩短无效重试的等待时间，提升效率
5. **统计准确**：跳过的曲目不应计入错误，避免误导

---

**修复版本**: commit 273411c
**修复日期**: 2025-10-10
**影响范围**: 所有下载任务的错误处理和重试逻辑

