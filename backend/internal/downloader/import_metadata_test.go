package downloader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nacl-dev/tanuki/internal/models"
)

func TestWriteImportMetadataInfersWorkMetadataFromTitle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "sample.mp4")
	if err := os.WriteFile(mediaPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write media: %v", err)
	}

	if err := writeImportMetadata(mediaPath, models.ImportMetadata{Title: "Sample Series - 02"}); err != nil {
		t.Fatalf("writeImportMetadata returned error: %v", err)
	}

	meta := readImportMetadata(mediaPath)
	if meta == nil {
		t.Fatal("expected metadata sidecar")
	}
	if meta.WorkTitle != "Sample Series" {
		t.Fatalf("WorkTitle = %q, want %q", meta.WorkTitle, "Sample Series")
	}
	if meta.WorkIndex != 2 {
		t.Fatalf("WorkIndex = %d, want 2", meta.WorkIndex)
	}
}

func TestWriteImportMetadataKeepsExplicitWorkMetadata(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "sample.mp4")
	if err := os.WriteFile(mediaPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write media: %v", err)
	}

	if err := writeImportMetadata(mediaPath, models.ImportMetadata{
		Title:     "Ignored Series - 03",
		WorkTitle: "Manual Series",
		WorkIndex: 7,
	}); err != nil {
		t.Fatalf("writeImportMetadata returned error: %v", err)
	}

	meta := readImportMetadata(mediaPath)
	if meta == nil {
		t.Fatal("expected metadata sidecar")
	}
	if meta.WorkTitle != "Manual Series" {
		t.Fatalf("WorkTitle = %q, want %q", meta.WorkTitle, "Manual Series")
	}
	if meta.WorkIndex != 7 {
		t.Fatalf("WorkIndex = %d, want 7", meta.WorkIndex)
	}
}

func TestWriteImportMetadataInfersWorkMetadataFromChapterTitle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "chapter.mp4")
	if err := os.WriteFile(mediaPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write media: %v", err)
	}

	if err := writeImportMetadata(mediaPath, models.ImportMetadata{Title: "Sample Saga Chapter 12 - Reunion"}); err != nil {
		t.Fatalf("writeImportMetadata returned error: %v", err)
	}

	meta := readImportMetadata(mediaPath)
	if meta == nil {
		t.Fatal("expected metadata sidecar")
	}
	if meta.WorkTitle != "Sample Saga" {
		t.Fatalf("WorkTitle = %q, want %q", meta.WorkTitle, "Sample Saga")
	}
	if meta.WorkIndex != 12 {
		t.Fatalf("WorkIndex = %d, want 12", meta.WorkIndex)
	}
}

func TestWriteImportMetadataInfersWorkMetadataFromVolumeChapterTitle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "volume-chapter.mp4")
	if err := os.WriteFile(mediaPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write media: %v", err)
	}

	if err := writeImportMetadata(mediaPath, models.ImportMetadata{Title: "Sample Saga Vol. 2 Ch. 14"}); err != nil {
		t.Fatalf("writeImportMetadata returned error: %v", err)
	}

	meta := readImportMetadata(mediaPath)
	if meta == nil {
		t.Fatal("expected metadata sidecar")
	}
	if meta.WorkTitle != "Sample Saga" {
		t.Fatalf("WorkTitle = %q, want %q", meta.WorkTitle, "Sample Saga")
	}
	if meta.WorkIndex != 14 {
		t.Fatalf("WorkIndex = %d, want 14", meta.WorkIndex)
	}
}

func TestWriteImportMetadataInfersWorkMetadataFromSeasonEpisodeTitle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "season-episode.mp4")
	if err := os.WriteFile(mediaPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write media: %v", err)
	}

	if err := writeImportMetadata(mediaPath, models.ImportMetadata{Title: "Sample Saga S02E05 - Return"}); err != nil {
		t.Fatalf("writeImportMetadata returned error: %v", err)
	}

	meta := readImportMetadata(mediaPath)
	if meta == nil {
		t.Fatal("expected metadata sidecar")
	}
	if meta.WorkTitle != "Sample Saga" {
		t.Fatalf("WorkTitle = %q, want %q", meta.WorkTitle, "Sample Saga")
	}
	if meta.WorkIndex != 5 {
		t.Fatalf("WorkIndex = %d, want 5", meta.WorkIndex)
	}
}

func TestWriteImportMetadataInfersWorkMetadataFromReleaseVersionTitle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "release-version.mp4")
	if err := os.WriteFile(mediaPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write media: %v", err)
	}

	if err := writeImportMetadata(mediaPath, models.ImportMetadata{Title: "[Studio] Sample Saga 03v2"}); err != nil {
		t.Fatalf("writeImportMetadata returned error: %v", err)
	}

	meta := readImportMetadata(mediaPath)
	if meta == nil {
		t.Fatal("expected metadata sidecar")
	}
	if meta.WorkTitle != "Sample Saga" {
		t.Fatalf("WorkTitle = %q, want %q", meta.WorkTitle, "Sample Saga")
	}
	if meta.WorkIndex != 3 {
		t.Fatalf("WorkIndex = %d, want 3", meta.WorkIndex)
	}
}

