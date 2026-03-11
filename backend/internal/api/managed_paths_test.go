package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureManagedPathAllowsPathInsideRoot(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "media")
	if err := os.MkdirAll(filepath.Join(root, "videos"), 0o755); err != nil {
		t.Fatalf("create media root: %v", err)
	}
	path := filepath.Join(root, "videos", "sample.mp4")

	got, err := ensureManagedPath(path, root)
	if err != nil {
		t.Fatalf("expected path to be allowed, got error: %v", err)
	}
	if got != filepath.Clean(path) {
		t.Fatalf("unexpected cleaned path: %s", got)
	}
}

func TestEnsureManagedPathRejectsPathOutsideRoot(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	root := filepath.Join(base, "media")
	path := filepath.Join(base, "other", "sample.mp4")

	if _, err := ensureManagedPath(path, root); err == nil {
		t.Fatalf("expected outside path to be rejected")
	}
}

func TestEnsureManagedPathRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()

	targetFile := filepath.Join(outside, "sample.mp4")
	if err := os.WriteFile(targetFile, []byte("video"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	escapeDir := filepath.Join(root, "escape")
	if err := os.Symlink(outside, escapeDir); err != nil {
		t.Skipf("symlink unsupported in this environment: %v", err)
	}

	path := filepath.Join(escapeDir, "sample.mp4")
	if _, err := ensureManagedPath(path, root); err == nil {
		t.Fatalf("expected symlink escape path to be rejected")
	}
}
