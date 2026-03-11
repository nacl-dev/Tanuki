package api

import (
	"os"
	"path/filepath"
)

func pathStatus(path string) map[string]any {
	info, err := os.Stat(path)
	if err != nil {
		return map[string]any{
			"path":   path,
			"exists": false,
			"error":  err.Error(),
		}
	}

	return map[string]any{
		"path":     path,
		"exists":   true,
		"is_dir":   info.IsDir(),
		"writable": isWritable(path),
	}
}

func isWritable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	if info.IsDir() {
		file, err := os.CreateTemp(path, ".tanuki-health-*")
		if err != nil {
			return false
		}
		name := file.Name()
		_ = file.Close()
		_ = os.Remove(name)
		return true
	}

	file, err := os.OpenFile(filepath.Clean(path), os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	_ = file.Close()
	return true
}
