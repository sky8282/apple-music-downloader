package lyrics

import (
	"encoding/json"
	"errors"
	"fmt"
	"main/internal/core"
	"main/internal/translator"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/beevik/etree"
)

var translationLock sync.Mutex

type SongLyrics struct {
	Data []struct {
		Id         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Ttml              string `json:"ttml"`
			TtmlLocalizations string `json:"ttmlLocalizations"`
			PlayParams        struct {
				Id          string `json:"id"`
				Kind        string `json:"kind"`
				CatalogId   string `json:"catalogId"`
				DisplayType int    `json:"displayType"`
			} `json:"playParams"`
		} `json:"attributes"`
	} `json:"data"`
}

func Get(storefront, songId, lrcType, language, lrcFormat, token, mediaUserToken string, enableTranslation bool, transLanguage string) (string, error) {
	if len(mediaUserToken) < 50 {
		return "", errors.New("MediaUserToken not set")
	}

	reqLang := language
	if enableTranslation && transLanguage != "" {
		reqLang = transLanguage
	}

	ttml, err := getSongLyrics(songId, storefront, token, mediaUserToken, lrcType, reqLang)
	if err != nil {
		return "", err
	}

	if lrcFormat == "ttml" {
		return ttml, nil
	}

	lrc, err := TtmlToLrc(ttml, enableTranslation)
	if err != nil {
		return "", err
	}

	return lrc, nil
}

