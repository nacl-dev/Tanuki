package models

import (
	"time"
)

// MediaType represents the kind of media stored.
type MediaType string

const (
	MediaTypeVideo     MediaType = "video"
	MediaTypeImage     MediaType = "image"
	MediaTypeManga     MediaType = "manga"
	MediaTypeComic     MediaType = "comic"
	MediaTypeDoujinshi MediaType = "doujinshi"
)

// AutoTagStatus represents the auto-tagging state of a media item.
type AutoTagStatus string

const (
	AutoTagStatusPending    AutoTagStatus = "pending"
	AutoTagStatusProcessing AutoTagStatus = "processing"
	AutoTagStatusCompleted  AutoTagStatus = "completed"
	AutoTagStatusFailed     AutoTagStatus = "failed"
	AutoTagStatusSkipped    AutoTagStatus = "skipped"
)

// Media represents a single media item in the library.
type Media struct {
	ID                string          `db:"id"                  json:"id"`
	OwnerID           *string         `db:"owner_id"            json:"-"`
	Title             string          `db:"title"               json:"title"`
	Type              MediaType       `db:"type"                json:"type"`
	FilePath          string          `db:"file_path"           json:"file_path"`
	FileSize          int64           `db:"file_size"           json:"file_size"`
	Checksum          string          `db:"checksum"            json:"checksum"`
	Rating            int             `db:"rating"              json:"rating"`
	Favorite          bool            `db:"favorite"            json:"favorite"`
	ViewCount         int             `db:"view_count"          json:"view_count"`
	Language          string          `db:"language"            json:"language"`
	SourceURL         string          `db:"source_url"          json:"source_url"`
	ThumbnailPath     string          `db:"thumbnail_path"      json:"thumbnail_path"`
	ReadProgress      int             `db:"read_progress"       json:"read_progress"`
	ReadTotal         int             `db:"read_total"          json:"read_total"`
	CreatedAt         time.Time       `db:"created_at"          json:"created_at"`
	UpdatedAt         time.Time       `db:"updated_at"          json:"updated_at"`
	DeletedAt         *time.Time      `db:"deleted_at"          json:"deleted_at,omitempty"`
	ScanMTime         *time.Time      `db:"scan_mtime"          json:"scan_mtime,omitempty"`
	AutoTagStatus     AutoTagStatus   `db:"auto_tag_status"     json:"auto_tag_status"`
	AutoTagSource     string          `db:"auto_tag_source"     json:"auto_tag_source,omitempty"`
	AutoTagSimilarity float32         `db:"auto_tag_similarity" json:"auto_tag_similarity,omitempty"`
	AutoTaggedAt      *time.Time      `db:"auto_tagged_at"      json:"auto_tagged_at,omitempty"`
	PHash             *int64          `db:"phash"               json:"phash,omitempty"`
	PHashComputedAt   *time.Time      `db:"phash_computed_at"   json:"phash_computed_at,omitempty"`
	Tags              []Tag           `db:"-"                   json:"tags,omitempty"`
	Collections       []CollectionRef `db:"-"                 json:"collections,omitempty"`
}

type CollectionRef struct {
	ID   string `db:"id"   json:"id"`
	Name string `db:"name" json:"name"`
}
