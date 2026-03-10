package plugins

import (
	"context"
	"sync"

	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
)

// Registry holds all loaded plugins in memory and provides lookup methods.
type Registry struct {
	mu      sync.RWMutex
	plugins []models.Plugin
	loader  *Loader
	runner  *Runner
	db      *database.DB
	log     *zap.Logger
}

// NewRegistry creates a Registry backed by the given Loader and Runner.
func NewRegistry(db *database.DB, pluginsDir string, log *zap.Logger) *Registry {
	return &Registry{
		loader: NewLoader(pluginsDir, db, log),
		runner: NewRunner(log),
		db:     db,
		log:    log,
	}
}

// LoadAll scans the plugin directory and refreshes the in-memory list.
func (r *Registry) LoadAll() error {
	plugins, err := r.loader.ScanDir()
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.plugins = plugins
	r.mu.Unlock()
	r.log.Info("plugins: registry loaded", zap.Int("count", len(plugins)))
	return nil
}

// ListPlugins returns all known plugins from the database.
func (r *Registry) ListPlugins(ctx context.Context) ([]models.Plugin, error) {
	var plugins []models.Plugin
	if err := r.db.SelectContext(ctx, &plugins, `SELECT * FROM plugins ORDER BY name`); err != nil {
		return nil, err
	}
	return plugins, nil
}

// FindHandler returns the first enabled plugin that can handle the given URL.
func (r *Registry) FindHandler(ctx context.Context, url string) (*models.Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.plugins {
		if !p.Enabled {
			continue
		}
		ok, err := r.runner.CanHandle(ctx, p.FilePath, url)
		if err != nil {
			r.log.Warn("plugins: can_handle error",
				zap.String("plugin", p.SourceName),
				zap.Error(err),
			)
			continue
		}
		if ok {
			return &p, nil
		}
	}
	return nil, nil
}

// FetchMetadata runs the given plugin's fetch_metadata for the URL.
func (r *Registry) FetchMetadata(ctx context.Context, plugin *models.Plugin, url string) (*MetadataResult, error) {
	return r.runner.FetchMetadata(ctx, plugin.FilePath, url)
}

// TogglePlugin enables or disables a plugin by its ID.
func (r *Registry) TogglePlugin(ctx context.Context, id string, enabled bool) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE plugins SET enabled = $1, updated_at = NOW() WHERE id = $2`,
		enabled, id,
	)
	if err != nil {
		return err
	}
	// Refresh in-memory list.
	return r.refreshFromDB(ctx)
}

// DeletePlugin removes a plugin record from the database (file deletion is
// handled by the API handler).
func (r *Registry) DeletePlugin(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM plugins WHERE id = $1`, id)
	if err != nil {
		return err
	}
	return r.refreshFromDB(ctx)
}

func (r *Registry) refreshFromDB(ctx context.Context) error {
	var plugins []models.Plugin
	if err := r.db.SelectContext(ctx, &plugins, `SELECT * FROM plugins ORDER BY name`); err != nil {
		return err
	}
	r.mu.Lock()
	r.plugins = plugins
	r.mu.Unlock()
	return nil
}
