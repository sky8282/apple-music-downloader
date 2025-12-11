package translator

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"main/utils/structs"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Translator interface {
	Translate(texts []string, targetLang string) ([]string, error)
}

func New(cfg structs.ConfigSet) (Translator, error) {
	switch strings.ToLower(cfg.TranslatorProvider) {
	case "libretranslate":
		if cfg.LibreTranslate.Url == "" {
			return nil, errors.New("libretranslate url is empty")
		}
		cfg.LibreTranslate.Url = strings.TrimRight(cfg.LibreTranslate.Url, "/")
		return &LibreTranslateTranslator{Config: cfg.LibreTranslate}, nil
	case "tencent":
		if cfg.Tencent.SecretId == "" || cfg.Tencent.SecretKey == "" {
			return nil, errors.New("tencent secret-id or secret-key is empty")
		}
		return &TencentTranslator{Config: cfg.Tencent}, nil
	case "deepl":
		if cfg.DeepL.AuthKey == "" {
			return nil, errors.New("deepl auth-key is empty")
		}
		return &DeepLTranslator{Config: cfg.DeepL}, nil
	case "microsoft":
		if cfg.Microsoft.Key == "" || cfg.Microsoft.Region == "" {
			return nil, errors.New("microsoft key or region is empty")
		}
		return &MicrosoftTranslator{Config: cfg.Microsoft}, nil
	case "google":
		if cfg.Google.ApiKey == "" {
			return nil, errors.New("google api-key is empty")
		}
		return &GoogleTranslator{Config: cfg.Google}, nil
	default:
		return nil, fmt.Errorf("unknown translator provider: %s", cfg.TranslatorProvider)
	}
}

type LibreTranslateTranslator struct {
	Config structs.LibreTranslateConfig
}

