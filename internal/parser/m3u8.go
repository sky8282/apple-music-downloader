package parser

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"main/internal/core"
	"main/internal/logger"
	"main/utils/structs"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/grafov/m3u8"
	"github.com/olekukonko/tablewriter"
)

// ExtractMvAudio extracts the best audio stream URL from a music video's master m3u8
func ExtractMvAudio(c string) (string, error) {
	MediaUrl, err := url.Parse(c)
	if err != nil {
		return "", err
	}
	resp, err := http.Get(c)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errors.New(resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	audioString := string(body)
	from, listType, err := m3u8.DecodeFrom(strings.NewReader(audioString), true)
	if err != nil || listType != m3u8.MASTER {
		return "", errors.New("m3u8 not of media type")
	}
	audio := from.(*m3u8.MasterPlaylist)

	var audioPriority = []string{"audio-atmos", "audio-ac3", "audio-stereo-256"}
	if *core.Mv_audio_type == "ac3" {
		audioPriority = []string{"audio-ac3", "audio-stereo-256"}
	} else if *core.Mv_audio_type == "aac" {
		audioPriority = []string{"audio-stereo-256"}
	}

	re := regexp.MustCompile(`_gr(\d+)_`)

	type AudioStream struct {
		URL     string
		Rank    int
		GroupID string
	}
	var audioStreams []AudioStream

	for _, variant := range audio.Variants {
		for _, audiov := range variant.Alternatives {
			if audiov.URI != "" {
				for _, priority := range audioPriority {
					if audiov.GroupId == priority {
						matches := re.FindStringSubmatch(audiov.URI)
						if len(matches) == 2 {
							var rank int
							fmt.Sscanf(matches[1], "%d", &rank)
							streamUrl, _ := MediaUrl.Parse(audiov.URI)
							audioStreams = append(audioStreams, AudioStream{
								URL:     streamUrl.String(),
								Rank:    rank,
								GroupID: audiov.GroupId,
							})
						}
					}
				}
			}
		}
	}
	if len(audioStreams) == 0 {
		return "", errors.New("no suitable audio stream found")
	}
	sort.Slice(audioStreams, func(i, j int) bool {
		return audioStreams[i].Rank > audioStreams[j].Rank
	})
	return audioStreams[0].URL, nil
}

// CheckM3u8 retrieves the m3u8 URL from a connected device
func CheckM3u8(b string, f string, account *structs.Account) (string, error) {
	var EnhancedHls string
	if core.Config.GetM3u8FromDevice {
		adamID := b
		conn, err := net.Dial("tcp", account.GetM3u8Port)
		if err != nil {
			return "none", err
		}
		defer conn.Close()

		adamIDBuffer := []byte(adamID)
		lengthBuffer := []byte{byte(len(adamIDBuffer))}

		_, err = conn.Write(lengthBuffer)
		if err != nil {
			return "none", err
		}
		_, err = conn.Write(adamIDBuffer)
		if err != nil {
			return "none", err
		}
		response, err := bufio.NewReader(conn).ReadBytes('\n')
		if err != nil {
			return "none", err
		}
		response = bytes.TrimSpace(response)
		if len(response) > 0 {
			EnhancedHls = string(response)
		}
	}
	return EnhancedHls, nil
}

func formatAvailability(available bool, quality string) string {
	if !available {
		return "Not Available"
	}
	return quality
}

// ExtractMedia extracts the best media stream URL and quality info from a master m3u8
func ExtractMedia(b string, more_mode bool) (string, string, string, error) {
	masterUrl, err := url.Parse(b)
	if err != nil {
		return "", "", "", err
	}
	resp, err := http.Get(b)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", "", errors.New(resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", err
	}
	masterString := string(body)
	from, listType, err := m3u8.DecodeFrom(strings.NewReader(masterString), true)
	if err != nil || listType != m3u8.MASTER {
		return "", "", "", errors.New("m3u8 not of master type")
	}
	master := from.(*m3u8.MasterPlaylist)
	var streamUrl *url.URL
	sort.Slice(master.Variants, func(i, j int) bool {
		return master.Variants[i].AverageBandwidth > master.Variants[j].AverageBandwidth
	})

	var hasAAC, hasLossless, hasHiRes, hasAtmos, hasDolbyAudio bool
	var aacQuality, losslessQuality, hiResQuality, atmosQuality, dolbyAudioQuality string

	for _, variant := range master.Variants {
		if variant.Codecs == "mp4a.40.2" { // AAC
			hasAAC = true
			split := strings.Split(variant.Audio, "-")
			// Handle different AAC formats: standard, binaural, downmix
			// e.g., "audio-stereo-256", "audio-stereo-binaural-256", "audio-stereo-downmix-256"
			if len(split) >= 3 {
				// Get the bitrate from the last part
				bitrateStr := split[len(split)-1]
				bitrate, _ := strconv.Atoi(bitrateStr)
				currentBitrate := 0
				if aacQuality != "" {
					fmt.Sscanf(aacQuality, "%d kbps", &currentBitrate)
				}
				if bitrate > currentBitrate {
					aacQuality = fmt.Sprintf("%d kbps", bitrate)
				}
			}
		} else if variant.Codecs == "ec-3" && strings.Contains(variant.Audio, "atmos") { // Dolby Atmos
			hasAtmos = true
			split := strings.Split(variant.Audio, "-")
			if len(split) > 0 {
				bitrateStr := split[len(split)-1]
				if len(bitrateStr) == 4 && bitrateStr[0] == '2' {
					bitrateStr = bitrateStr[1:]
				}
				bitrate, _ := strconv.Atoi(bitrateStr)
				currentBitrate := 0
				if atmosQuality != "" {
					fmt.Sscanf(atmosQuality, "%d kbps", &currentBitrate)
				}
				if bitrate > currentBitrate {
					atmosQuality = fmt.Sprintf("%d kbps", bitrate)
				}
			}
		} else if variant.Codecs == "alac" { // ALAC (Lossless or Hi-Res)
			split := strings.Split(variant.Audio, "-")
			if len(split) >= 3 {
				bitDepth := split[len(split)-1]
				sampleRate := split[len(split)-2]
				sampleRateInt, _ := strconv.Atoi(sampleRate)
				if sampleRateInt > 48000 { // Hi-Res
					hasHiRes = true
					hiResQuality = fmt.Sprintf("%sbit/%.1fkHz", bitDepth, float64(sampleRateInt)/1000.0)
				} else { // Standard Lossless
					hasLossless = true
					losslessQuality = fmt.Sprintf("%sbit/%.1fkHz", bitDepth, float64(sampleRateInt)/1000.0)
				}
			}
		} else if variant.Codecs == "ac-3" { // Dolby Audio
			hasDolbyAudio = true
			split := strings.Split(variant.Audio, "-")
			if len(split) > 0 {
				bitrate, _ := strconv.Atoi(split[len(split)-1])
				dolbyAudioQuality = fmt.Sprintf("%d kbps", bitrate)
			}
		}
	}

	var qualityForDisplay string
	if hasHiRes {
		qualityForDisplay = hiResQuality
	} else if hasLossless {
		qualityForDisplay = losslessQuality
	} else if hasAtmos {
		qualityForDisplay = "Dolby Atmos"
	} else if hasDolbyAudio {
		qualityForDisplay = "Dolby Audio"
	} else if hasAAC {
		qualityForDisplay = "AAC"
	}

	if core.Debug_mode && more_mode {
		logger.Debug("\nDebug: All Available Variants:")
		var data [][]string
		for _, variant := range master.Variants {
			data = append(data, []string{variant.Codecs, variant.Audio, fmt.Sprint(variant.Bandwidth)})
		}
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Codec", "Audio", "Bandwidth"})
		table.SetRowLine(true)
		table.AppendBulk(data)
		table.Render()

		logger.Debug("Available Audio Formats:")
		logger.Debug("------------------------")
		logger.Debug("AAC             : %s", formatAvailability(hasAAC, aacQuality))
		
		// Additional AAC variant details for debugging binaural/downmix issues
		if hasAAC {
			logger.Debug("AAC Variants Found:")
			for _, variant := range master.Variants {
				if variant.Codecs == "mp4a.40.2" {
					logger.Debug("  - %s", variant.Audio)
				}
			}
		}
		
		logger.Debug("Lossless        : %s", formatAvailability(hasLossless, losslessQuality))
		logger.Debug("Hi-Res Lossless : %s", formatAvailability(hasHiRes, hiResQuality))
		logger.Debug("Dolby Atmos     : %s", formatAvailability(hasAtmos, atmosQuality))
		logger.Debug("Dolby Audio     : %s", formatAvailability(hasDolbyAudio, dolbyAudioQuality))
		logger.Debug("------------------------")

		return "", "", "", nil
	}
	var qualityForFilename string
	for _, variant := range master.Variants {
		if core.Dl_atmos {
			if variant.Codecs == "ec-3" && strings.Contains(variant.Audio, "atmos") {
				split := strings.Split(variant.Audio, "-")
				length_int, err := strconv.Atoi(split[len(split)-1])
				if err == nil && length_int <= *core.Atmos_max {
					streamUrl, _ = masterUrl.Parse(variant.URI)
					qualityForFilename = fmt.Sprintf("%s kbps", split[len(split)-1])
					break
				}
			} else if variant.Codecs == "ac-3" {
				streamUrl, _ = masterUrl.Parse(variant.URI)
				split := strings.Split(variant.Audio, "-")
				qualityForFilename = fmt.Sprintf("%s kbps", split[len(split)-1])
				break
			}
		} else if core.Dl_aac {
			if variant.Codecs == "mp4a.40.2" {
				// Fix for aac-binaural and aac-downmix formats:
				// Apple Music AAC streams can have different naming patterns:
				// - Standard AAC: "audio-stereo-256" → should match "aac"
				// - Binaural AAC (Pattern 1): "audio-stereo-binaural-256" → should match "aac-binaural"
				// - Binaural AAC (Pattern 2): "audio-stereo-256-binaural" → should match "aac-binaural"
				// - Downmix AAC (Pattern 1): "audio-stereo-downmix-256" → should match "aac-downmix"
				// - Downmix AAC (Pattern 2): "audio-stereo-256-downmix" → should match "aac-downmix"
				//
				// The previous regex `audio-stereo-\d+` only matched the standard format,
				// causing binaural/downmix streams to be skipped, leading to bitstream
				// parsing errors when the wrong variant was selected as fallback.
				//
				// New approach: Parse the audio string to detect format type and bitrate
				var audioFormat string
				if strings.HasPrefix(variant.Audio, "audio-stereo-") {
					// Remove "audio-stereo-" prefix
					remainder := strings.TrimPrefix(variant.Audio, "audio-stereo-")
					parts := strings.Split(remainder, "-")
					
					if len(parts) == 1 {
						// Format: "256" (standard AAC)
						audioFormat = "aac"
					} else if len(parts) >= 2 {
						// Check both patterns:
						// Pattern 1: "binaural-256" or "downmix-256"
						// Pattern 2: "256-binaural" or "256-downmix"
						
						// Try to parse first part as bitrate to detect pattern
						if _, err := strconv.Atoi(parts[0]); err == nil && len(parts) >= 2 {
							// Pattern 2: bitrate comes first, format type is last
							formatType := parts[len(parts)-1]
							audioFormat = "aac-" + formatType
						} else {
							// Pattern 1: format type comes first
							audioFormat = "aac-" + parts[0]
						}
					}
				}
				
				if audioFormat == *core.Aac_type {
					streamUrl, _ = masterUrl.Parse(variant.URI)
					split := strings.Split(variant.Audio, "-")
					
					// Extract bitrate - try to find the numeric part
					var bitrate string
					for _, part := range split {
						if _, err := strconv.Atoi(part); err == nil {
							bitrate = part
							break
						}
					}
					if bitrate == "" {
						// Fallback: use last part
						bitrate = split[len(split)-1]
					}
					qualityForFilename = fmt.Sprintf("%s kbps", bitrate)
					
					// Debug logging when debug mode is enabled
					if core.Debug_mode {
						logger.Debug("Selected AAC variant: %s (audio: %s, type: %s, bitrate: %s)", 
							variant.URI, variant.Audio, *core.Aac_type, bitrate)
					}
					break
				}
			}
		} else {
			if variant.Codecs == "alac" {
				split := strings.Split(variant.Audio, "-")
				length_int, err := strconv.Atoi(split[len(split)-2])
				if err == nil && length_int <= *core.Alac_max {
					streamUrl, _ = masterUrl.Parse(variant.URI)
					KHZ := float64(length_int) / 1000.0
					qualityForFilename = fmt.Sprintf("%sB-%.1fkHz", split[len(split)-1], KHZ)
					break
				}
			}
		}
	}
	if streamUrl == nil {
		// Log warning when requested AAC type wasn't found
		if core.Dl_aac && len(master.Variants) > 0 {
			logger.Warn("Warning: Requested AAC type '%s' not found in playlist. Available AAC variants:", *core.Aac_type)
			for _, variant := range master.Variants {
				if variant.Codecs == "mp4a.40.2" {
					logger.Warn("  - %s", variant.Audio)
				}
			}
			logger.Warn("Falling back to first available variant. This may cause bitstream parsing errors.")
		}
		
		if len(master.Variants) > 0 {
			streamUrl, _ = masterUrl.Parse(master.Variants[0].URI)
		} else {
			return "", "", qualityForDisplay, errors.New("no variants found in playlist")
		}
	}
	return streamUrl.String(), qualityForFilename, qualityForDisplay, nil
}

