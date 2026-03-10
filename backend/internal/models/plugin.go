package models

import "time"

// Plugin represents an installed community plugin.
type Plugin struct {
	ID         string    `db:"id"          json:"id"`
	Name       string    `db:"name"        json:"name"`
	SourceName string    `db:"source_name" json:"source_name"`
	SourceURL  string    `db:"source_url"  json:"source_url"`
	FilePath   string    `db:"file_path"   json:"file_path"`
	Enabled    bool      `db:"enabled"     json:"enabled"`
	Version    string    `db:"version"     json:"version"`
	CreatedAt  time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"  json:"updated_at"`
}
