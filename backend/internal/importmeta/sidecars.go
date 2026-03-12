package importmeta

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nacl-dev/tanuki/internal/models"
)

func CandidatePaths(mediaPath string) []string {
	return []string{
		mediaPath + ".tanuki.json",
		mediaPath + ".info.json",
		mediaPath + ".json",
	}
}

func LoadMedia(mediaPath string) (*models.ImportMetadata, error) {
	for _, path := range CandidatePaths(mediaPath) {
		metadata, recognized, err := LoadCompanion(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if recognized {
			return metadata, nil
		}
	}
	return nil, nil
}

func LoadCompanion(path string) (*models.ImportMetadata, bool, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	return ParseCompanion(path, body)
}

func ParseCompanion(path string, body []byte) (*models.ImportMetadata, bool, error) {
	lower := strings.ToLower(filepath.Base(path))
	switch {
	case strings.HasSuffix(lower, ".tanuki.json"):
		metadata, err := parseTanukiMetadata(body)
		return metadata, true, err
	case strings.HasSuffix(lower, ".info.json"):
		metadata, err := parseInfoMetadata(body)
		return metadata, true, err
	case strings.HasSuffix(lower, ".json"):
		return parseGalleryDLMetadata(body)
	default:
		return nil, false, nil
	}
}

func ParseGalleryDLMetadata(body []byte) (*models.ImportMetadata, bool, error) {
	return parseGalleryDLMetadata(body)
}

func parseTanukiMetadata(body []byte) (*models.ImportMetadata, error) {
	var metadata models.ImportMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return nil, err
	}
	metadata.Title = strings.TrimSpace(metadata.Title)
	metadata.WorkTitle = strings.TrimSpace(metadata.WorkTitle)
	metadata.SourceURL = strings.TrimSpace(metadata.SourceURL)
	metadata.PosterURL = strings.TrimSpace(metadata.PosterURL)
	metadata.Tags = compactStrings(metadata.Tags)
	if metadata.WorkIndex < 0 {
		metadata.WorkIndex = 0
	}
	return &metadata, nil
}