// ExtractVideo extracts the best video stream URL from a master m3u8 and returns resolution info
func ExtractVideo(c string) (string, string, error) {
	MediaUrl, err := url.Parse(c)
	if err != nil {
		return "", "", err
	}
	resp, err := http.Get(c)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", errors.New(resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	videoString := string(body)

	from, listType, err := m3u8.DecodeFrom(strings.NewReader(videoString), true)
	if err != nil || listType != m3u8.MASTER {
		return "", "", errors.New("m3u8 not of media type")
	}
	video := from.(*m3u8.MasterPlaylist)

	var streamUrl *url.URL
	var resolution string
	sort.Slice(video.Variants, func(i, j int) bool {
		return video.Variants[i].AverageBandwidth > video.Variants[j].AverageBandwidth
	})

	maxHeight := *core.Mv_max
	re := regexp.MustCompile(`_(\d+)x(\d+)`)

	for _, variant := range video.Variants {
		matches := re.FindStringSubmatch(variant.URI)
		if len(matches) == 3 {
			width, _ := strconv.Atoi(matches[1])
			height, _ := strconv.Atoi(matches[2])
			if height <= maxHeight {
				streamUrl, _ = MediaUrl.Parse(variant.URI)
				// Determine quality label based on height
				var qualityLabel string
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
				resolution = fmt.Sprintf("%dx%d (%s)", width, height, qualityLabel)
				break
			}
		}
	}

	if streamUrl == nil {
		if len(video.Variants) > 0 {
			streamUrl, _ = MediaUrl.Parse(video.Variants[0].URI)
			// Try to extract resolution from first variant
			matches := re.FindStringSubmatch(video.Variants[0].URI)
			if len(matches) == 3 {
				width, _ := strconv.Atoi(matches[1])
				height, _ := strconv.Atoi(matches[2])
				var qualityLabel string
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
				resolution = fmt.Sprintf("%dx%d (%s)", width, height, qualityLabel)
			} else {
				resolution = "未知"
			}
		} else {
			return "", "", errors.New("no suitable video stream found")
		}
	}
	return streamUrl.String(), resolution, nil
}
