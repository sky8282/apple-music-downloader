package main

import (
	"bufio"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"main/internal/api"
	"main/internal/core"
	"main/internal/downloader"
	"main/internal/history"
	"main/internal/parser"

	"github.com/fatih/color"
	"github.com/spf13/pflag"
)

// 版本信息（编译时通过 ldflags 注入）
var (
	Version   = "dev"     // 版本号
	BuildTime = "unknown" // 编译时间
	GitCommit = "unknown" // Git提交哈希
)

func handleSingleMV(urlRaw string) {
	if core.Debug_mode {
		return
	}
	storefront, albumId := parser.CheckUrlMv(urlRaw)
	accountForMV, err := core.GetAccountForStorefront(storefront)
	if err != nil {
		fmt.Printf("MV 下载失败: %v\n", err)
		core.SharedLock.Lock()
		core.Counter.Error++
		core.SharedLock.Unlock()
		return
	}

	core.SharedLock.Lock()
	core.Counter.Total++
	core.SharedLock.Unlock()
	if len(accountForMV.MediaUserToken) <= 50 {
		core.SharedLock.Lock()
		core.Counter.Error++
		core.SharedLock.Unlock()
		return
	}
	if _, err := exec.LookPath("mp4decrypt"); err != nil {
		core.SharedLock.Lock()
		core.Counter.Error++
		core.SharedLock.Unlock()
		return
	}

	mvInfo, err := api.GetMVInfoFromAdam(albumId, accountForMV, storefront)
	if err != nil {
		fmt.Printf("获取 MV 信息失败: %v\n", err)
		core.SharedLock.Lock()
		core.Counter.Error++
		core.SharedLock.Unlock()
		return
	}

	// 输出MV信息
	core.SafePrintf("🎤 歌手: %s\n", mvInfo.Data[0].Attributes.ArtistName)
	core.SafePrintf("🎬 MV: %s\n", mvInfo.Data[0].Attributes.Name)

	// 提取发行年份
	var releaseYear string
	if len(mvInfo.Data[0].Attributes.ReleaseDate) >= 4 {
		releaseYear = mvInfo.Data[0].Attributes.ReleaseDate[:4]
		core.SafePrintf("📅 发行年份: %s\n", releaseYear)
	}

	var artistFolder string
	if core.Config.ArtistFolderFormat != "" {
		artistFolder = strings.NewReplacer(
			"{UrlArtistName}", core.LimitString(mvInfo.Data[0].Attributes.ArtistName),
			"{ArtistName}", core.LimitString(mvInfo.Data[0].Attributes.ArtistName),
			"{ArtistId}", "",
		).Replace(core.Config.ArtistFolderFormat)
	}
	sanitizedArtistFolder := core.ForbiddenNames.ReplaceAllString(artistFolder, "_")

	// Use MVSaveFolder if configured, otherwise fallback to AlacSaveFolder
	mvSaveFolder := core.Config.MVSaveFolder
	if mvSaveFolder == "" {
		mvSaveFolder = core.Config.AlacSaveFolder
	}

	// 应用缓存机制
	cachePath, finalPath, usingCache := downloader.GetCacheBasePath(mvSaveFolder, albumId)

	mvOutPath, mvResolution, err := downloader.MvDownloader(albumId, cachePath, sanitizedArtistFolder, "", storefront, nil, accountForMV)

	// 分辨率信息已在 MvDownloader 内部显示，这里不再重复显示
	_ = mvResolution

	// 如果使用缓存且下载成功，移动文件到最终位置
	if err == nil && usingCache && mvOutPath != "" {
		// 计算最终路径
		relPath, _ := filepath.Rel(cachePath, mvOutPath)
		finalMvPath := filepath.Join(finalPath, relPath)

		// 移动文件
		core.SafePrintf("\n📤 正在从缓存转移MV文件到目标位置...\n")
		if moveErr := downloader.SafeMoveFile(mvOutPath, finalMvPath); moveErr != nil {
			fmt.Printf("从缓存移动MV文件失败: %v\n", moveErr)
			err = moveErr
		} else {
			core.SafePrintf("📥 MV文件转移完成！\n")
			core.SafePrintf("💾 保存路径: %s\n", finalMvPath)

			// 清理缓存目录
			mvCacheDir := filepath.Dir(mvOutPath)
			for mvCacheDir != cachePath && mvCacheDir != "." && mvCacheDir != "/" {
				if os.Remove(mvCacheDir) != nil {
					break
				}
				mvCacheDir = filepath.Dir(mvCacheDir)
			}
		}
	} else if err == nil && !usingCache && mvOutPath != "" {
		// 未使用缓存，直接保存
		core.SafePrintf("\n📥 MV下载完成！\n")
		core.SafePrintf("💾 保存路径: %s\n", mvOutPath)
	}

	// 如果出错且使用了缓存，清理缓存
	if err != nil && usingCache {
		os.RemoveAll(cachePath)
	}

	if err != nil {
		core.SharedLock.Lock()
		core.Counter.Error++
		core.SharedLock.Unlock()
		return
	}
	core.SharedLock.Lock()
	core.Counter.Success++
	core.SharedLock.Unlock()
}

