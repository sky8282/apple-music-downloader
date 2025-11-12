package qobuz

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"main/internal/core"
	"main/internal/utils"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	appID      = "798273057"
	loginURL   = "https://www.qobuz.com/api.json/0.2/user/login"
	searchURL  = "https://www.qobuz.com/api.json/0.2/catalog/search"
	albumURL   = "https://www.qobuz.com/api.json/0.2/album/get"
	tokenFile  = "qobuz_token.json"
	userAgent  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
)

type LoginResponse struct {
	UserAuthToken string `json:"user_auth_token"`
}

type TokenCache struct {
	UserAuthToken string `json:"user_auth_token"`
}

type SearchResponse struct {
	Albums struct {
		Items []struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Artist struct {
				Name string `json:"name"`
			} `json:"artist"`
		} `json:"items"`
	} `json:"albums"`
}

type AlbumInfo struct {
	Description string `json:"description"`
	Extras      []struct {
		Type  string `json:"type"`
		URL   string `json:"url"`
		Title string `json:"title"`
	} `json:"extras"`
	Goodies []struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	} `json:"goodies"`
}

type PDFExtra struct {
	Title string
	URL   string
}

var (
	token      string
	tokenMutex sync.Mutex
	httpClient = &http.Client{Timeout: 15 * time.Second}
)

func saveToken(token string) {
	tokenMutex.Lock()
	defer tokenMutex.Unlock()
	cache := TokenCache{UserAuthToken: token}
	data, err := json.Marshal(cache)
	if err == nil {
		_ = os.WriteFile(tokenFile, data, 0644)
	}
}

func loadToken() string {
	tokenMutex.Lock()
	defer tokenMutex.Unlock()
	if _, err := os.Stat(tokenFile); err == nil {
		data, err := os.ReadFile(tokenFile)
		if err == nil {
			var cache TokenCache
			if json.Unmarshal(data, &cache) == nil {
				return cache.UserAuthToken
			}
		}
	}
	return ""
}

func login(email, password string) (string, error) {
	data := url.Values{}
	data.Set("email", email)
	data.Set("password", password)
	data.Set("app_id", appID)

	req, err := http.NewRequest("POST", loginURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login failed with status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var loginResp LoginResponse
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return "", err
	}

	if loginResp.UserAuthToken == "" {
		return "", errors.New("login failed, user_auth_token not returned")
	}

	token = loginResp.UserAuthToken
	saveToken(token)
	return token, nil
}

func getValidToken(email, password string) (string, error) {
	tokenMutex.Lock()
	if token != "" {
		tokenMutex.Unlock()
		return token, nil
	}
	tokenMutex.Unlock()

	cachedToken := loadToken()
	if cachedToken != "" {
		tokenMutex.Lock()
		token = cachedToken
		tokenMutex.Unlock()
		return cachedToken, nil
	}

	if email == "" || password == "" {
		return "", errors.New("Qobuz credentials not configured")
	}

	return login(email, password)
}

func searchQobuz(token, query string, limit int) (*SearchResponse, error) {
	params := url.Values{}
	params.Set("query", query)
	params.Set("limit", fmt.Sprintf("%d", limit))

	req, err := http.NewRequest("GET", fmt.Sprintf("%s?%s", searchURL, params.Encode()), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-App-Id", appID)
	req.Header.Set("X-User-Auth-Token", token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed with status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var searchResp SearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, err
	}
	return &searchResp, nil
}

func fetchAlbumInfo(token, albumID string) (*AlbumInfo, error) {
	params := url.Values{}
	params.Set("album_id", albumID)
	params.Set("user_auth_token", token)
	params.Set("app_id", appID)

	req, err := http.NewRequest("GET", fmt.Sprintf("%s?%s", albumURL, params.Encode()), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-App-Id", appID)
	req.Header.Set("X-User-Auth-Token", token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch album info failed with status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var albumInfo AlbumInfo
	if err := json.Unmarshal(body, &albumInfo); err != nil {
		return nil, err
	}
	return &albumInfo, nil
}

func CleanHTML(text string) string {
	re := regexp.MustCompile("<[^>]*>")
	cleanText := re.ReplaceAllString(text, "")
	re = regexp.MustCompile("&[a-z]+;")
	cleanText = re.ReplaceAllString(cleanText, "")
	return cleanText
}

func GetQobuzExtras(artistName, albumName string) (string, []PDFExtra, error) {
	if core.Config.QobuzUsername == "" || core.Config.QobuzPassword == "" {
		return "", nil, nil
	}

	authToken, err := getValidToken(core.Config.QobuzUsername, core.Config.QobuzPassword)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get Qobuz token: %w", err)
	}

	searchQuery := fmt.Sprintf("%s %s", artistName, albumName)
	searchResp, err := searchQobuz(authToken, searchQuery, 10)
	if err != nil {
		return "", nil, fmt.Errorf("Qobuz search failed: %w", err)
	}

	if searchResp == nil || len(searchResp.Albums.Items) == 0 {
		return "", nil, nil
	}

	var matchedAlbumID string
	for _, album := range searchResp.Albums.Items {
		if strings.EqualFold(strings.TrimSpace(album.Title), strings.TrimSpace(albumName)) &&
			strings.EqualFold(strings.TrimSpace(album.Artist.Name), strings.TrimSpace(artistName)) {
			matchedAlbumID = album.ID
			break
		}
	}

	if matchedAlbumID == "" {
		return "", nil, nil
	}

	albumInfo, err := fetchAlbumInfo(authToken, matchedAlbumID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to fetch Qobuz album info: %w", err)
	}

	var description string
	if albumInfo.Description != "" {
		description = CleanHTML(albumInfo.Description)
	}

	var pdfs []PDFExtra
	pdfMap := make(map[string]bool)

	for _, extra := range albumInfo.Extras {
		if (extra.Type == "pdf" || strings.HasSuffix(extra.URL, ".pdf")) && extra.URL != "" {
			if _, ok := pdfMap[extra.URL]; !ok {
				pdfs = append(pdfs, PDFExtra{Title: extra.Title, URL: extra.URL})
				pdfMap[extra.URL] = true
			}
		}
	}
	for _, g := range albumInfo.Goodies {
		if strings.HasSuffix(g.URL, ".pdf") && g.URL != "" {
			if _, ok := pdfMap[g.URL]; !ok {
				pdfs = append(pdfs, PDFExtra{Title: g.Name, URL: g.URL})
				pdfMap[g.URL] = true
			}
		}
	}

	return description, pdfs, nil
}

func DownloadPDF(pdf PDFExtra, saveFolder string) error {
	req, err := http.NewRequest("GET", pdf.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download PDF, status: %s", resp.Status)
	}

	var filename string
	if pdf.Title != "" {
		filename = core.ForbiddenNames.ReplaceAllString(pdf.Title, "_") + ".pdf"
	} else {
		urlParts := strings.Split(pdf.URL, "/")
		filename = urlParts[len(urlParts)-1]
		if !strings.HasSuffix(filename, ".pdf") {
			filename += ".pdf"
		}
	}
	filename = core.ForbiddenNames.ReplaceAllString(filename, "_")

	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	ext := filepath.Ext(filename)
	savePath := filepath.Join(saveFolder, filename)

	for i := 1; ; i++ {
		exists, err := utils.FileExists(savePath)
		if err != nil {
			return err
		}
		if !exists {
			break
		}
		filename = fmt.Sprintf("%s (%d)%s", base, i, ext)
		savePath = filepath.Join(saveFolder, filename)
	}

	f, err := os.Create(savePath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return err
	}
	return nil
}
