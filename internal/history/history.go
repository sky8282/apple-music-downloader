package history

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TaskHistory represents a batch download task with history tracking
type TaskHistory struct {
	TaskID       string           `json:"task_id"`
	TaskFile     string           `json:"task_file"`
	StartTime    time.Time        `json:"start_time"`
	EndTime      time.Time        `json:"end_time,omitempty"`
	TotalCount   int              `json:"total_count"`
	SuccessCount int              `json:"success_count"`
	FailedCount  int              `json:"failed_count"`
	SkippedCount int              `json:"skipped_count"`
	Records      []DownloadRecord `json:"records"`
	mu           sync.Mutex       `json:"-"`
}

// DownloadRecord represents a single download record
type DownloadRecord struct {
	URL         string    `json:"url"`
	AlbumID     string    `json:"album_id"`
	AlbumName   string    `json:"album_name,omitempty"`
	Status      string    `json:"status"` // success, failed, skipped
	DownloadAt  time.Time `json:"download_at"`
	ErrorMsg    string    `json:"error_msg,omitempty"`
	
	// Quality parameters for detecting changes
	QualityHash string `json:"quality_hash,omitempty"`
	GetM3u8Mode string `json:"get_m3u8_mode,omitempty"`
	AacType     string `json:"aac_type,omitempty"`
	AlacMax     int    `json:"alac_max,omitempty"`
	AtmosMax    int    `json:"atmos_max,omitempty"`
}

var (
	currentTask  *TaskHistory
	historyDir   = "history"
	taskMutex    sync.Mutex
)

// InitHistory initializes the history directory
func InitHistory() error {
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}
	return nil
}

// NewTask creates a new task history
func NewTask(taskFile string, totalCount int) (*TaskHistory, error) {
	taskMutex.Lock()
	defer taskMutex.Unlock()

	timestamp := time.Now().Unix()
	taskID := fmt.Sprintf("%s_%d", filepath.Base(taskFile), timestamp)
	
	currentTask = &TaskHistory{
		TaskID:     taskID,
		TaskFile:   taskFile,
		StartTime:  time.Now(),
		TotalCount: totalCount,
		Records:    make([]DownloadRecord, 0),
	}
	
	return currentTask, nil
}

// AddRecord adds a download record to the current task
func AddRecord(record DownloadRecord) {
	taskMutex.Lock()
	defer taskMutex.Unlock()
	
	if currentTask == nil {
		return
	}
	
	currentTask.mu.Lock()
	defer currentTask.mu.Unlock()
	
	currentTask.Records = append(currentTask.Records, record)
	
	switch record.Status {
	case "success":
		currentTask.SuccessCount++
	case "failed":
		currentTask.FailedCount++
	case "skipped":
		currentTask.SkippedCount++
	}
}

// SaveTask saves the current task history to a JSON file
func SaveTask() error {
	taskMutex.Lock()
	defer taskMutex.Unlock()
	
	if currentTask == nil {
		return fmt.Errorf("no active task to save")
	}
	
	currentTask.EndTime = time.Now()
	
	filename := filepath.Join(historyDir, currentTask.TaskID+".json")
	data, err := json.MarshalIndent(currentTask, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task history: %w", err)
	}
	
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}
	
	return nil
}

// GetCompletedRecords loads completed records from previous history files for the same task file
func GetCompletedRecords(taskFile string) (map[string]*DownloadRecord, error) {
	records := make(map[string]*DownloadRecord)
	
	// Check if history directory exists
	if _, err := os.Stat(historyDir); os.IsNotExist(err) {
		return records, nil
	}
	
	// Read all history files
	files, err := os.ReadDir(historyDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read history directory: %w", err)
	}
	
	baseTaskFile := filepath.Base(taskFile)
	
	// Find history files matching this task file
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		
		// Check if this history file is for the same task file
		if !strings.HasPrefix(file.Name(), baseTaskFile+"_") {
			continue
		}
		
		// Load the history file
		fullPath := filepath.Join(historyDir, file.Name())
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue // Skip files we can't read
		}
		
		var task TaskHistory
		if err := json.Unmarshal(data, &task); err != nil {
			continue // Skip invalid JSON files
		}
		
		// Add successful records to the map (latest overwrites)
		for _, record := range task.Records {
			if record.Status == "success" {
				recordCopy := record
				records[record.URL] = &recordCopy
			}
		}
	}
	
	return records, nil
}

// GetQualityHash generates a hash string representing the quality settings
func GetQualityHash(getM3u8Mode, aacType string, alacMax, atmosMax int) string {
	data := fmt.Sprintf("%s|%s|%d|%d", getM3u8Mode, aacType, alacMax, atmosMax)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:8]) // Use first 8 bytes for brevity
}