func processURL(urlRaw string, wg *sync.WaitGroup, semaphore chan struct{}, currentTask int, totalTasks int) (string, string, error) {
	if wg != nil {
		defer wg.Done()
	}
	if semaphore != nil {
		defer func() { <-semaphore }()
	}

	if totalTasks > 1 {
		core.SafePrintf("🧾 [%d/%d] 开始处理: %s\n", currentTask, totalTasks, urlRaw)
	}

	var storefront, albumId string
	var albumName string
	_ = albumName // 用于历史记录

	if strings.Contains(urlRaw, "/music-video/") {
		handleSingleMV(urlRaw)
		return "", "", nil
	}

	if strings.Contains(urlRaw, "/song/") {
		tempStorefront, _ := parser.CheckUrlSong(urlRaw)
		accountForSong, err := core.GetAccountForStorefront(tempStorefront)
		if err != nil {
			fmt.Printf("获取歌曲信息失败 for %s: %v\n", urlRaw, err)
			return "", "", err
		}
		urlRaw, err = api.GetUrlSong(urlRaw, accountForSong)
		if err != nil {
			fmt.Printf("获取歌曲链接失败 for %s: %v\n", urlRaw, err)
			return "", "", err
		}
		core.Dl_song = true
	}

	if strings.Contains(urlRaw, "/playlist/") {
		storefront, albumId = parser.CheckUrlPlaylist(urlRaw)
	} else {
		storefront, albumId = parser.CheckUrl(urlRaw)
	}

	if albumId == "" {
		err := fmt.Errorf("无效的URL")
		fmt.Printf("无效的URL: %s\n", urlRaw)
		return "", "", err
	}

	// 获取专辑信息用于历史记录
	mainAccount, err := core.GetAccountForStorefront(storefront)
	if err == nil {
		meta, err := api.GetMeta(albumId, mainAccount, storefront)
		if err == nil && len(meta.Data) > 0 {
			albumName = meta.Data[0].Attributes.Name
		}
	}

	parse, err := url.Parse(urlRaw)
	if err != nil {
		log.Printf("解析URL失败 %s: %v", urlRaw, err)
		return albumId, albumName, err
	}
	var urlArg_i = parse.Query().Get("i")
	err = downloader.Rip(albumId, storefront, urlArg_i, urlRaw)
	if err != nil {
		core.SafePrintf("专辑下载失败: %s -> %v\n", urlRaw, err)
		return albumId, albumName, err
	} else {
		if totalTasks > 1 {
			core.SafePrintf("✅ [%d/%d] 任务完成: %s\n", currentTask, totalTasks, urlRaw)
		}
		return albumId, albumName, nil
	}
}

// parseTxtFile 从TXT文件中解析URL列表
func parseTxtFile(filePath string) ([]string, error) {
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %v", err)
	}

	lines := strings.Split(string(fileBytes), "\n")
	var urls []string
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		// 跳过空行和注释行（以#开头）
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}
		// 支持一行多个链接（空格分隔）
		linksInLine := strings.Fields(trimmedLine)
		for _, link := range linksInLine {
			link = strings.TrimSpace(link)
			if link != "" {
				urls = append(urls, link)
			}
		}
	}
	return urls, nil
}

