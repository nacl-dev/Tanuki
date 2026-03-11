package scanner

import (
	"os"
	"path/filepath"
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
