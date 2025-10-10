package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"main/internal/core"
)

// EnsureSafePath truncates path components to ensure the total path length does not exceed the limit
func EnsureSafePath(basePath, artistDir, albumDir, fileName string) (string, string, string) {
	truncate := func(s string, n int) string {
		if n <= 0 {
			return s
		}
		runes := []rune(s)
		if len(runes) <= n {
			return ""
		}
		return string(runes[:len(runes)-n])
	}

	for {
		currentPath := filepath.Join(basePath, artistDir, albumDir, fileName)
		if len(currentPath) <= core.MaxPathLength {
			break
		}

		overage := len(currentPath) - core.MaxPathLength
		ext := filepath.Ext(fileName)
		stem := strings.TrimSuffix(fileName, ext)

		var prefixPart string
		var namePart string
		re := regexp.MustCompile(`^(\d+[\s.-]*)`)
		matches := re.FindStringSubmatch(stem)

		if len(matches) > 1 {
			prefixPart = matches[1]
			namePart = strings.TrimPrefix(stem, prefixPart)
		} else {
			prefixPart = ""
			namePart = stem
		}

		if len(namePart) > 0 {
			canShorten := len(namePart)
			shortenAmount := overage
			if shortenAmount > canShorten {
				shortenAmount = canShorten
			}
			namePart = truncate(namePart, shortenAmount)

			if namePart == "" {
				prefixPart = strings.TrimRight(prefixPart, " .-")
			}

			fileName = prefixPart + namePart + ext
			continue
		}

		if len(albumDir) > 1 { // 至少保留一个字符
			canShorten := len(albumDir)
			shortenAmount := overage
			if shortenAmount > canShorten {
				shortenAmount = canShorten
			}
			albumDir = truncate(albumDir, shortenAmount)
			continue
		}

		if len(artistDir) > 1 { // 至少保留一个字符
			canShorten := len(artistDir)
			shortenAmount := overage
			if shortenAmount > canShorten {
				shortenAmount = canShorten
			}
			artistDir = truncate(artistDir, shortenAmount)
			continue
		}

		break
	}

	return artistDir, albumDir, fileName
}

// IsInArray checks if a target integer is in an array of integers
func IsInArray(arr []int, target int) bool {
	for _, num := range arr {
		if num == target {
			return true
		}
	}
	return false
}

// FileExists checks if a file exists at the given path
func FileExists(path string) (bool, error) {
	f, err := os.Stat(path)
	if err == nil {
		return !f.IsDir(), nil
	} else if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// FormatSpeed formats a download speed from bytes/sec to a human-readable string
func FormatSpeed(bytesPerSecond float64) string {
	if bytesPerSecond < 1024 {
		return fmt.Sprintf("%.1f B/s", bytesPerSecond)
	}
	kbps := bytesPerSecond / 1024
	if kbps < 1024 {
		return fmt.Sprintf("%.1f KB/s", kbps)
	}
	mbps := kbps / 1024
	return fmt.Sprintf("%.1f MB/s", mbps)
}

// Contains checks if a string slice contains a specific item
func Contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

// FormatQualityTag 统一质量标签格式为首字母大写、其余小写
// 按照报告规范处理 Dolby Atmos, Hi-Res Lossless, Alac, Aac 256 等标签
func FormatQualityTag(tag string) string {
	// 特殊处理：Dolby Atmos 保持两个单词首字母大写
	if tag == "Dolby Atmos" || tag == "DOLBY ATMOS" || tag == "dolby atmos" {
		return "Dolby Atmos"
	}

	// 特殊处理：Hi-Res Lossless 保持格式
	if tag == "Hi-Res Lossless" || tag == "HI-RES LOSSLESS" || tag == "hi-res lossless" {
		return "Hi-Res Lossless"
	}

	// 特殊处理：AAC 256 转换为 Aac 256
	if tag == "AAC 256" || tag == "aac 256" {
		return "Aac 256"
	}

	// ALAC -> Alac
	if tag == "ALAC" || tag == "alac" {
		return "Alac"
	}

	// 其他情况：保持原样或进行首字母大写处理
	if len(tag) == 0 {
		return tag
	}

	// 如果已经是合理的格式，直接返回
	return tag
}

// SafeMoveFile 安全地移动文件从源路径到目标路径
// 如果是跨文件系统操作，会使用拷贝+删除的方式
// 如果目标文件已存在，会跳过移动并返回特殊错误
func SafeMoveFile(src, dst string) error {
	// 检查目标文件是否已存在
	targetExists, _ := FileExists(dst)
	if targetExists {
		// 目标文件已存在，跳过移动（不覆盖）
		return fmt.Errorf("目标文件已存在，跳过")
	}

	// 确保目标目录存在
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, os.ModePerm); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 首先尝试直接重命名（最快的方式）
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// 如果重命名失败（可能是跨文件系统），使用拷贝+删除
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer dstFile.Close()

	// 拷贝文件内容
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		dstFile.Close()
		os.Remove(dst) // 清理不完整的目标文件
		return fmt.Errorf("拷贝文件内容失败: %w", err)
	}

	// 确保数据写入磁盘
	if err := dstFile.Sync(); err != nil {
		dstFile.Close()
		os.Remove(dst)
		return fmt.Errorf("同步文件失败: %w", err)
	}

	dstFile.Close()
	srcFile.Close()

	// 拷贝文件权限
	srcInfo, err := os.Stat(src)
	if err == nil {
		os.Chmod(dst, srcInfo.Mode())
	}

	// 删除源文件
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("删除源文件失败: %w", err)
	}

	return nil
}

// SafeMoveDirectory 安全地移动整个目录（递归）
func SafeMoveDirectory(src, dst string) error {
	// 首先尝试直接重命名整个目录
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// 如果直接重命名失败，递归移动文件
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 计算相对路径
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			// 创建目标目录
			return os.MkdirAll(targetPath, info.Mode())
		}

		// 移动文件
		return SafeMoveFile(path, targetPath)
	})
}

// CleanupCacheDirectory 清理缓存目录（在出错时调用）
func CleanupCacheDirectory(cachePath string) error {
	if cachePath == "" {
		return nil
	}

	// 安全检查：确保不会删除重要目录
	if cachePath == "/" || cachePath == "." || cachePath == ".." {
		return fmt.Errorf("拒绝删除危险路径: %s", cachePath)
	}

	return os.RemoveAll(cachePath)
}
