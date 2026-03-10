package models

// Collection groups related media items together (e.g., a series or folder).
type Collection struct {
	ID             string `db:"id"               json:"id"`
	Name           string `db:"name"             json:"name"`
	Description    string `db:"description"      json:"description"`
	CoverImagePath string `db:"cover_image_path" json:"cover_image_path"`
}
