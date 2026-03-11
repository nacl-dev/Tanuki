package api

import (
	"path/filepath"
	"testing"
)

func TestEnsureManagedPathAllowsPathInsideRoot(t *testing.T) {
	t.Parallel()

	root := filepath.Clean(filepath.Join("C:\\", "media"))
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

	root := filepath.Clean(filepath.Join("C:\\", "media"))
	path := filepath.Clean(filepath.Join("C:\\", "other", "sample.mp4"))

	if _, err := ensureManagedPath(path, root); err == nil {
		t.Fatalf("expected outside path to be rejected")
	}
}