func runDownloads(initialUrls []string, isBatch bool, taskFile string) {
	var finalUrls []string

	// 显示输入链接统计
	if isBatch && len(initialUrls) > 0 {
		core.SafePrintf("📋 初始链接总数: %d\n", len(initialUrls))
		core.SafePrintf("🔄 开始预处理链接...\n\n")
	}

	for _, urlRaw := range initialUrls {
		if strings.Contains(urlRaw, "/artist/") {
			core.SafePrintf("🔍 正在解析歌手页面: %s\n", urlRaw)
			artistAccount := &core.Config.Accounts[0]
			urlArtistName, urlArtistID, err := api.GetUrlArtistName(urlRaw, artistAccount)
			if err != nil {
				core.SafePrintf("获取歌手名称失败 for %s: %v\n", urlRaw, err)
				continue
			}

			core.Config.ArtistFolderFormat = strings.NewReplacer(
				"{UrlArtistName}", core.LimitString(urlArtistName),
				"{ArtistId}", urlArtistID,
			).Replace(core.Config.ArtistFolderFormat)

			albumArgs, err := api.CheckArtist(urlRaw, artistAccount, "albums")
			if err != nil {
				core.SafePrintf("获取歌手专辑失败 for %s: %v\n", urlRaw, err)
			} else {
				finalUrls = append(finalUrls, albumArgs...)
				core.SafePrintf("📀 从歌手 %s 页面添加了 %d 张专辑到队列。\n", urlArtistName, len(albumArgs))
			}

			mvArgs, err := api.CheckArtist(urlRaw, artistAccount, "music-videos")
			if err != nil {
				core.SafePrintf("获取歌手MV失败 for %s: %v\n", urlRaw, err)
			} else {
				finalUrls = append(finalUrls, mvArgs...)
				core.SafePrintf("🎬 从歌手 %s 页面添加了 %d 个MV到队列。\n", urlArtistName, len(mvArgs))
			}
		} else {
			finalUrls = append(finalUrls, urlRaw)
		}
	}

	if len(finalUrls) == 0 {
		fmt.Println("队列中没有有效的链接可供下载。")
		return
	}

	totalTasks := len(finalUrls)

	// 初始化历史记录系统
	var task *history.TaskHistory
	var completedURLs map[string]bool
	if isBatch && taskFile != "" {
		// 初始化历史记录目录
		if err := history.InitHistory(); err != nil {
			core.SafePrintf("⚠️  初始化历史记录失败: %v\n", err)
		}

		// 检查历史记录，获取已完成的URL
		var err error
		completedURLs, err = history.GetCompletedURLs(taskFile)
		if err != nil {
			core.SafePrintf("⚠️  读取历史记录失败: %v\n", err)
			completedURLs = make(map[string]bool)
		}

		// 过滤已完成的URL
		skippedCount := 0
		var remainingUrls []string
		for _, url := range finalUrls {
			if completedURLs[url] {
				skippedCount++
			} else {
				remainingUrls = append(remainingUrls, url)
			}
		}

		if skippedCount > 0 {
			core.SafePrintf("📜 历史记录检测: 发现 %d 个已完成的任务\n", skippedCount)
			core.SafePrintf("⏭️  已自动跳过，剩余 %d 个任务\n\n", len(remainingUrls))
			finalUrls = remainingUrls
			totalTasks = len(finalUrls)

			if totalTasks == 0 {
				core.SafePrintf("✅ 所有任务都已完成，无需重复下载！\n")
				return
			}
		}

		// 创建新任务
		task, err = history.NewTask(taskFile, totalTasks)
		if err != nil {
			core.SafePrintf("⚠️  创建任务记录失败: %v\n", err)
		}
	}

	if isBatch {
		core.SafePrintf("\n📋 ========== 开始下载任务 ==========\n")
		if len(initialUrls) != totalTasks {
			core.SafePrintf("📝 预处理完成: %d 个链接 → %d 个任务\n", len(initialUrls), totalTasks)
		} else {
			core.SafePrintf("📝 任务总数: %d\n", totalTasks)
		}
		core.SafePrintf("⚡ 执行模式: 串行模式 \n")
		core.SafePrintf("📦 专辑内并发: 由配置文件控制\n")
		if task != nil {
			core.SafePrintf("📜 历史记录: 已启用\n")
		}
		core.SafePrintf("====================================\n\n")
	} else {
		core.SafePrintf("📋 开始下载任务\n📝 总数: %d\n--------------------\n", totalTasks)
	}

	// 批量模式：串行执行（按链接顺序依次下载）
	// 专辑内歌曲并发数由配置文件控制 (lossless_downloadthreads 等)
	for i, urlToProcess := range finalUrls {
		albumId, albumName, err := processURL(urlToProcess, nil, nil, i+1, totalTasks)

		// 记录到历史
		if task != nil && albumId != "" {
			status := "success"
			errorMsg := ""
			if err != nil {
				status = "failed"
				errorMsg = err.Error()
			}

			history.AddRecord(history.DownloadRecord{
				URL:        urlToProcess,
				AlbumID:    albumId,
				AlbumName:  albumName,
				Status:     status,
				DownloadAt: time.Now(),
				ErrorMsg:   errorMsg,
			})
		}

		// 任务之间添加视觉间隔（最后一个任务不需要）
		if isBatch && i < len(finalUrls)-1 {
			core.SafePrintf("\n%s\n\n", strings.Repeat("=", 80))
		}
	}

	// 保存历史记录
	if task != nil {
		if err := history.SaveTask(); err != nil {
			core.SafePrintf("⚠️  保存历史记录失败: %v\n", err)
		} else {
			core.SafePrintf("\n📜 历史记录已保存至: history/%s.json\n", task.TaskID)
		}
	}
}

