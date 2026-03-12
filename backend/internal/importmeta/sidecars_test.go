package importmeta

import "testing"

func TestParseGalleryDLMetadata(t *testing.T) {
	t.Parallel()

	metadata, recognized, err := ParseGalleryDLMetadata([]byte(`{
		"title":"Sample Gallery",
		"page_url":"https://example.test/post/1",
		"url":"https://cdn.example.test/image/001.jpg",
		"category":"nhentai",
		"artist":["Example Artist"],
		"language":["english"],
		"tags":["tag one","tag two"],
		"num":1,
		"filename":"001",
		"extension":"jpg"
	}`))
	if err != nil {
		t.Fatalf("ParseGalleryDLMetadata returned error: %v", err)
	}
	if !recognized {
		t.Fatalf("expected gallery-dl metadata to be recognized")
	}
	if metadata == nil {
		t.Fatalf("expected metadata result")
	}
	if metadata.Title != "Sample Gallery" {
		t.Fatalf("Title = %q, want %q", metadata.Title, "Sample Gallery")
	}
	if metadata.WorkTitle != "Sample Gallery" {
		t.Fatalf("WorkTitle = %q, want %q", metadata.WorkTitle, "Sample Gallery")
	}
	if metadata.WorkIndex != 1 {
		t.Fatalf("WorkIndex = %d, want 1", metadata.WorkIndex)
	}
	if metadata.SourceURL != "https://example.test/post/1" {
		t.Fatalf("SourceURL = %q, want page URL", metadata.SourceURL)
	}
	expectedTags := map[string]bool{
		"tag one":               true,
		"tag two":               true,
		"artist:Example Artist": true,
		"language:english":      true,
		"site:nhentai":          true,
	}
	for _, tag := range metadata.Tags {
		delete(expectedTags, tag)
	}
	if len(expectedTags) != 0 {
		t.Fatalf("missing expected tags: %#v", expectedTags)
	}
}

func TestParseGalleryDLMetadataIgnoresUnrelatedJSON(t *testing.T) {
	t.Parallel()

	metadata, recognized, err := ParseGalleryDLMetadata([]byte(`{"foo":"bar","hello":"world"}`))
	if err != nil {
		t.Fatalf("ParseGalleryDLMetadata returned error: %v", err)
	}
	if recognized {
		t.Fatalf("expected unrelated json to be ignored, got %#v", metadata)
	}
}

func TestParseGalleryDLMetadataSupportsNestedTagsAndChapterFields(t *testing.T) {
	t.Parallel()

	metadata, recognized, err := ParseGalleryDLMetadata([]byte(`{
		"title":"Chapter 12 - Night Run",
		"series":"Example Saga",
		"chapter":"12",
		"page_url":"https://example.test/chapters/12",
		"thumbnail_url":"https://cdn.example.test/thumb.jpg",
		"extractor":"mangadex",
		"scanlator":["Night Team"],
		"translated_language":"de",
		"uploader":"Mirror User",
		"tags":{
			"artist":["Lead Artist"],
			"character":["Heroine"],
			"parody":["Example Saga"],
			"female":["glasses"],
			"misc":["bonus"]
		},
		"filename":"012",
		"extension":"jpg"
	}`))
	if err != nil {
		t.Fatalf("ParseGalleryDLMetadata returned error: %v", err)
	}
	if !recognized || metadata == nil {
		t.Fatalf("expected gallery-dl metadata to be recognized")
	}
	if metadata.WorkTitle != "Example Saga" {
		t.Fatalf("WorkTitle = %q, want %q", metadata.WorkTitle, "Example Saga")
	}
	if metadata.WorkIndex != 12 {
		t.Fatalf("WorkIndex = %d, want 12", metadata.WorkIndex)
	}
	if metadata.PosterURL != "https://cdn.example.test/thumb.jpg" {
		t.Fatalf("PosterURL = %q", metadata.PosterURL)
	}

	expectedTags := map[string]bool{
		"artist:Lead Artist":   true,
		"artist:Night Team":    true,
		"character:Heroine":    true,
		"series:Example Saga":  true,
		"genre:glasses":        true,
		"language:de":          true,
		"uploader:Mirror User": true,
		"site:mangadex":        true,
	}
	for _, tag := range metadata.Tags {
		delete(expectedTags, tag)
	}
	if len(expectedTags) != 0 {
		t.Fatalf("missing expected tags: %#v", expectedTags)
	}
}

func TestParseGalleryDLMetadataSupportsSeasonStudioAndCastFields(t *testing.T) {
	t.Parallel()

	metadata, recognized, err := ParseGalleryDLMetadata([]byte(`{
		"alt_title":"Episode 5 - Return",
		"show":"Example Show",
		"season":"Season 2",
		"episode":"5",
		"studio":["Studio Nova"],
		"cast":["Lead Model"],
		"fetishes":["latex"],
		"extractor_key":"customtube",
		"filename":"episode-5",
		"extension":"mp4"
	}`))
	if err != nil {
		t.Fatalf("ParseGalleryDLMetadata returned error: %v", err)
	}
	if !recognized || metadata == nil {
		t.Fatalf("expected gallery-dl metadata to be recognized")
	}
	if metadata.Title != "Episode 5 - Return" {
		t.Fatalf("Title = %q, want %q", metadata.Title, "Episode 5 - Return")
	}
	if metadata.WorkTitle != "Example Show" {
		t.Fatalf("WorkTitle = %q, want %q", metadata.WorkTitle, "Example Show")
	}
	if metadata.WorkIndex != 5 {
		t.Fatalf("WorkIndex = %d, want 5", metadata.WorkIndex)
	}

	expectedTags := map[string]bool{
		"artist:Studio Nova":   true,
		"character:Lead Model": true,
		"series:Example Show":  true,
		"series:Season 2":      true,
		"genre:latex":          true,
		"site:customtube":      true,
	}
	for _, tag := range metadata.Tags {
		delete(expectedTags, tag)
	}
	if len(expectedTags) != 0 {
		t.Fatalf("missing expected tags: %#v", expectedTags)
	}
}
