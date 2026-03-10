// Package models contains sqlx-tagged structs that map to database tables.
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

// Media represents a single media item in the library.
type Media struct {
	ID        string     `db:"id"         json:"id"`
	Title     string     `db:"title"       json:"title"`
	Type      MediaType  `db:"type"        json:"type"`
	FilePath  string     `db:"file_path"   json:"file_path"`
	FileSize  int64      `db:"file_size"   json:"file_size"`
	Checksum  string     `db:"checksum"    json:"checksum"`
	Rating    int        `db:"rating"      json:"rating"`    // 0-5
	Favorite  bool       `db:"favorite"    json:"favorite"`
	ViewCount int        `db:"view_count"  json:"view_count"`
	Language      string     `db:"language"       json:"language"`
	SourceURL     string     `db:"source_url"     json:"source_url"`
	ThumbnailPath string     `db:"thumbnail_path" json:"thumbnail_path"`
	ReadProgress  int        `db:"read_progress"  json:"read_progress"`
	ReadTotal     int        `db:"read_total"     json:"read_total"`
	CreatedAt     time.Time  `db:"created_at"     json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"     json:"updated_at"`
	DeletedAt     *time.Time `db:"deleted_at"     json:"deleted_at,omitempty"`

	// Computed / joined fields (not DB columns)
	Tags []Tag `db:"-" json:"tags,omitempty"`
}
