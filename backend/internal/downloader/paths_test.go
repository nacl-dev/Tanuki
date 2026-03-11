package downloader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeTargetDirectoryAllowsConfiguredRoots(t *testing.T) {
	downloadsRoot := t.TempDir()
	mediaRoot := t.TempDir()
	target := filepath.Join(mediaRoot, "library")

	result, err := NormalizeTargetDirectory(target, downloadsRoot, mediaRoot)
	if err != nil {
		t.Fatalf("expected path to be allowed, got error: %v", err)
	}
	if result != target {
		t.Fatalf("unexpected normalized path: %s", result)
	}
}

func TestNormalizeTargetDirectoryRejectsTraversalOutsideRoots(t *testing.T) {
	downloadsRoot := t.TempDir()
	mediaRoot := t.TempDir()
	outside := t.TempDir()

	_, err := NormalizeTargetDirectory(outside, downloadsRoot, mediaRoot)
	if err == nil {
		t.Fatal("expected outside path to be rejected")
	}
}

func TestResolveTargetDirectoryFallsBackToDownloads(t *testing.T) {
	downloadsRoot := t.TempDir()
	mediaRoot := t.TempDir()
	outside := t.TempDir()

	result := ResolveTargetDirectory(outside, downloadsRoot, mediaRoot)
	if result != downloadsRoot {
		t.Fatalf("expected fallback to downloads root, got %s", result)
	}
}

func TestNormalizeTargetDirectoryRejectsSymlinkEscape(t *testing.T) {
	downloadsRoot := t.TempDir()
	mediaRoot := t.TempDir()
	outside := t.TempDir()

	escape := filepath.Join(mediaRoot, "escape")
	if err := os.Symlink(outside, escape); err != nil {
		t.Skipf("symlink unsupported in this environment: %v", err)
	}

	target := filepath.Join(escape, "nested")
	if _, err := NormalizeTargetDirectory(target, downloadsRoot, mediaRoot); err == nil {
		t.Fatal("expected symlink escape target to be rejected")
	}
}

func TestNormalizeTargetDirectoryRejectsNestedSymlinkEscape(t *testing.T) {
	downloadsRoot := t.TempDir()
	mediaRoot := t.TempDir()
	outside := t.TempDir()

	escape := filepath.Join(mediaRoot, "escape")
	if err := os.Symlink(outside, escape); err != nil {
		t.Skipf("symlink unsupported in this environment: %v", err)
	}

	target := filepath.Join(escape, "nested", "child")
	if _, err := NormalizeTargetDirectory(target, downloadsRoot, mediaRoot); err == nil {
		t.Fatal("expected nested symlink escape target to be rejected")
	}
}

func TestNormalizeTargetDirectoryAllowsMissingPathInsideRoot(t *testing.T) {
	downloadsRoot := t.TempDir()
	mediaRoot := t.TempDir()
	target := filepath.Join(mediaRoot, "new", "nested", "path")

	result, err := NormalizeTargetDirectory(target, downloadsRoot, mediaRoot)
	if err != nil {
		t.Fatalf("expected missing path inside root to be allowed, got error: %v", err)
	}
	if result != target {
		t.Fatalf("unexpected normalized path: %s", result)
	}
}
