package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ensureManagedPath(path string, roots ...string) (string, error) {
	cleaned, err := canonicalManagedPath(path)
	if err != nil {
		return "", err
	}
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		canonicalRoot, err := canonicalManagedPath(root)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(canonicalRoot, cleaned)
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

func canonicalManagedPath(path string) (string, error) {
	cleaned := strings.TrimSpace(path)
	if cleaned == "" {
		return "", fmt.Errorf("path is empty")
	}

	absolute, err := filepath.Abs(filepath.Clean(cleaned))
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		return filepath.Clean(resolved), nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	parent := filepath.Dir(absolute)
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		if os.IsNotExist(err) {
			return filepath.Clean(absolute), nil
		}
		return "", err
	}
	return filepath.Join(filepath.Clean(resolvedParent), filepath.Base(absolute)), nil
}