func parseInfoMetadata(body []byte) (*models.ImportMetadata, error) {
	var payload struct {
		Title       string   `json:"title"`
		WorkTitle   string   `json:"work_title"`
		WorkIndex   int      `json:"work_index"`
		WebpageURL  string   `json:"webpage_url"`
		OriginalURL string   `json:"original_url"`
		Thumbnail   string   `json:"thumbnail"`
		Tags        []string `json:"tags"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	sourceURL := strings.TrimSpace(payload.WebpageURL)
	if sourceURL == "" {
		sourceURL = strings.TrimSpace(payload.OriginalURL)
	}

	return &models.ImportMetadata{
		Title:     strings.TrimSpace(payload.Title),
		WorkTitle: strings.TrimSpace(payload.WorkTitle),
		WorkIndex: payload.WorkIndex,
		SourceURL: sourceURL,
		PosterURL: strings.TrimSpace(payload.Thumbnail),
		Tags:      compactStrings(payload.Tags),
	}, nil
}

func parseGalleryDLMetadata(body []byte) (*models.ImportMetadata, bool, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false, err
	}
	if !looksLikeGalleryDLMetadata(payload) {
		return nil, false, nil
	}

	title := firstNonEmptyString(
		stringValue(payload["title"]),
		stringValue(payload["alt_title"]),
		stringValue(payload["episode_title"]),
		stringValue(payload["chapter_title"]),
		stringValue(payload["scene_title"]),
		stringValue(payload["id"]),
	)

	workIndex := firstPositiveInt(
		intValue(payload["work_index"]),
		intValue(payload["episode"]),
		intValue(payload["chapter"]),
		intValue(payload["chapter_number"]),
		intValue(payload["chapter_id"]),
		intValue(payload["scene"]),
		intValue(payload["scene_number"]),
		intValue(payload["season_episode"]),
		intValue(payload["episode_id"]),
		intValue(payload["volume"]),
		intValue(payload["volume_number"]),
		intValue(payload["season_number"]),
		intValue(payload["episode_number"]),
		intValue(payload["page_number"]),
		intValue(payload["page"]),
		intValue(payload["num"]),
	)
	workTitle := strings.TrimSpace(stringValue(payload["work_title"]))
	if workTitle == "" && workIndex > 0 {
		workTitle = firstNonEmptyString(
			stringValue(payload["series"]),
			stringValue(payload["show"]),
			stringValue(payload["show_title"]),
			stringValue(payload["season"]),
			stringValue(payload["book_title"]),
			stringValue(payload["manga_title"]),
			stringValue(payload["parody"]),
			stringValue(payload["manga"]),
			stringValue(payload["gallery_title"]),
			stringValue(payload["collection_title"]),
			stringValue(payload["album"]),
			stringValue(payload["franchise"]),
			title,
			stringValue(payload["chapter_title"]),
		)
	}

	sourceURL := firstNonEmptyString(
		stringValue(payload["page_url"]),
		stringValue(payload["post_url"]),
		stringValue(payload["gallery_url"]),
		stringValue(payload["webpage_url"]),
		stringValue(payload["original_url"]),
		stringValue(payload["source_url"]),
		nestedStringValue(payload["parent"], "url"),
	)
	if sourceURL == "" {
		url := stringValue(payload["url"])
		if url != "" && !looksLikeDirectMediaURL(url) {
			sourceURL = url
		}
	}

	posterURL := firstNonEmptyString(
		stringValue(payload["thumbnail"]),
		stringValue(payload["thumbnail_url"]),
		stringValue(payload["cover"]),
		stringValue(payload["cover_url"]),
		stringValue(payload["preview_url"]),
	)

	tags := buildGalleryDLTags(payload)
	metadata := &models.ImportMetadata{
		Title:     title,
		WorkTitle: strings.TrimSpace(workTitle),
		WorkIndex: workIndex,
		SourceURL: sourceURL,
		PosterURL: posterURL,
		Tags:      compactStrings(tags),
	}
	return metadata, true, nil
}

func looksLikeGalleryDLMetadata(payload map[string]interface{}) bool {
	if payload == nil {
		return false
	}
	recognizedMarkers := []string{
		"category", "subcategory", "filename", "extension", "gallery_id",
		"page_url", "post_url", "gallery_url", "num", "count",
	}
	hasMarker := false
	for _, key := range recognizedMarkers {
		if _, ok := payload[key]; ok {
			hasMarker = true
			break
		}
	}
	if !hasMarker {
		return false
	}
	return stringValue(payload["title"]) != "" ||
		stringValue(payload["url"]) != "" ||
		stringValue(payload["filename"]) != ""
}

func buildGalleryDLTags(payload map[string]interface{}) []string {
	tags := make([]string, 0, 24)
	tags = append(tags, stringsFromValue(payload["tags"])...)
	tags = append(tags, tagsFromNestedTagMap(payload["tags"])...)

	for _, key := range []string{"artist", "artists", "author", "authors", "creator", "creators", "circle", "circles", "group", "groups", "studio", "studios", "brand", "brands"} {
		tags = append(tags, qualifyTags("artist", stringsFromValue(payload[key]))...)
	}
	for _, key := range []string{"performer", "performers", "actor", "actors", "cast", "models"} {
		tags = append(tags, qualifyTags("character", stringsFromValue(payload[key]))...)
	}
	for _, key := range []string{"character", "characters"} {
		tags = append(tags, qualifyTags("character", stringsFromValue(payload[key]))...)
	}
	for _, key := range []string{"parody", "parodies", "series", "copyright", "copyrights", "franchise", "show", "show_title", "season"} {
		tags = append(tags, qualifyTags("series", stringsFromValue(payload[key]))...)
	}
	for _, key := range []string{"genre", "genres", "categories", "theme", "themes", "male", "female", "fetish", "fetishes"} {
		tags = append(tags, qualifyTags("genre", stringsFromValue(payload[key]))...)
	}
	for _, key := range []string{"category", "subcategory"} {
		tags = append(tags, qualifyTags("site", stringsFromValue(payload[key]))...)
	}
	for _, key := range []string{"language", "languages"} {
		tags = append(tags, qualifyTags("language", stringsFromValue(payload[key]))...)
	}
	for _, key := range []string{"lang", "translated_language"} {
		tags = append(tags, qualifyTags("language", stringsFromValue(payload[key]))...)
	}
	for _, key := range []string{"rating", "type"} {
		tags = append(tags, qualifyTags("meta", stringsFromValue(payload[key]))...)
	}
	for _, key := range []string{"uploader", "uploaders", "posted_by"} {
		tags = append(tags, qualifyTags("uploader", stringsFromValue(payload[key]))...)
	}
	for _, key := range []string{"scanlator", "scanlators"} {
		tags = append(tags, qualifyTags("artist", stringsFromValue(payload[key]))...)
	}
	for _, key := range []string{"site", "source", "sources", "extractor", "extractor_key"} {
		tags = append(tags, qualifyTags("site", stringsFromValue(payload[key]))...)
	}
	for _, key := range []string{"publisher", "publishers", "magazine"} {
		tags = append(tags, qualifyTags("meta", stringsFromValue(payload[key]))...)
	}

	for key, value := range payload {
		lower := strings.ToLower(strings.TrimSpace(key))
		if !strings.HasPrefix(lower, "tags_") {
			continue
		}
		namespace := strings.TrimPrefix(lower, "tags_")
		namespace = galleryDLNamespace(namespace)
		tags = append(tags, qualifyTags(namespace, stringsFromValue(value))...)
	}

	return compactStrings(tags)
}

func tagsFromNestedTagMap(value interface{}) []string {
	object, ok := value.(map[string]interface{})
	if !ok || object == nil {
		return nil
	}

	tags := make([]string, 0, len(object)*2)
	for key, nested := range object {
		namespace := galleryDLNamespace(key)
		tags = append(tags, qualifyTags(namespace, stringsFromValue(nested))...)
	}
	return compactStrings(tags)
}

func galleryDLNamespace(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "artist", "artists", "author", "authors", "creator", "creators", "circle", "circles", "group", "groups":
		return "artist"
	case "studio", "studios", "brand", "brands":
		return "artist"
	case "performer", "performers", "actor", "actors", "cast", "models":
		return "character"
	case "character", "characters":
		return "character"
	case "parody", "parodies", "series", "copyright", "copyrights", "franchise", "show", "show_title", "season":
		return "series"
	case "genre", "genres", "categories", "theme", "themes", "male", "female", "fetish", "fetishes":
		return "genre"
	case "category", "subcategory":
		return "site"
	case "language", "languages":
		return "language"
	case "rating", "type":
		return "meta"
	default:
		return ""
	}
}

func qualifyTags(namespace string, values []string) []string {
	namespace = strings.TrimSpace(strings.ToLower(namespace))
	if namespace == "" {
		return compactStrings(values)
	}

	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, namespace+":"+value)
	}
	return compactStrings(out)
}

func nestedStringValue(value interface{}, key string) string {
	object, ok := value.(map[string]interface{})
	if !ok || object == nil {
		return ""
	}
	return stringValue(object[key])
}

func stringValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return strings.TrimSpace(v.String())
	default:
		return ""
	}
}

func stringsFromValue(value interface{}) []string {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		return compactStrings(splitLooseString(v))
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if text := stringValue(item); text != "" {
				out = append(out, text)
			}
		}
		return compactStrings(out)
	case []string:
		return compactStrings(v)
	default:
		return nil
	}
}

func splitLooseString(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.Contains(value, ",") {
		parts := strings.Split(value, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	}
	return []string{value}
}

func intValue(value interface{}) int {
	switch v := value.(type) {
	case float64:
		if v > 0 {
			return int(v)
		}
	case int:
		if v > 0 {
			return v
		}
	case int64:
		if v > 0 {
			return int(v)
		}
	case json.Number:
		if parsed, err := strconv.Atoi(v.String()); err == nil && parsed > 0 {
			return parsed
		}
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && parsed > 0 {
			return parsed
		}
	}
	return 0
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func looksLikeDirectMediaURL(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	for _, suffix := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".mp4", ".webm", ".mkv", ".mov", ".zip", ".cbz"} {
		if strings.Contains(lower, suffix) {
			return true
		}
	}
	return false
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}
