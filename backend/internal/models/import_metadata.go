package models

// ImportMetadata stores metadata discovered during download so the scanner can
// apply it when the file later appears in the library.
type ImportMetadata struct {
	Title     string   `json:"title,omitempty"`
	WorkTitle string   `json:"work_title,omitempty"`
	WorkIndex int      `json:"work_index,omitempty"`
	SourceURL string   `json:"source_url,omitempty"`
	PosterURL string   `json:"poster_url,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}
