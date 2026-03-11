package downloader

import (
	"fmt"
	"path/filepath"
	"strings"
)

// NormalizeTargetDirectory validates an optional user-provided target directory
// against the configured download/media roots.
func NormalizeTargetDirectory(targetDirectory, downloadsDir, mediaPath string) (string, error) {
	targetDirectory = strings.TrimSpace(targetDirectory)
	if targetDirectory == "" {
		return "", nil
	}
	if !filepath.IsAbs(targetDirectory) {
		return "", fmt.Errorf("target_directory must be an absolute path")
	}

	cleaned := filepath.Clean(targetDirectory)
	for _, root := range allowedTargetRoots(downloadsDir, mediaPath) {
		if isWithinRoot(cleaned, root) {
			return cleaned, nil
		}
	}

	return "", fmt.Errorf("target_directory must stay within the configured media/download roots")
}

// ResolveTargetDirectory defensively resolves a runtime target path even if a
// stored job contains an invalid directory.
func ResolveTargetDirectory(targetDirectory, downloadsDir, mediaPath string) string {
	if normalized, err := NormalizeTargetDirectory(targetDirectory, downloadsDir, mediaPath); err == nil && normalized != "" {
		return normalized
	}
	if normalized, err := NormalizeTargetDirectory(downloadsDir, downloadsDir, mediaPath); err == nil && normalized != "" {
		return normalized
	}
	if normalized, err := NormalizeTargetDirectory(mediaPath, downloadsDir, mediaPath); err == nil && normalized != "" {
		return normalized
	}
	if strings.TrimSpace(downloadsDir) != "" {
		return filepath.Clean(downloadsDir)
	}
	if strings.TrimSpace(mediaPath) != "" {
		return filepath.Clean(mediaPath)
	}
	return "/downloads"
}

func allowedTargetRoots(downloadsDir, mediaPath string) []string {
	roots := make([]string, 0, 2)
	seen := map[string]struct{}{}
	for _, raw := range []string{downloadsDir, mediaPath} {
		raw = strings.TrimSpace(raw)
		if raw == "" || !filepath.IsAbs(raw) {
			continue
		}
		cleaned := filepath.Clean(raw)
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		roots = append(roots, cleaned)
	}
	return roots
}

func isWithinRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
