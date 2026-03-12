package downloader

import (
	"context"

	"github.com/nacl-dev/tanuki/internal/models"
)

// SourceMetadata holds metadata fetched from a remote source before downloading.
type SourceMetadata struct {
	Title       string            `json:"title"`
	WorkTitle   string            `json:"work_title,omitempty"`
	WorkIndex   int               `json:"work_index,omitempty"`
	Description string            `json:"description"`
	Tags        []string          `json:"tags"`
	TotalFiles  int               `json:"total_files"`
	Extra       map[string]string `json:"extra,omitempty"`
}

// Engine is the interface that each download backend must implement.
type Engine interface {
	// CanHandle returns true if this engine can process the given URL.
	CanHandle(url string) bool

	// Download fetches the content at job.URL and writes it to job.TargetDirectory.
	Download(ctx context.Context, job *models.DownloadJob) error

	// FetchMetadata retrieves metadata from the remote source without downloading.
	FetchMetadata(url string) (*SourceMetadata, error)
}

// ProgressAwareEngine can stream incremental progress updates back to the manager.
type ProgressAwareEngine interface {
	SetProgressUpdater(func(id string, downloaded, total int64, files, totalFiles int))
}
