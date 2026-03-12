package models

import "strings"

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

func FormatTagExpression(name string, category TagCategory) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if category == TagCategoryGeneral {
		return name
	}
	return string(category) + ":" + name
}

func (t Tag) Expression() string {
	return FormatTagExpression(t.Name, t.Category)
}

// ParsedTag keeps a normalized representation of a raw tag input.
type ParsedTag struct {
	Raw        string
	Name       string
	Category   TagCategory
	Namespace  string
	Namespaced bool
}

// ParseTag normalizes free-form input and maps supported namespace prefixes to
// the closest Tanuki tag category.
func ParseTag(raw string) ParsedTag {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return ParsedTag{}
	}

	parsed := ParsedTag{
		Raw:      normalized,
		Name:     normalized,
		Category: TagCategoryGeneral,
	}

	namespace, value, ok := strings.Cut(normalized, ":")
	if !ok {
		return parsed
	}

	namespace = strings.TrimSpace(namespace)
	value = strings.TrimSpace(value)
	if value == "" {
		return parsed
	}

	category, mapped := TagCategoryForNamespace(namespace)
	if !mapped {
		return parsed
	}

	parsed.Name = value
	parsed.Category = category
	parsed.Namespace = namespace
	parsed.Namespaced = true
	return parsed
}

// ShouldPromoteTagCategory returns true when a more specific category should
// replace a weaker one for the same tag name.
func ShouldPromoteTagCategory(current, next TagCategory) bool {
	return tagCategorySpecificity(next) > tagCategorySpecificity(current)
}

func tagCategorySpecificity(category TagCategory) int {
	switch category {
	case TagCategoryArtist, TagCategoryCharacter, TagCategoryParody, TagCategoryGenre:
		return 2
	case TagCategoryMeta:
		return 1
	default:
		return 0
	}
}

func TagCategoryForNamespace(namespace string) (TagCategory, bool) {
	switch strings.TrimSpace(strings.ToLower(namespace)) {
	case "general", "tag", "tags":
		return TagCategoryGeneral, true
	case "artist", "artists", "author", "authors", "creator", "creators", "circle", "circles", "group", "groups":
		return TagCategoryArtist, true
	case "character", "characters", "char":
		return TagCategoryCharacter, true
	case "parody", "parodies", "copyright", "copyrights", "series", "franchise", "property":
		return TagCategoryParody, true
	case "genre", "genres", "male", "female", "mixed", "other", "species", "theme", "themes", "fetish", "fetishes", "category", "categories", "format", "formats":
		return TagCategoryGenre, true
	case "meta", "title", "page", "rating", "language", "lang", "source", "site", "uploader", "date":
		return TagCategoryMeta, true
	default:
		return "", false
	}
}