func main() {
	// 打印版本信息
	cyan := color.New(color.FgCyan, color.Bold)
	yellow := color.New(color.FgYellow)
	fmt.Println(strings.Repeat("=", 80))
	cyan.Printf("🎵 Apple Music Downloader %s\n", Version)
	yellow.Printf("📅 编译时间: %s\n", BuildTime)
	if GitCommit != "unknown" {
		yellow.Printf("🔖 Git提交: %s\n", GitCommit)
	}
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	core.InitFlags()

	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "用法: %s [选项] [url1 url2 ... | file.txt ...]\n", os.Args[0])
		fmt.Println("如果没有提供URL或文件，程序将进入交互模式。")
		fmt.Println()
		fmt.Println("支持的启动方式:")
		fmt.Println("  1. 交互模式: 运行程序后输入链接或TXT文件路径")
		fmt.Println("  2. 单链接模式: ./程序名 <url>")
		fmt.Println("  3. 多链接模式: ./程序名 <url1> <url2> ...")
		fmt.Println("  4. TXT文件模式: ./程序名 <file.txt>")
		fmt.Println("  5. 混合模式: ./程序名 <url1> <file.txt> <url2> ...")
		fmt.Println()
		fmt.Println("TXT文件格式:")
		fmt.Println("  - 支持单行单链接（传统格式）")
		fmt.Println("  - 支持单行多链接（空格分隔）")
		fmt.Println("  - 支持注释行（以#开头）")
		fmt.Println("  - 空行会被自动跳过")
		fmt.Println()
		fmt.Println("选项:")
		pflag.PrintDefaults()
	}

	pflag.Parse()

	err := core.LoadConfig(core.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) && core.ConfigPath == "config.yaml" {
			fmt.Println("错误: 默认配置文件 config.yaml 未找到。")
			pflag.Usage()
			return
		}
		fmt.Printf("加载配置文件 %s 失败: %v\n", core.ConfigPath, err)
		return
	}

	if core.OutputPath != "" {
		core.Config.AlacSaveFolder = core.OutputPath
		core.Config.AtmosSaveFolder = core.OutputPath
	}

	token, err := api.GetToken()
	if err != nil {
		if len(core.Config.Accounts) > 0 && core.Config.Accounts[0].AuthorizationToken != "" && core.Config.Accounts[0].AuthorizationToken != "your-authorization-token" {
			token = strings.Replace(core.Config.Accounts[0].AuthorizationToken, "Bearer ", "", -1)
		} else {
			fmt.Println("获取开发者 token 失败。")
			return
		}
	}
	core.DeveloperToken = token

	args := pflag.Args()
	if len(args) == 0 {
		fmt.Print("请输入专辑链接或TXT文件路径: ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			fmt.Println("未输入内容，程序退出。")
			return
		}

		if strings.HasSuffix(strings.ToLower(input), ".txt") {
			if _, err := os.Stat(input); err == nil {
				urls, err := parseTxtFile(input)
				if err != nil {
					fmt.Printf("读取文件 %s 失败: %v\n", input, err)
					return
				}
				fmt.Printf("📊 从文件 %s 中解析到 %d 个链接\n\n", input, len(urls))
				runDownloads(urls, true, input)
			} else {
				fmt.Printf("错误: 文件不存在 %s\n", input)
				return
			}
		} else {
			runDownloads([]string{input}, false, "")
		}
	} else {
		// 处理命令行参数：支持TXT文件或直接的URL列表
		var urls []string
		isBatch := false
		var taskFile string

		for _, arg := range args {
			if strings.HasSuffix(strings.ToLower(arg), ".txt") {
				// 参数是TXT文件
				if _, err := os.Stat(arg); err == nil {
					fileUrls, err := parseTxtFile(arg)
					if err != nil {
						fmt.Printf("读取文件 %s 失败: %v\n", arg, err)
						continue
					}
					fmt.Printf("📊 从文件 %s 中解析到 %d 个链接\n", arg, len(fileUrls))
					urls = append(urls, fileUrls...)
					isBatch = true
					// 记录第一个txt文件作为任务文件
					if taskFile == "" {
						taskFile = arg
					}
				} else {
					fmt.Printf("错误: 文件不存在 %s\n", arg)
				}
			} else {
				// 参数是URL
				urls = append(urls, arg)
			}
		}

		if len(urls) > 1 {
			isBatch = true
		}

		if len(urls) > 0 {
			if isBatch {
				fmt.Println()
			}
			runDownloads(urls, isBatch, taskFile)
		} else {
			fmt.Println("没有有效的链接可供处理。")
		}
	}

	fmt.Printf("\n📦 已完成: %d/%d | 警告: %d | 错误: %d\n", core.Counter.Success, core.Counter.Total, core.Counter.Unavailable+core.Counter.NotSong, core.Counter.Error)
	if core.Counter.Error > 0 {
		fmt.Println("部分任务在执行过程中出错，请检查上面的日志记录。")
	}
}