func (l *LibreTranslateTranslator) Translate(texts []string, targetLang string) ([]string, error) {
	if strings.HasPrefix(strings.ToLower(targetLang), "zh") {
		targetLang = "zh"
	}

	apiUrl := l.Config.Url + "/translate"
	payload := map[string]interface{}{
		"q":      texts,
		"source": "auto",
		"target": targetLang,
		"format": "text",
	}

	if l.Config.ApiKey != "" {
		payload["api_key"] = l.Config.ApiKey
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", apiUrl, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("libretranslate api error, status code: %d", resp.StatusCode)
	}

	var response struct {
		TranslatedText interface{} `json:"translatedText"`
		Error          string      `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	if response.Error != "" {
		return nil, errors.New(response.Error)
	}

	var result []string
	if strs, ok := response.TranslatedText.([]interface{}); ok {
		for _, s := range strs {
			if str, ok := s.(string); ok {
				result = append(result, str)
			} else {
				result = append(result, "")
			}
		}
	} else if str, ok := response.TranslatedText.(string); ok {
		result = append(result, str)
	} else {
		return nil, errors.New("invalid response format from libretranslate")
	}

	if len(result) != len(texts) {
		return nil, fmt.Errorf("translation count mismatch: sent %d, got %d", len(texts), len(result))
	}

	return result, nil
}

type TencentTranslator struct {
	Config structs.TencentConfig
}

func (t *TencentTranslator) Translate(texts []string, targetLang string) ([]string, error) {
	if targetLang == "zh-Hans-CN" || targetLang == "zh-Hans" {
		targetLang = "zh"
	}
	endpoint := "tmt.tencentcloudapi.com"
	service := "tmt"
	action := "TextTranslateBatch"
	version := "2018-03-21"
	payload := map[string]interface{}{
		"SourceTextList": texts,
		"Source":         "auto",
		"Target":         targetLang,
		"ProjectId":      0,
	}
	payloadBytes, _ := json.Marshal(payload)
	ts := time.Now().Unix()
	date := time.Now().UTC().Format("2006-01-02")
	canonicalHeaders := fmt.Sprintf("content-type:application/json; charset=utf-8\nhost:%s\n", endpoint)
	signedHeaders := "content-type;host"
	hashedRequestPayload := sha256Hex(payloadBytes)
	canonicalRequest := fmt.Sprintf("POST\n/\n\n%s\n%s\n%s", canonicalHeaders, signedHeaders, hashedRequestPayload)
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service) 
	hashedCanonicalRequest := sha256Hex([]byte(canonicalRequest))
	stringToSign := fmt.Sprintf("TC3-HMAC-SHA256\n%d\n%s\n%s", ts, credentialScope, hashedCanonicalRequest)
	secretDate := hmacSha256([]byte("TC3"+t.Config.SecretKey), []byte(date))
	secretService := hmacSha256(secretDate, []byte(service))
	secretSigning := hmacSha256(secretService, []byte("tc3_request"))
	signature := hex.EncodeToString(hmacSha256(secretSigning, []byte(stringToSign)))
	authHeader := fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		t.Config.SecretId, credentialScope, signedHeaders, signature)
	req, _ := http.NewRequest("POST", "https://"+endpoint, bytes.NewBuffer(payloadBytes))
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Host", endpoint)
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Version", version)
	req.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", ts))
	req.Header.Set("X-TC-Region", t.Config.Region)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var response struct {
		Response struct {
			TargetTextList []string `json:"TargetTextList"`
			Error          struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	if response.Response.Error.Code != "" {
		return nil, fmt.Errorf("tencent api error: %s - %s", response.Response.Error.Code, response.Response.Error.Message)
	}
	return response.Response.TargetTextList, nil
}

type DeepLTranslator struct {
	Config structs.DeepLConfig
}

func (d *DeepLTranslator) Translate(texts []string, targetLang string) ([]string, error) {
	if strings.HasPrefix(targetLang, "zh") {
		targetLang = "ZH"
	}
	
	domain := "api-free.deepl.com"
	if d.Config.IsPro {
		domain = "api.deepl.com"
	}
	apiUrl := fmt.Sprintf("https://%s/v2/translate", domain)
	data := url.Values{}
	for _, text := range texts {
		data.Add("text", text)
	}
	data.Set("target_lang", targetLang)
	req, _ := http.NewRequest("POST", apiUrl, strings.NewReader(data.Encode()))
	req.Header.Set("Authorization", "DeepL-Auth-Key "+d.Config.AuthKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var response struct {
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	var result []string
	for _, t := range response.Translations {
		result = append(result, t.Text)
	}
	return result, nil
}

type MicrosoftTranslator struct {
	Config structs.MicrosoftConfig
}

func (m *MicrosoftTranslator) Translate(texts []string, targetLang string) ([]string, error) {
	if targetLang == "zh-Hans-CN" {
		targetLang = "zh-Hans"
	}

	apiUrl := "https://api.cognitive.microsofttranslator.com/translate?api-version=3.0&to=" + targetLang
	
	var body []map[string]string
	for _, text := range texts {
		body = append(body, map[string]string{"Text": text})
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", apiUrl, bytes.NewBuffer(jsonBody))
	req.Header.Set("Ocp-Apim-Subscription-Key", m.Config.Key)
	req.Header.Set("Ocp-Apim-Subscription-Region", m.Config.Region)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var response []struct {
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	var result []string
	for _, item := range response {
		if len(item.Translations) > 0 {
			result = append(result, item.Translations[0].Text)
		} else {
			result = append(result, "")
		}
	}
	return result, nil
}

type GoogleTranslator struct {
	Config structs.GoogleConfig
}

func (g *GoogleTranslator) Translate(texts []string, targetLang string) ([]string, error) {
	if targetLang == "zh-Hans-CN" {
		targetLang = "zh-CN"
	}
	
	apiUrl := "https://translation.googleapis.com/language/translate/v2?key=" + g.Config.ApiKey
	payload := map[string]interface{}{
		"q":      texts,
		"target": targetLang,
		"format": "text",
	}
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", apiUrl, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var response struct {
		Data struct {
			Translations []struct {
				TranslatedText string `json:"translatedText"`
			} `json:"translations"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	var result []string
	for _, t := range response.Data.Translations {
		result = append(result, t.TranslatedText)
	}
	return result, nil
}

func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func hmacSha256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
