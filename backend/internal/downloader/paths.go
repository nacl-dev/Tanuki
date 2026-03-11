package downloader

import (
	"fmt"
	"os"
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

	cleaned, err := canonicalTargetPath(targetDirectory)
	if err != nil {
		return "", fmt.Errorf("target_directory is invalid: %w", err)
	}
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
		cleaned, err := canonicalTargetPath(raw)
		if err != nil {
			continue
		}
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

func canonicalTargetPath(path string) (string, error) {
	cleaned := strings.TrimSpace(path)
	if cleaned == "" {
		return "", fmt.Errorf("path is empty")
	}

	absolute, err := filepath.Abs(filepath.Clean(cleaned))
	if err != nil {
		return "", err
	}

	current := absolute
	missingSegments := make([]string, 0, 4)
	for {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			for i := len(missingSegments) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missingSegments[i])
			}
			return filepath.Clean(resolved), nil
		} else if !os.IsNotExist(err) {
			return "", err
		}

		parent := filepath.Dir(current)
		if parent == current {
			for i := len(missingSegments) - 1; i >= 0; i-- {
				current = filepath.Join(current, missingSegments[i])
			}
			return filepath.Clean(current), nil
		}

		missingSegments = append(missingSegments, filepath.Base(current))
		current = parent
	}
}
