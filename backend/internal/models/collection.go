package models

import "time"

type Collection struct {
	ID             string    `db:"id"               json:"id"`
	UserID         *string   `db:"user_id"          json:"user_id,omitempty"`
	Name           string    `db:"name"             json:"name"`
	Description    string    `db:"description"      json:"description"`
	CoverImagePath string    `db:"cover_image_path" json:"cover_image_path"`
	CreatedAt      time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"  json:"updated_at"`
	ItemCount      int       `db:"item_count"  json:"item_count"`
	Items          []Media   `db:"-"           json:"items,omitempty"`
}
