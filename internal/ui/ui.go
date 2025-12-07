package ui

import (
	"bufio"
	"errors"
	"fmt"
	"main/internal/core"
	"main/internal/utils"
	"main/utils/runv14"
	"main/utils/structs"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

type ProgressUI struct {
	p              *mpb.Progress
	wg             *sync.WaitGroup
	bars           map[int]*barState
	completedCount int
	totalTracks    int
	mu             sync.Mutex
}

type barState struct {
	bar        *mpb.Bar
	trackName  string
	prefix     string
	qualityStr string
	stateTxt   string
	speedStr   string
	account    string
	isDecrypt  bool
	isDone     bool
	statusMu   sync.Mutex
}

func NewProgressUI(wg *sync.WaitGroup) *ProgressUI {
	var p *mpb.Progress
	if wg != nil {
		p = mpb.New(mpb.WithWaitGroup(wg), mpb.WithOutput(os.Stdout), mpb.WithWidth(60))
	} else {
		p = mpb.New(mpb.WithOutput(os.Stdout), mpb.WithWidth(60))
	}
	return &ProgressUI{
		p:    p,
		wg:   wg,
		bars: make(map[int]*barState),
	}
}

func truncateString(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	r := []rune(s)
	return string(r[:maxLen-3]) + "..."
}

func stripAnsi(str string) string {
	const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"
	var re = regexp.MustCompile(ansi)
	return re.ReplaceAllString(str, "")
}

func (pui *ProgressUI) AddTrack(trackIndex, totalTracks int, trackName, qualityStr string) {
	pui.mu.Lock()
	defer pui.mu.Unlock()

	if pui.totalTracks == 0 {
		pui.totalTracks = totalTracks
	}

	if _, exists := pui.bars[trackIndex]; exists {
		return
	}

	shortName := truncateString(trackName, 25)
	prefix := fmt.Sprintf("Track %02d/%02d: %s", trackIndex, totalTracks, shortName)

	bs := &barState{
		trackName:  shortName,
		prefix:     prefix,
		qualityStr: qualityStr,
		stateTxt:   "准备中",
		speedStr:   "- MB/s",
		account:    "",
		isDone:     false,
	}

	red := color.New(color.FgRed).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	bar, _ := pui.p.Add(100,
		nil,
		mpb.PrependDecorators(
			decor.Any(func(s decor.Statistics) string {
				bs.statusMu.Lock()
				defer bs.statusMu.Unlock()
				return bs.prefix
			}),
			decor.Any(func(s decor.Statistics) string {
				bs.statusMu.Lock()
				defer bs.statusMu.Unlock()
				return " " + bs.qualityStr
			}, decor.WC{W: len(qualityStr) + 1}),
		),
		mpb.AppendDecorators(
			decor.Any(func(s decor.Statistics) string {
				bs.statusMu.Lock()
				defer bs.statusMu.Unlock()

				speed := bs.speedStr
				if speed == "" {
					speed = "- MB/s"
				}

				p := int64(0)
				if s.Total > 0 {
					p = s.Current * 100 / s.Total
				}

				state := bs.stateTxt
				acc := bs.account

				var statusStr string
				fullState := state
				if acc != "" {
					fullState = fmt.Sprintf("%s %s", acc, state)
				}

				if strings.Contains(state, "元数据") {
					statusStr = cyan(fullState)
				} else if strings.Contains(state, "等待解密") {
					statusStr = yellow(fullState)
				} else if strings.Contains(state, "完成") || strings.Contains(state, "已存在") {
					statusStr = green(fullState)
				} else if bs.isDecrypt || strings.Contains(state, "解密") || strings.Contains(state, "封装") || strings.Contains(state, "处理") || strings.Contains(state, "失败") || strings.Contains(state, "写入") {
					statusStr = red(fullState)
				} else {
					statusStr = yellow(fullState)
				}

				if strings.Contains(state, "元数据") || strings.Contains(state, "等待解密") || strings.Contains(state, "完成") || strings.Contains(state, "已存在") || strings.Contains(state, "失败") {
					return fmt.Sprintf(" [%s]", statusStr)
				}
				if p >= 100 || strings.Contains(state, "写入") || strings.Contains(state, "封装") || strings.Contains(state, "处理") {
					return fmt.Sprintf(" %3d %%       [%s]", p, statusStr)
				}
				if strings.Contains(state, "解密") {
					return fmt.Sprintf(" %3d %% [%s] [%s]", p, speed, statusStr)
				}

				return fmt.Sprintf(" %3d %% [%s] [%s]", p, speed, statusStr)
			}),
		),
	)

	bs.bar = bar
	pui.bars[trackIndex] = bs
}

func (pui *ProgressUI) UpdateStatus(trackIndex int, newStatus string) {
	pui.mu.Lock()
	bs, ok := pui.bars[trackIndex]
	pui.mu.Unlock()

	if ok {
		bs.statusMu.Lock()
		defer bs.statusMu.Unlock()
		if bs.isDone {
			return
		}
		bs.stateTxt = newStatus
		if strings.Contains(newStatus, "解密") || strings.Contains(newStatus, "封装") || strings.Contains(newStatus, "处理") || strings.Contains(newStatus, "写入") {
			bs.isDecrypt = true
		}
	}
}

func (pui *ProgressUI) UpdateProgress(trackIndex int, percentage int, speedBPS float64) {
	pui.mu.Lock()
	bs, ok := pui.bars[trackIndex]
	pui.mu.Unlock()

	if ok {
		bs.statusMu.Lock()
		if bs.isDone {
			bs.statusMu.Unlock()
			return
		}
		bs.speedStr = utils.FormatSpeed(speedBPS)
		bs.statusMu.Unlock()

		bs.bar.SetCurrent(int64(percentage))
	}
}

func (pui *ProgressUI) HandleProgress(trackIndex int, progressChan chan runv14.ProgressUpdate, accountName string) {
	pui.mu.Lock()
	bs, ok := pui.bars[trackIndex]
	pui.mu.Unlock()

	if !ok {
		go func() { for range progressChan {} }()
		return
	}

	go func() {
		var hasStartedDecrypting bool = false

		for p := range progressChan {
			bs.statusMu.Lock()
			if bs.isDone {
				bs.statusMu.Unlock()
				return
			}
			bs.speedStr = utils.FormatSpeed(p.SpeedBPS)
			bs.account = accountName

			if p.Stage == "decrypt" {
				if !hasStartedDecrypting {
					hasStartedDecrypting = true
					bs.bar.SetTotal(100, false)
					bs.bar.SetCurrent(0)
				}
				bs.bar.SetCurrent(int64(p.Percentage))
				
				if p.Percentage >= 100 {
					bs.stateTxt = "元数据写入中"
				} else if p.Percentage == 0 {
					bs.stateTxt = "账号等待解密中"
				} else {
					bs.stateTxt = "账号解密中"
				}

				bs.isDecrypt = true
			} else {
				hasStartedDecrypting = false
				bs.bar.SetTotal(100, false)
				bs.bar.SetCurrent(int64(p.Percentage))
				bs.isDecrypt = false

				if p.Percentage >= 100 {
					bs.stateTxt = "账号等待解密中"
				} else {
					bs.stateTxt = "下载中"
				}
			}
			bs.statusMu.Unlock()
		}
	}()
}

func (pui *ProgressUI) SetDone(trackIndex int, status string) {
	pui.mu.Lock()
	bs, ok := pui.bars[trackIndex]
	if ok {
		delete(pui.bars, trackIndex)
		pui.completedCount++
	}
	pui.mu.Unlock()

	if ok {
		bs.statusMu.Lock()
		bs.stateTxt = status
		bs.isDone = true
		bs.speedStr = ""
		bs.statusMu.Unlock()

		bs.bar.SetTotal(100, true)
		bs.bar.SetCurrent(100)
	}
}

func (pui *ProgressUI) Abort(trackIndex int, status string) {
	pui.mu.Lock()
	bs, ok := pui.bars[trackIndex]
	if ok {
		delete(pui.bars, trackIndex)
		pui.completedCount++
	}
	pui.mu.Unlock()

	if ok {
		bs.statusMu.Lock()
		bs.stateTxt = "失败: " + truncateString(status, 15)
		bs.isDone = true
		bs.speedStr = ""
		bs.statusMu.Unlock()

		bs.bar.SetTotal(100, true)
	}
}

func (pui *ProgressUI) Wait() {
	pui.p.Wait()
}

func SelectTracks(meta *structs.AutoGenerated, storefront, urlArg_i string) []int {
	trackTotal := len(meta.Data[0].Relationships.Tracks.Data)
	arr := make([]int, trackTotal)
	for i := 0; i < trackTotal; i++ {
		arr[i] = i + 1
	}
	selected := []int{}

	if core.Dl_song {
		found := false
		for i, track := range meta.Data[0].Relationships.Tracks.Data {
			if urlArg_i == track.ID {
				selected = append(selected, i+1)
				found = true
				break
			}
		}
		if !found {
			fmt.Println(errors.New("指定的单曲ID未在专辑中找到"))
			return nil
		}
	} else if !core.Dl_select {
		selected = arr
	} else {
		var data [][]string
		for trackNum, track := range meta.Data[0].Relationships.Tracks.Data {
			trackNum++
			var trackName string
			if meta.Data[0].Type == "albums" {
				trackName = fmt.Sprintf("%02d. %s", track.Attributes.TrackNumber, track.Attributes.Name)
			} else {
				trackName = fmt.Sprintf("%s - %s", track.Attributes.Name, track.Attributes.ArtistName)
			}
			data = append(data, []string{fmt.Sprint(trackNum),
				trackName,
				track.Attributes.ContentRating,
				track.Type})
		}
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"", "Track Name", "Rating", "Type"})
		table.SetRowLine(false)
		table.SetCaption(meta.Data[0].Type == "albums", fmt.Sprintf("Storefront: %s, %d tracks missing", strings.ToUpper(storefront), meta.Data[0].Attributes.TrackCount-trackTotal))
		table.SetHeaderColor(tablewriter.Colors{},
			tablewriter.Colors{tablewriter.FgRedColor, tablewriter.Bold},
			tablewriter.Colors{tablewriter.FgBlackColor, tablewriter.Bold},
			tablewriter.Colors{tablewriter.FgBlackColor, tablewriter.Bold})

		table.SetColumnColor(tablewriter.Colors{tablewriter.FgCyanColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor})
		for _, row := range data {
			if row[2] == "explicit" {
				row[2] = "E"
			} else if row[2] == "clean" {
				row[2] = "C"
			} else {
				row[2] = "None"
			}
			if row[3] == "music-videos" {
				row[3] = "MV"
			} else if row[3] == "songs" {
				row[3] = "SONG"
			}
			table.Append(row)
		}
		table.Render()
		fmt.Println("Please select from the track options above (multiple options separated by commas, ranges supported, or type 'all' to select all)")
		cyanColor := color.New(color.FgCyan)
		cyanColor.Print("select: ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println(err)
		}
		input = strings.TrimSpace(input)
		if input == "all" {
			selected = arr
		} else {
			selectedOptions := [][]string{}
			parts := strings.Split(input, ",")
			for _, part := range parts {
				if strings.Contains(part, "-") {
					rangeParts := strings.Split(part, "-")
					selectedOptions = append(selectedOptions, rangeParts)
				} else {
					selectedOptions = append(selectedOptions, []string{part})
				}
			}
			for _, opt := range selectedOptions {
				if len(opt) == 1 {
					num, err := strconv.Atoi(opt[0])
					if err != nil {
						continue
					}
					if num > 0 && num <= len(arr) {
						selected = append(selected, num)
					}
				} else if len(opt) == 2 {
					start, err1 := strconv.Atoi(opt[0])
					end, err2 := strconv.Atoi(opt[1])
					if err1 != nil || err2 != nil {
						continue
					}
					if start < 1 || end > len(arr) || start > end {
						continue
					}
					for i := start; i <= end; i++ {
						selected = append(selected, i)
					}
				}
			}
		}
	}
	return selected
}
