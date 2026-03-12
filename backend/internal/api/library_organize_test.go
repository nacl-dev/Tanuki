package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOrganizeDirectoryMovesCompanionSidecars(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	inboxDir := t.TempDir()
	sourceDir := filepath.Join(inboxDir, "batch-a")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}

	sourceMediaPath := filepath.Join(sourceDir, "clip.mp4")
	if err := os.WriteFile(sourceMediaPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write media file: %v", err)
	}
	sourceSidecarPath := sourceMediaPath + ".tanuki.json"
	if err := os.WriteFile(sourceSidecarPath, []byte(`{"tags":["artist:Inbox"]}`), 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	handler := &LibraryHandler{mediaPath: mediaDir, inboxPath: inboxDir}
	stats, err := handler.organizeDirectory(sourceDir, "move", false)
	if err != nil {
		t.Fatalf("organizeDirectory returned error: %v", err)
	}
	if stats.Moved != 1 {
		t.Fatalf("expected moved count 1, got %d", stats.Moved)
	}
	if len(stats.Items) != 1 {
		t.Fatalf("expected one organized item, got %d", len(stats.Items))
	}

	targetMediaPath := stats.Items[0].TargetPath
	targetSidecarPath := targetMediaPath + ".tanuki.json"
	if _, err := os.Stat(targetMediaPath); err != nil {
		t.Fatalf("expected organized media file: %v", err)
	}
	if _, err := os.Stat(targetSidecarPath); err != nil {
		t.Fatalf("expected companion sidecar at target: %v", err)
	}
	if _, err := os.Stat(sourceSidecarPath); !os.IsNotExist(err) {
		t.Fatalf("expected source sidecar to be moved away, stat err=%v", err)
	}
}

func TestOrganizeDirectoryMovesLargeImageFolderAsGallery(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	inboxDir := t.TempDir()
	sourceDir := filepath.Join(inboxDir, "gallery-batch")
	galleryDir := filepath.Join(sourceDir, "Sample Gallery")
	if err := os.MkdirAll(galleryDir, 0o755); err != nil {
		t.Fatalf("mkdir gallery dir: %v", err)
	}

	for i := 1; i <= 20; i++ {
		name := filepath.Join(galleryDir, "page-"+string(rune('A'+(i-1)%26))+".jpg")
		if err := os.WriteFile(name, []byte("img"), 0o644); err != nil {
			t.Fatalf("write gallery file %d: %v", i, err)
		}
	}

	handler := &LibraryHandler{mediaPath: mediaDir, inboxPath: inboxDir}
	stats, err := handler.organizeDirectory(sourceDir, "move", false)
	if err != nil {
		t.Fatalf("organizeDirectory returned error: %v", err)
	}
	if stats.Moved != 1 {
		t.Fatalf("expected moved count 1, got %d", stats.Moved)
	}
	if len(stats.Items) != 1 {
		t.Fatalf("expected one organized item, got %d", len(stats.Items))
	}
	if stats.Items[0].MediaType != "doujinshi" {
		t.Fatalf("expected doujinshi media type, got %q", stats.Items[0].MediaType)
	}

	targetDir := stats.Items[0].TargetPath
	if info, err := os.Stat(targetDir); err != nil || !info.IsDir() {
		t.Fatalf("expected target gallery dir, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "page-A.jpg")); err != nil {
		t.Fatalf("expected gallery image in target dir: %v", err)
	}
	if _, err := os.Stat(galleryDir); !os.IsNotExist(err) {
		t.Fatalf("expected source gallery dir to be moved away, stat err=%v", err)
	}
}