func getSongLyrics(songId string, storefront string, token string, userToken string, lrcType string, language string) (string, error) {
	req, err := http.NewRequest("GET",
		fmt.Sprintf("https://amp-api.music.apple.com/v1/catalog/%s/songs/%s/%s?l=%s&extend=ttmlLocalizations", storefront, songId, lrcType, language), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Origin", "https://music.apple.com")
	req.Header.Set("Referer", "https://music.apple.com/")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	cookie := http.Cookie{Name: "media-user-token", Value: userToken}
	req.AddCookie(&cookie)
	do, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer do.Body.Close()
	obj := new(SongLyrics)
	_ = json.NewDecoder(do.Body).Decode(&obj)
	if obj.Data != nil {
		if len(obj.Data[0].Attributes.TtmlLocalizations) > 0 {
			return obj.Data[0].Attributes.TtmlLocalizations, nil
		}
		if len(obj.Data[0].Attributes.Ttml) > 0 {
			return obj.Data[0].Attributes.Ttml, nil
		}
		return obj.Data[0].Attributes.TtmlLocalizations, nil
	} else {
		return "", errors.New("failed to get lyrics")
	}
}

func containsCJK(s string) bool {
	for _, r := range s {
		if (r >= 0x1100 && r <= 0x11FF) ||
			(r >= 0x2E80 && r <= 0x2EFF) ||
			(r >= 0x2F00 && r <= 0x2FDF) ||
			(r >= 0x2FF0 && r <= 0x2FFF) ||
			(r >= 0x3000 && r <= 0x303F) ||
			(r >= 0x3040 && r <= 0x309F) ||
			(r >= 0x30A0 && r <= 0x30FF) ||
			(r >= 0x3130 && r <= 0x318F) ||
			(r >= 0x31C0 && r <= 0x31EF) ||
			(r >= 0x31F0 && r <= 0x31FF) ||
			(r >= 0x3200 && r <= 0x32FF) ||
			(r >= 0x3300 && r <= 0x33FF) ||
			(r >= 0x3400 && r <= 0x4DBF) ||
			(r >= 0x4E00 && r <= 0x9FFF) ||
			(r >= 0xA960 && r <= 0xA97F) ||
			(r >= 0xAC00 && r <= 0xD7AF) ||
			(r >= 0xD7B0 && r <= 0xD7FF) ||
			(r >= 0xF900 && r <= 0xFAFF) ||
			(r >= 0xFE30 && r <= 0xFE4F) ||
			(r >= 0xFF65 && r <= 0xFF9F) ||
			(r >= 0xFFA0 && r <= 0xFFDC) ||
			(r >= 0x1AFF0 && r <= 0x1AFFF) ||
			(r >= 0x1B000 && r <= 0x1B0FF) ||
			(r >= 0x1B100 && r <= 0x1B12F) ||
			(r >= 0x1B130 && r <= 0x1B16F) ||
			(r >= 0x1F200 && r <= 0x1F2FF) ||
			(r >= 0x20000 && r <= 0x2A6DF) ||
			(r >= 0x2A700 && r <= 0x2B73F) ||
			(r >= 0x2B740 && r <= 0x2B81F) ||
			(r >= 0x2B820 && r <= 0x2CEAF) ||
			(r >= 0x2CEB0 && r <= 0x2EBEF) ||
			(r >= 0x2EBF0 && r <= 0x2EE5F) ||
			(r >= 0x2F800 && r <= 0x2FA1F) ||
			(r >= 0x30000 && r <= 0x3134F) ||
			(r >= 0x31350 && r <= 0x323AF) {
			return true
		}
	}
	return false
}

func TtmlToLrc(ttml string, enableTranslation bool) (string, error) {
	parsedTTML := etree.NewDocument()
	err := parsedTTML.ReadFromString(ttml)
	if err != nil {
		return "", err
	}

	timingAttr := parsedTTML.FindElement("tt").SelectAttr("itunes:timing")
	if timingAttr != nil && timingAttr.Value == "Word" {
		hasOfficialTrans := false
		if head := parsedTTML.FindElement("tt").FindElement("head"); head != nil {
			if meta := head.FindElement("metadata"); meta != nil {
				if iTunesMetadata := meta.FindElement("iTunesMetadata"); iTunesMetadata != nil {
					if len(iTunesMetadata.FindElements("translations")) > 0 {
						hasOfficialTrans = true
					}
				}
			}
		}

		if hasOfficialTrans || !enableTranslation {
			lrc, err := conventSyllableTTMLToLRC(ttml, enableTranslation)
			if err == nil {
				if !enableTranslation {
					return lrc, nil
				}
				if containsCJK(lrc) {
					return lrc, nil
				}
			}
		}
	}

	if timingAttr != nil && timingAttr.Value == "None" {
		var finalOutput []string
		var rawLines []string
		for _, p := range parsedTTML.FindElements("//p") {
			text := strings.TrimSpace(p.Text())
			if text != "" {
				rawLines = append(rawLines, text)
			}
		}

		if enableTranslation && len(rawLines) > 0 {
			translationLock.Lock()
			transEngine, err := translator.New(core.Config)
			if err == nil {
				fmt.Printf(" [纯文本歌词] 正在翻译 %d 行...\n", len(rawLines))
				translatedTexts, err := transEngine.Translate(rawLines, core.Config.TranslationLanguage)
				if err == nil && len(translatedTexts) == len(rawLines) {
					for i, line := range rawLines {
						finalOutput = append(finalOutput, line)
						if translatedTexts[i] != "" {
							finalOutput = append(finalOutput, translatedTexts[i])
						}
					}
					translationLock.Unlock()
					return strings.Join(finalOutput, "\n"), nil
				} else {
					if err != nil {
						fmt.Printf(" [纯文本翻译失败] %v\n", err)
					}
				}
			}
			translationLock.Unlock()
		}

		return strings.Join(rawLines, "\n"), nil
	}

	type LyricLine struct {
		M, S, MS int
		Text     string
		Trans    string
	}
	var lines []LyricLine

	body := parsedTTML.FindElement("tt").FindElement("body")
	if body == nil {
		return "", errors.New("invalid ttml: no body")
	}

	var iTunesMetadata *etree.Element
	if head := parsedTTML.FindElement("tt").FindElement("head"); head != nil {
		if meta := head.FindElement("metadata"); meta != nil {
			iTunesMetadata = meta.FindElement("iTunesMetadata")
		}
	}

	var elements []*etree.Element
	var collectElements func(e *etree.Element)
	collectElements = func(e *etree.Element) {
		isContainer := false
		for _, child := range e.ChildElements() {
			if child.Tag == "div" || child.Tag == "p" {
				isContainer = true
				break
			}
		}

		if isContainer {
			for _, child := range e.ChildElements() {
				collectElements(child)
			}
		} else {
			if e.SelectAttr("begin") != nil {
				elements = append(elements, e)
			}
		}
	}

	for _, child := range body.ChildElements() {
		collectElements(child)
	}

	parseTime := func(timeValue string) (int, int, int, error) {
		var h, m, s, ms int
		if strings.Contains(timeValue, ":") {
			_, err = fmt.Sscanf(timeValue, "%d:%d:%d.%d", &h, &m, &s, &ms)
			if err != nil {
				_, err = fmt.Sscanf(timeValue, "%d:%d.%d", &m, &s, &ms)
				h = 0
			}
		} else {
			_, err = fmt.Sscanf(timeValue, "%d.%d", &s, &ms)
			h, m = 0, 0
		}
		
		if err != nil {
			return 0, 0, 0, err
		}

		totalSeconds := h*3600 + m*60 + s
		
		finalM := totalSeconds / 60
		finalS := totalSeconds % 60
		finalMS := ms / 10

		return finalM, finalS, finalMS, nil
	}

	for _, el := range elements {
		beginValue := el.SelectAttr("begin").Value
		
		var l LyricLine
		var err error
		l.M, l.S, l.MS, err = parseTime(beginValue)
		if err != nil {
			continue
		}

		var textBuilder strings.Builder
		if attr := el.SelectAttr("text"); attr != nil {
			textBuilder.WriteString(attr.Value)
		} else {
			childIndex := 0
			for _, childToken := range el.Child {
				if cd, ok := childToken.(*etree.CharData); ok {
					if strings.TrimSpace(cd.Data) != "" || childIndex > 0 {
					}
				}
				
				if childElem, ok := childToken.(*etree.Element); ok {
					if childIndex > 0 {
						textBuilder.WriteString(" ")
					}
					
					var extractedText string
					if attr := childElem.SelectAttr("text"); attr != nil {
						extractedText = attr.Value
					} else {
						extractedText = childElem.Text()
					}
					textBuilder.WriteString(extractedText)
					childIndex++
				}
			}
			if textBuilder.Len() == 0 {
				textBuilder.WriteString(el.Text())
			}
		}
		
		l.Text = textBuilder.String()

		if enableTranslation && iTunesMetadata != nil {
			key := el.SelectAttr("itunes:key")
			if key != nil {
				xpath := fmt.Sprintf("translations/translation/text[@for='%s']", key.Value)
				if transNode := iTunesMetadata.FindElement(xpath); transNode != nil {
					if transNode.SelectAttr("text") != nil {
						l.Trans = transNode.SelectAttr("text").Value
					} else {
						l.Trans = transNode.Text()
					}
				}
			}
		}
		lines = append(lines, l)
	}

	hasOfficialTrans := false
	for _, l := range lines {
		if l.Trans != "" {
			hasOfficialTrans = true
			break
		}
	}

	if enableTranslation && !hasOfficialTrans {
		var textsToTranslate []string
		for _, l := range lines {
			if strings.TrimSpace(l.Text) != "" {
				textsToTranslate = append(textsToTranslate, l.Text)
			}
		}

		if len(textsToTranslate) > 0 {
			translationLock.Lock()
			time.Sleep(200 * time.Millisecond)

			transEngine, err := translator.New(core.Config)
			if err == nil {
				translatedTexts, err := transEngine.Translate(textsToTranslate, core.Config.TranslationLanguage)
				if err == nil {
					transIndex := 0
					for i := range lines {
						if strings.TrimSpace(lines[i].Text) != "" {
							if transIndex < len(translatedTexts) {
								lines[i].Trans = translatedTexts[transIndex]
								transIndex++
							}
						}
					}
				}
			}
			translationLock.Unlock()
		}
	}

	var lrcBuilder strings.Builder
	for _, l := range lines {
		lrcBuilder.WriteString(fmt.Sprintf("[%02d:%02d.%02d]%s\n", l.M, l.S, l.MS, l.Text))
		if l.Trans != "" {
			lrcBuilder.WriteString(fmt.Sprintf("[%02d:%02d.%02d]%s\n", l.M, l.S, l.MS, l.Trans))
		}
	}

	return strings.TrimSpace(lrcBuilder.String()), nil
}

func conventSyllableTTMLToLRC(ttml string, enableTranslation bool) (string, error) {
	parsedTTML := etree.NewDocument()
	err := parsedTTML.ReadFromString(ttml)
	if err != nil {
		return "", err
	}
	var lrcLines []string
	parseTime := func(timeValue string, newLine int) (string, error) {
		var h, m, s, ms int
		if strings.Contains(timeValue, ":") {
			_, err = fmt.Sscanf(timeValue, "%d:%d:%d.%d", &h, &m, &s, &ms)
			if err != nil {
				_, err = fmt.Sscanf(timeValue, "%d:%d.%d", &m, &s, &ms)
				h = 0
			}
		} else {
			_, err = fmt.Sscanf(timeValue, "%d.%d", &s, &ms)
			h, m = 0, 0
		}
		if err != nil {
			return "", err
		}
		
		totalSeconds := h*3600 + m*60 + s
		m = totalSeconds / 60
		s = totalSeconds % 60
		ms = ms / 10

		if newLine == 0 {
			return fmt.Sprintf("[%02d:%02d.%02d]", m, s, ms), nil
		} else if newLine == -1 {
			return fmt.Sprintf("[%02d:%02d.%02d]", m, s, ms), nil
		} else {
			return fmt.Sprintf("<%02d:%02d.%02d>", m, s, ms), nil
		}
	}
	divs := parsedTTML.FindElement("tt").FindElement("body").FindElements("div")
	for _, div := range divs {
		for _, item := range div.ChildElements() {
			var lineTextBuilder strings.Builder
			var lineStartTime string
			var i int = 0
			var translitLine, transLine string

			for _, lyrics := range item.Child {
				if _, ok := lyrics.(*etree.CharData); ok {
					if i > 0 {
						lineTextBuilder.WriteString(" ")
						continue
					}
					continue
				}
				lyric := lyrics.(*etree.Element)
				if lyric.SelectAttr("begin") == nil {
					continue
				}

				if i == 0 {
					lineStartTime, err = parseTime(lyric.SelectAttr("begin").Value, -1)
					if err != nil {
						return "", err
					}
				}

				var text string
				if lyric.SelectAttr("text") == nil {
					var textTmp []string
					for _, span := range lyric.Child {
						if _, ok := span.(*etree.CharData); ok {
							textTmp = append(textTmp, span.(*etree.CharData).Data)
						} else {
							textTmp = append(textTmp, span.(*etree.Element).Text())
						}
					}
					text = strings.Join(textTmp, "")
				} else {
					text = lyric.SelectAttr("text").Value
				}

				if i > 0 {
					lineTextBuilder.WriteString(" ")
				}
				lineTextBuilder.WriteString(text)

				if i == 0 {
					transBeginTime := lineStartTime
					if len(parsedTTML.FindElement("tt").FindElements("head")) > 0 {
						if len(parsedTTML.FindElement("tt").FindElement("head").FindElements("metadata")) > 0 {
							Metadata := parsedTTML.FindElement("tt").FindElement("head").FindElement("metadata")
							if len(Metadata.FindElements("iTunesMetadata")) > 0 {
								iTunesMetadata := Metadata.FindElement("iTunesMetadata")

								if len(iTunesMetadata.FindElements("transliterations")) > 0 {
									if len(iTunesMetadata.FindElement("transliterations").FindElements("transliteration")) > 0 {
										xpath := fmt.Sprintf("text[@for='%s']", item.SelectAttr("itunes:key").Value)
										trans := iTunesMetadata.FindElement("transliterations").FindElement("transliteration").FindElement(xpath)
										var transTxtParts []string
										if trans != nil {
											for _, span := range trans.ChildElements() {
												if span.Tag == "span" {
													spanText := span.Text()
													transTxtParts = append(transTxtParts, spanText)
												}
											}
										}
										if len(transTxtParts) > 0 {
											translitLine = fmt.Sprintf("%s%s", transBeginTime, strings.Join(transTxtParts, " "))
										}
									}
								}

								if enableTranslation {
									if len(iTunesMetadata.FindElements("translations")) > 0 {
										if len(iTunesMetadata.FindElement("translations").FindElements("translation")) > 0 {
											xpath := fmt.Sprintf("text[@for='%s']", item.SelectAttr("itunes:key").Value)
											trans := iTunesMetadata.FindElement("translations").FindElement("translation").FindElement(xpath)
											if trans != nil {
												var transTxt string
												if trans.SelectAttr("text") == nil {
													var textTmp []string
													for _, span := range trans.Child {
														if _, ok := span.(*etree.CharData); ok {
															textTmp = append(textTmp, span.(*etree.CharData).Data)
														}
													}
													transTxt = strings.Join(textTmp, "")
												} else {
													transTxt = trans.SelectAttr("text").Value
												}
												transLine = lineStartTime + transTxt
											}
										}
									}
								}
							}
						}
					}
				}
				i += 1
			}

			finalLineText := lineTextBuilder.String()

			if len(translitLine) > 0 && containsCJK(finalLineText) {
				lrcLines = append(lrcLines, translitLine)
			} else {
				lrcLines = append(lrcLines, lineStartTime+finalLineText)
			}

			if enableTranslation && len(transLine) > 0 {
				lrcLines = append(lrcLines, transLine)
			}
		}
	}
	return strings.Join(lrcLines, "\n"), nil
}
