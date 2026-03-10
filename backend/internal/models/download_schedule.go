package models

import (
	"encoding/json"
	"time"
)

// DownloadSchedule defines a recurring download triggered by a cron expression.
type DownloadSchedule struct {
	ID              string           `db:"id"               json:"id"`
	UserID          *string          `db:"user_id"          json:"user_id,omitempty"`
	Name            string           `db:"name"             json:"name"`
	URLPattern      string           `db:"url_pattern"      json:"url_pattern"`
	SourceType      string           `db:"source_type"      json:"source_type"`
	CronExpression  string           `db:"cron_expression"  json:"cron_expression"`
	Enabled         bool             `db:"enabled"          json:"enabled"`
	DefaultTags     *json.RawMessage `db:"default_tags"     json:"default_tags,omitempty"`
	TargetDirectory string           `db:"target_directory" json:"target_directory"`
	LastRun         *time.Time       `db:"last_run"         json:"last_run,omitempty"`
	NextRun         *time.Time       `db:"next_run"         json:"next_run,omitempty"`
	CreatedAt       time.Time        `db:"created_at"       json:"created_at"`
	UpdatedAt       time.Time        `db:"updated_at"       json:"updated_at"`
}
