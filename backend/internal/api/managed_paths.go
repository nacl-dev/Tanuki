package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ensureManagedPath(path string, roots ...string) (string, error) {
	cleaned := strings.TrimSpace(path)
	if cleaned == "" {
		return "", fmt.Errorf("path is empty")
	}

	cleaned = filepath.Clean(cleaned)
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		rel, err := filepath.Rel(filepath.Clean(root), cleaned)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return cleaned, nil
		}
	}

	return "", fmt.Errorf("path is outside managed roots")
}

func statManagedPath(path string, roots ...string) error {
	cleaned, err := ensureManagedPath(path, roots...)
	if err != nil {
		return err
	}
	if _, err := os.Stat(cleaned); err != nil {
		return err
	}
	return nil
}
