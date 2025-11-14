package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"

	"main/internal/api"
	"main/internal/core"
	"main/internal/downloader"
	"main/internal/parser"
	"github.com/spf13/pflag"
)

var jsonOutput bool

func printJSONError(message string) {
	type JsonError struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	errJSON, _ := json.Marshal(JsonError{
		Status:  "error",
		Message: message,
	})
	fmt.Println(string(errJSON))
}

func handleSingleMV(urlRaw string) {
	if core.Debug_mode {
		return
	}
	storefront, albumId := parser.CheckUrlMv(urlRaw)
	accountForMV, err := core.GetAccountForStorefront(storefront)
	if err != nil {
		if jsonOutput {
			printJSONError(fmt.Sprintf("MV 下载失败: %v", err))
		} else {
			fmt.Printf("MV 下载失败: %v\n", err)
		}
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
		if jsonOutput {
			printJSONError("MV 下载失败: media-user-token 无效")
		}
		return
	}
	if _, err := exec.LookPath("mp4decrypt"); err != nil {
		core.SharedLock.Lock()
		core.Counter.Error++
		core.SharedLock.Unlock()
		if jsonOutput {
			printJSONError("MV 下载失败: 未找到 mp4decrypt")
		}
		return
	}

	mvInfo, err := api.GetMVInfoFromAdam(albumId, accountForMV, storefront)
	if err != nil {
		errMsg := fmt.Sprintf("获取 MV 信息失败: %v", err)
		if jsonOutput {
			printJSONError(errMsg)
		} else {
			fmt.Println(errMsg)
		}
		core.SharedLock.Lock()
		core.Counter.Error++
		core.SharedLock.Unlock()
		return
	}

	if jsonOutput {
		type JsonStatus struct {
			Status    string `json:"status"`
			TrackNum  int    `json:"trackNum"`
			TrackName string `json:"trackName"`
			AlbumName string `json:"albumName"`
			AlbumID   string `json:"albumId"`
		}
		statusJSON, _ := json.Marshal(JsonStatus{
			Status:    "start",
			TrackNum:  1,
			TrackName: mvInfo.Data[0].Attributes.Name,
			AlbumName: mvInfo.Data[0].Attributes.Name,
			AlbumID:   albumId,
		})
		fmt.Println(string(statusJSON))
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

	_, err = downloader.MvDownloader(albumId, core.Config.AlacSaveFolder, sanitizedArtistFolder, "", storefront, nil, accountForMV, nil)

	if err != nil {
		core.SharedLock.Lock()
		core.Counter.Error++
		core.SharedLock.Unlock()
		if jsonOutput {
			printJSONError(fmt.Sprintf("MV 下载失败: %v", err))
		}
		return
	}
	core.SharedLock.Lock()
	core.Counter.Success++
	core.SharedLock.Unlock()

	if jsonOutput {
		type JsonStatusComplete struct {
			Status     string `json:"status"`
			TrackNum   int    `json:"trackNum"`
			TrackName  string `json:"trackName"`
			AlbumName  string `json:"albumName"`
			AlbumID    string `json:"albumId"`
			Percentage int    `json:"percentage"`
		}
		statusJSON, _ := json.Marshal(JsonStatusComplete{
			Status:     "complete",
			TrackNum:   1,
			TrackName:  mvInfo.Data[0].Attributes.Name,
			AlbumName:  mvInfo.Data[0].Attributes.Name,
			AlbumID:    albumId,
			Percentage: 100,
		})
		fmt.Println(string(statusJSON))
	}
}

func processURL(urlRaw string, wg *sync.WaitGroup, semaphore chan struct{}, currentTask int, totalTasks int) {
	if wg != nil {
		defer wg.Done()
	}
	if semaphore != nil {
		defer func() { <-semaphore }()
	}

	if totalTasks > 1 && !jsonOutput {
		fmt.Printf("[%d/%d] 开始处理: %s\n", currentTask, totalTasks, urlRaw)
	}

	var storefront, albumId string

	if strings.Contains(urlRaw, "/music-video/") {
		handleSingleMV(urlRaw)
		return
	}

	if strings.Contains(urlRaw, "/song/") {
		tempStorefront, _ := parser.CheckUrlSong(urlRaw)
		accountForSong, err := core.GetAccountForStorefront(tempStorefront)
		if err != nil {
			errMsg := fmt.Sprintf("获取歌曲信息失败 for %s: %v", urlRaw, err)
			if jsonOutput {
				printJSONError(errMsg)
			} else {
				fmt.Println(errMsg)
			}
			return
		}
		urlRaw, err = api.GetUrlSong(urlRaw, accountForSong)
		if err != nil {
			errMsg := fmt.Sprintf("获取歌曲链接失败 for %s: %v", urlRaw, err)
			if jsonOutput {
				printJSONError(errMsg)
			} else {
				fmt.Println(errMsg)
			}
			return
		}
		core.Dl_song = true
	}

	if strings.Contains(urlRaw, "/playlist/") {
		storefront, albumId = parser.CheckUrlPlaylist(urlRaw)
	} else {
		storefront, albumId = parser.CheckUrl(urlRaw)
	}

	if albumId == "" {
		errMsg := fmt.Sprintf("无效的URL: %s", urlRaw)
		if jsonOutput {
			printJSONError(errMsg)
		} else {
			fmt.Println(errMsg)
		}
		return
	}

	parse, err := url.Parse(urlRaw)
	if err != nil {
		errMsg := fmt.Sprintf("解析URL失败 %s: %v", urlRaw, err)
		if jsonOutput {
			printJSONError(errMsg)
		} else {
			log.Println(errMsg)
		}
		return
	}
	var urlArg_i = parse.Query().Get("i")
	err = downloader.Rip(albumId, storefront, urlArg_i, urlRaw, jsonOutput)

	if err != nil {
		errMsg := fmt.Sprintf("专辑下载失败: %s -> %v", urlRaw, err)
		if jsonOutput {
			printJSONError(errMsg)
		} else {
			fmt.Println(errMsg)
		}
	} else {
		if totalTasks > 1 && !jsonOutput {
			fmt.Printf("[%d/%d] 任务完成: %s\n", currentTask, totalTasks, urlRaw)
		}
	}
}

func runDownloads(initialUrls []string, isBatch bool) {
	var finalUrls []string

	for _, urlRaw := range initialUrls {
		if strings.Contains(urlRaw, "/artist/") {
			if !jsonOutput {
				fmt.Printf("正在解析歌手页面: %s\n", urlRaw)
			}
			artistAccount := &core.Config.Accounts[0]
			urlArtistName, urlArtistID, err := api.GetUrlArtistName(urlRaw, artistAccount)
			if err != nil {
				if !jsonOutput {
					fmt.Printf("获取歌手名称失败 for %s: %v\n", urlRaw, err)
				}
				continue
			}

			core.Config.ArtistFolderFormat = strings.NewReplacer(
				"{UrlArtistName}", core.LimitString(urlArtistName),
				"{ArtistId}", urlArtistID,
			).Replace(core.Config.ArtistFolderFormat)

			albumArgs, err := api.CheckArtist(urlRaw, artistAccount, "albums")
			if err != nil {
				if !jsonOutput {
					fmt.Printf("获取歌手专辑失败 for %s: %v\n", urlRaw, err)
				}
			} else {
				finalUrls = append(finalUrls, albumArgs...)
				if !jsonOutput {
					fmt.Printf("从歌手 %s 页面添加了 %d 张专辑到队列。\n", urlArtistName, len(albumArgs))
				}
			}

			mvArgs, err := api.CheckArtist(urlRaw, artistAccount, "music-videos")
			if err != nil {
				if !jsonOutput {
					fmt.Printf("获取歌手MV失败 for %s: %v\n", urlRaw, err)
				}
			} else {
				finalUrls = append(finalUrls, mvArgs...)
				if !jsonOutput {
					fmt.Printf("从歌手 %s 页面添加了 %d 个MV到队列。\n", urlArtistName, len(mvArgs))
				}
			}
		} else {
			finalUrls = append(finalUrls, urlRaw)
		}
	}

	if len(finalUrls) == 0 {
		if !jsonOutput {
			fmt.Println("队列中没有有效的链接可供下载。")
		}
		return
	}

	numThreads := 1
	if isBatch && core.Config.TxtDownloadThreads > 1 {
		numThreads = core.Config.TxtDownloadThreads
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, numThreads)
	totalTasks := len(finalUrls)

	if !jsonOutput {
		fmt.Printf("--- 开始下载任务 ---\n总数: %d, 并发数: %d\n--------------------\n", totalTasks, numThreads)
	}

	for i, urlToProcess := range finalUrls {
		wg.Add(1)
		semaphore <- struct{}{}
		go processURL(urlToProcess, &wg, semaphore, i+1, totalTasks)
	}

	wg.Wait()
}

func main() {
	core.InitFlags()
	pflag.BoolVar(&jsonOutput, "json-output", false, "启用JSON输出 (供给桌面App使用)")

	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "用法: %s [选项] [url1 url2 ...]\n", os.Args[0])
		fmt.Println("如果没有提供URL，程序将进入交互模式。")
		fmt.Println("选项:")
		pflag.PrintDefaults()
	}

	pflag.Parse()

	err := core.LoadConfig(core.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) && core.ConfigPath == "config.yaml" {
			errMsg := "错误: 默认配置文件 config.yaml 未找到。"
			if jsonOutput {
				printJSONError(errMsg)
			} else {
				fmt.Println(errMsg)
				pflag.Usage()
			}
			return
		}
		errMsg := fmt.Sprintf("加载配置文件 %s 失败: %v", core.ConfigPath, err)
		if jsonOutput {
			printJSONError(errMsg)
		} else {
			fmt.Println(errMsg)
		}
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
			errMsg := "获取开发者 token 失败。"
			if jsonOutput {
				printJSONError(errMsg)
			} else {
				fmt.Println(errMsg)
			}
			return
		}
	}
	core.DeveloperToken = token

	args := pflag.Args()
	if len(args) == 0 {
		if jsonOutput {
			printJSONError("JSON 模式下不支持交互式输入")
			return
		}

		fmt.Print("请输入专辑链接或txt文件路径: ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			fmt.Println("未输入内容，程序退出。")
			return
		}

		if strings.HasSuffix(strings.ToLower(input), ".txt") {
			if _, err := os.Stat(input); err == nil {
				fileBytes, err := os.ReadFile(input)
				if err != nil {
					fmt.Printf("读取文件 %s 失败: %v\n", input, err)
					return
				}
				lines := strings.Split(string(fileBytes), "\n")
				var urls []string
				for _, line := range lines {
					trimmedLine := strings.TrimSpace(line)
					if trimmedLine != "" {
						urls = append(urls, trimmedLine)
					}
				}
				runDownloads(urls, true)
			} else {
				fmt.Printf("错误: 文件不存在 %s\n", input)
				return
			}
		} else {
			runDownloads([]string{input}, false)
		}
	} else {
		runDownloads(args, false)
	}

	if !jsonOutput {
		fmt.Printf("\n=======  [✔ ] Completed: %d/%d  |  [⚠ ] Warnings: %d  |  [✖ ] Errors: %d  =======\n", core.Counter.Success, core.Counter.Total, core.Counter.Unavailable+core.Counter.NotSong, core.Counter.Error)
		if core.Counter.Error > 0 {
			fmt.Println("部分任务在执行过程中出错，请检查上面的日志记录")
		}
	}
}
