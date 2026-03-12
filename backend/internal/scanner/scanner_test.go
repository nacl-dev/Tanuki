package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanMTimePrefersNewestSidecar(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "sample.jpg")
	if err := os.WriteFile(mediaPath, []byte("image"), 0o644); err != nil {
		t.Fatalf("write media: %v", err)
	}
	infoPath := mediaPath + ".info.json"
	if err := os.WriteFile(infoPath, []byte(`{"title":"Sample"}`), 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	mediaTime := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)
	sidecarTime := mediaTime.Add(2 * time.Minute)
	if err := os.Chtimes(mediaPath, mediaTime, mediaTime); err != nil {
		t.Fatalf("chtimes media: %v", err)
	}
	if err := os.Chtimes(infoPath, sidecarTime, sidecarTime); err != nil {
		t.Fatalf("chtimes sidecar: %v", err)
	}

	scanTime := (&Scanner{}).scanMTime(mediaPath, mediaTime)
	if !scanTime.Equal(normalizeScanTime(sidecarTime)) {
		t.Fatalf("expected latest sidecar mtime %v, got %v", sidecarTime, scanTime)
	}
}

func TestSameScanTimeNormalizesPrecision(t *testing.T) {
	t.Parallel()

	value := time.Date(2026, 3, 11, 10, 0, 0, 123456789, time.UTC)
	stored := normalizeScanTime(value)
	if !sameScanTime(&stored, value) {
		t.Fatalf("expected times to match after normalization")
	}
}

func TestGalleryDigestIncludesImageMetadata(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	img1 := filepath.Join(dir, "001.jpg")
	img2 := filepath.Join(dir, "002.png")
	txt := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(img1, []byte("a"), 0o644); err != nil {
		t.Fatalf("write image1: %v", err)
	}
	if err := os.WriteFile(img2, []byte("bc"), 0o644); err != nil {
		t.Fatalf("write image2: %v", err)
	}
	if err := os.WriteFile(txt, []byte("ignored"), 0o644); err != nil {
		t.Fatalf("write txt: %v", err)
	}

	total, digest, err := galleryDigest(dir)
	if err != nil {
		t.Fatalf("galleryDigest: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total size 3, got %d", total)
	}
	if len(strings.TrimSpace(digest)) == 0 {
		t.Fatalf("expected non-empty digest")
	}
}

func TestLoadImportMetadataReadsWorkFieldsFromTanukiSidecar(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "chapter-01.cbz")
	if err := os.WriteFile(mediaPath, []byte("archive"), 0o644); err != nil {
		t.Fatalf("write media: %v", err)
	}
	if err := os.WriteFile(mediaPath+".tanuki.json", []byte(`{
		"title":"Chapter 1",
		"work_title":"Example Series",
		"work_index":1,
		"source_url":"https://example.test/item/1",
		"tags":["artist:test"]
	}`), 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	meta, err := (&Scanner{}).loadImportMetadata(mediaPath)
	if err != nil {
		t.Fatalf("loadImportMetadata: %v", err)
	}
	if meta == nil {
		t.Fatalf("expected metadata")
	}
	if meta.WorkTitle != "Example Series" {
		t.Fatalf("expected work title, got %q", meta.WorkTitle)
	}
	if meta.WorkIndex != 1 {
		t.Fatalf("expected work index 1, got %d", meta.WorkIndex)
	}
}

func TestLoadImportMetadataReadsGalleryDLJSONSidecar(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "sample.jpg")
	if err := os.WriteFile(mediaPath, []byte("image"), 0o644); err != nil {
		t.Fatalf("write media: %v", err)
	}
	if err := os.WriteFile(mediaPath+".json", []byte(`{
		"title":"Sample Gallery",
		"page_url":"https://example.test/post/1",
		"url":"https://cdn.example.test/image/001.jpg",
		"category":"pixiv",
		"artist":["Example Artist"],
		"tags":["tag one","tag two"],
		"num":1,
		"filename":"sample",
		"extension":"jpg"
	}`), 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	meta, err := (&Scanner{}).loadImportMetadata(mediaPath)
	if err != nil {
		t.Fatalf("loadImportMetadata: %v", err)
	}
	if meta == nil {
		t.Fatalf("expected metadata")
	}
	if meta.Title != "Sample Gallery" {
		t.Fatalf("expected title, got %q", meta.Title)
	}
	if meta.WorkTitle != "Sample Gallery" {
		t.Fatalf("expected work title, got %q", meta.WorkTitle)
	}
	if meta.WorkIndex != 1 {
		t.Fatalf("expected work index 1, got %d", meta.WorkIndex)
	}
	if meta.SourceURL != "https://example.test/post/1" {
		t.Fatalf("expected source url, got %q", meta.SourceURL)
	}
}
