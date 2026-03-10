package models

import (
	"encoding/json"
	"time"
)

// DownloadStatus represents the current state of a download job.
type DownloadStatus string

const (
	DownloadStatusQueued      DownloadStatus = "queued"
	DownloadStatusDownloading DownloadStatus = "downloading"
	DownloadStatusProcessing  DownloadStatus = "processing"
	DownloadStatusCompleted   DownloadStatus = "completed"
	DownloadStatusFailed      DownloadStatus = "failed"
	DownloadStatusPaused      DownloadStatus = "paused"
)

// DownloadJob represents a single download task in the queue.
type DownloadJob struct {
	ID              string          `db:"id"               json:"id"`
	URL             string          `db:"url"              json:"url"`
	SourceType      string          `db:"source_type"      json:"source_type"`
	Status          DownloadStatus  `db:"status"           json:"status"`
	Progress        float64         `db:"progress"         json:"progress"` // 0.0 – 100.0
	TotalFiles      int             `db:"total_files"      json:"total_files"`
	DownloadedFiles int             `db:"downloaded_files" json:"downloaded_files"`
	TotalBytes      int64           `db:"total_bytes"      json:"total_bytes"`
	DownloadedBytes int64           `db:"downloaded_bytes" json:"downloaded_bytes"`
	TargetDirectory string          `db:"target_directory" json:"target_directory"`
	SourceMetadata  json.RawMessage `db:"source_metadata"  json:"source_metadata,omitempty"`
	AutoTags        json.RawMessage `db:"auto_tags"        json:"auto_tags,omitempty"`
	ErrorMessage    string          `db:"error_message"    json:"error_message,omitempty"`
	RetryCount      int             `db:"retry_count"      json:"retry_count"`
	CreatedAt       time.Time       `db:"created_at"       json:"created_at"`
	UpdatedAt       time.Time       `db:"updated_at"       json:"updated_at"`
	CompletedAt     *time.Time      `db:"completed_at"     json:"completed_at,omitempty"`
}