func TestWriteImportMetadataInfersHentaiStyleWorkTitleWithoutEpisode(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "doujin.cbz")
	if err := os.WriteFile(mediaPath, []byte("archive"), 0o644); err != nil {
		t.Fatalf("write media: %v", err)
	}

	if err := writeImportMetadata(mediaPath, models.ImportMetadata{
		Title: "[Circle (Artist)] Secret Lesson (Naruto) [English] [Decensored]",
	}); err != nil {
		t.Fatalf("writeImportMetadata returned error: %v", err)
	}

	meta := readImportMetadata(mediaPath)
	if meta == nil {
		t.Fatal("expected metadata sidecar")
	}
	if meta.WorkTitle != "Secret Lesson" {
		t.Fatalf("WorkTitle = %q, want %q", meta.WorkTitle, "Secret Lesson")
	}
	if meta.WorkIndex != 0 {
		t.Fatalf("WorkIndex = %d, want 0", meta.WorkIndex)
	}
}

func TestOrganizeDownloadedFilesAddsGroupedWorkMetadata(t *testing.T) {
	t.Parallel()

	stagingDir := t.TempDir()
	targetDir := t.TempDir()
	workDir := filepath.Join(stagingDir, "Sample Gallery")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir work dir: %v", err)
	}

	firstPath := filepath.Join(workDir, "001.jpg")
	secondPath := filepath.Join(workDir, "002.jpg")
	for _, path := range []string{firstPath, secondPath} {
		if err := os.WriteFile(path, []byte("image"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	if err := writeImportMetadata(firstPath, models.ImportMetadata{Title: "Cover", Tags: []string{"artist:Example"}}); err != nil {
		t.Fatalf("write initial sidecar: %v", err)
	}

	moved, err := organizeDownloadedFiles(stagingDir, targetDir)
	if err != nil {
		t.Fatalf("organizeDownloadedFiles returned error: %v", err)
	}
	if len(moved) != 2 {
		t.Fatalf("moved %d files, want 2", len(moved))
	}

	firstMeta := readImportMetadata(moved[0])
	secondMeta := readImportMetadata(moved[1])
	if firstMeta == nil || secondMeta == nil {
		t.Fatal("expected metadata sidecars on organized files")
	}
	if firstMeta.WorkTitle != "Sample Gallery" || secondMeta.WorkTitle != "Sample Gallery" {
		t.Fatalf("unexpected work titles: %#v %#v", firstMeta.WorkTitle, secondMeta.WorkTitle)
	}

	workIndexes := map[int]bool{
		firstMeta.WorkIndex:  true,
		secondMeta.WorkIndex: true,
	}
	if !workIndexes[1] || !workIndexes[2] {
		t.Fatalf("unexpected work indexes: %#v %#v", firstMeta.WorkIndex, secondMeta.WorkIndex)
	}
	if firstMeta.Title != "Cover" {
		t.Fatalf("Title = %q, want %q", firstMeta.Title, "Cover")
	}
	if len(firstMeta.Tags) != 1 || firstMeta.Tags[0] != "artist:Example" {
		t.Fatalf("Tags = %#v, want artist tag preserved", firstMeta.Tags)
	}
}

func TestOrganizeDownloadedFilesConvertsGalleryDLJSONSidecar(t *testing.T) {
	t.Parallel()

	stagingDir := t.TempDir()
	targetDir := t.TempDir()
	sourcePath := filepath.Join(stagingDir, "sample.jpg")
	if err := os.WriteFile(sourcePath, []byte("image"), 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}
	if err := os.WriteFile(sourcePath+".json", []byte(`{
		"title":"Sample Gallery",
		"page_url":"https://example.test/post/1",
		"url":"https://cdn.example.test/image/001.jpg",
		"category":"nhentai",
		"artist":["Example Artist"],
		"language":["english"],
		"tags":["tag one"],
		"num":1,
		"filename":"sample",
		"extension":"jpg"
	}`), 0o644); err != nil {
		t.Fatalf("write gallery-dl sidecar: %v", err)
	}

	moved, err := organizeDownloadedFiles(stagingDir, targetDir)
	if err != nil {
		t.Fatalf("organizeDownloadedFiles returned error: %v", err)
	}
	if len(moved) != 1 {
		t.Fatalf("moved %d files, want 1", len(moved))
	}

	meta := readImportMetadata(moved[0])
	if meta == nil {
		t.Fatal("expected converted tanuki sidecar")
	}
	if meta.Title != "Sample Gallery" {
		t.Fatalf("Title = %q, want %q", meta.Title, "Sample Gallery")
	}
	if meta.WorkTitle != "Sample Gallery" {
		t.Fatalf("WorkTitle = %q, want %q", meta.WorkTitle, "Sample Gallery")
	}
	if meta.WorkIndex != 1 {
		t.Fatalf("WorkIndex = %d, want 1", meta.WorkIndex)
	}
	if meta.SourceURL != "https://example.test/post/1" {
		t.Fatalf("SourceURL = %q, want page URL", meta.SourceURL)
	}
	expectedTags := map[string]bool{
		"tag one":               true,
		"artist:Example Artist": true,
		"language:english":      true,
		"site:nhentai":          true,
	}
	for _, tag := range meta.Tags {
		delete(expectedTags, tag)
	}
	if len(expectedTags) != 0 {
		t.Fatalf("missing expected tags: %#v", expectedTags)
	}
	if _, err := os.Stat(moved[0] + ".tanuki.json"); err != nil {
		t.Fatalf("expected tanuki sidecar at destination: %v", err)
	}
}
