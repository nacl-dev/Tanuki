// Package plugins manages community plugin loading, registration, and execution.
package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
)

// Loader discovers Python plugin files on disk and ensures they are registered
// in the database.
type Loader struct {
	dir string
	db  *database.DB
	log *zap.Logger
}

// NewLoader creates a Loader that scans the given directory for *.py files.
func NewLoader(dir string, db *database.DB, log *zap.Logger) *Loader {
	return &Loader{dir: dir, db: db, log: log}
}

// ScanDir walks the plugins directory, reads module-level constants from
// each .py file, and upserts them into the plugins table.  Returns the list
// of discovered plugins.
func (l *Loader) ScanDir() ([]models.Plugin, error) {
	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return nil, fmt.Errorf("ensure plugins dir: %w", err)
	}

	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return nil, fmt.Errorf("read plugins dir: %w", err)
	}

	var plugins []models.Plugin
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".py") {
			continue
		}
		absPath := filepath.Join(l.dir, entry.Name())
		p, err := l.loadOne(absPath)
		if err != nil {
			l.log.Warn("plugins: skip file", zap.String("file", entry.Name()), zap.Error(err))
			continue
		}
		plugins = append(plugins, *p)
	}

	return plugins, nil
}

// loadOne reads a single plugin file and upserts its record.
func (l *Loader) loadOne(path string) (*models.Plugin, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	sourceName := parseConstant(string(data), "SOURCE_NAME")
	if sourceName == "" {
		return nil, fmt.Errorf("missing SOURCE_NAME constant")
	}

	sourceURL := parseConstant(string(data), "SOURCE_URL")
	version := parseConstant(string(data), "VERSION")
	if version == "" {
		version = "0.0.0"
	}

	name := strings.TrimSuffix(filepath.Base(path), ".py")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.Title(name) //nolint:staticcheck

	var plugin models.Plugin
	err = l.db.Get(&plugin, `SELECT * FROM plugins WHERE source_name = $1`, sourceName)
	if err != nil {
		// Insert new plugin.
		err = l.db.Get(&plugin, `
			INSERT INTO plugins (name, source_name, source_url, file_path, version)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING *`,
			name, sourceName, sourceURL, path, version,
		)
		if err != nil {
			return nil, fmt.Errorf("insert plugin: %w", err)
		}
		l.log.Info("plugins: registered new plugin",
			zap.String("name", name),
			zap.String("source_name", sourceName),
		)
	} else {
		// Update path / version if changed.
		_, err = l.db.Exec(`
			UPDATE plugins SET file_path = $1, version = $2, source_url = $3, updated_at = NOW()
			WHERE id = $4`,
			path, version, sourceURL, plugin.ID,
		)
		if err != nil {
			return nil, fmt.Errorf("update plugin: %w", err)
		}
		plugin.FilePath = path
		plugin.Version = version
		plugin.SourceURL = sourceURL
	}

	return &plugin, nil
}

// parseConstant extracts a top-level Python string constant such as
//
//	SOURCE_NAME = "my_source"
//
// Returns the unquoted value or "" if not found.
func parseConstant(src, name string) string {
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, name) {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"'`)
		return val
	}
	return ""
}
