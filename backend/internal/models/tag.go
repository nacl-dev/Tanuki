package models

// TagCategory classifies a tag into a semantic namespace.
type TagCategory string

const (
	TagCategoryGeneral   TagCategory = "general"
	TagCategoryArtist    TagCategory = "artist"
	TagCategoryCharacter TagCategory = "character"
	TagCategoryParody    TagCategory = "parody"
	TagCategoryGenre     TagCategory = "genre"
	TagCategoryMeta      TagCategory = "meta"
)

// Tag represents a single tag that can be applied to media items.
type Tag struct {
	ID         string      `db:"id"          json:"id"`
	Name       string      `db:"name"        json:"name"`
	Category   TagCategory `db:"category"    json:"category"`
	UsageCount int         `db:"usage_count" json:"usage_count"`
}
