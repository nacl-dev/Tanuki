package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"
)

// MetadataResult is the structured output returned by a plugin's
// fetch_metadata function.
type MetadataResult struct {
	Title       string         `json:"title"`
	Tags        []string       `json:"tags"`
	Description string         `json:"description"`
	Language    string         `json:"language"`
	SourceURL   string         `json:"source_url"`
	Extra       map[string]any `json:"extra"`
}

// Runner executes a Python plugin in a subprocess.
type Runner struct {
	log *zap.Logger
}

// NewRunner creates a new plugin Runner.
func NewRunner(log *zap.Logger) *Runner {
	return &Runner{log: log}
}

// CanHandle invokes the plugin's can_handle(url) function and returns the
// boolean result.
func (r *Runner) CanHandle(ctx context.Context, pluginPath, url string) (bool, error) {
	script := fmt.Sprintf(
		`import importlib.util, sys; `+
			`spec = importlib.util.spec_from_file_location("plugin", %q); `+
			`mod = importlib.util.module_from_spec(spec); `+
			`spec.loader.exec_module(mod); `+
			`print(mod.can_handle(%q))`,
		pluginPath, url,
	)

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "python3", "-c", script).CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("can_handle: %w – %s", err, string(out))
	}

	result := strings.TrimSpace(string(out))
	return result == "True", nil
}

// FetchMetadata invokes the plugin's fetch_metadata(url) function and returns
// the parsed result.
func (r *Runner) FetchMetadata(ctx context.Context, pluginPath, url string) (*MetadataResult, error) {
	script := fmt.Sprintf(
		`import importlib.util, json, sys; `+
			`spec = importlib.util.spec_from_file_location("plugin", %q); `+
			`mod = importlib.util.module_from_spec(spec); `+
			`spec.loader.exec_module(mod); `+
			`result = mod.fetch_metadata(%q); `+
			`print(json.dumps(result if result else {}))`,
		pluginPath, url,
	)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "python3", "-c", script).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("fetch_metadata: %w – %s", err, string(out))
	}

	var meta MetadataResult
	if err := json.Unmarshal(out, &meta); err != nil {
		return nil, fmt.Errorf("parse metadata JSON: %w – raw: %s", err, string(out))
	}

	r.log.Info("plugins: fetched metadata",
		zap.String("plugin", pluginPath),
		zap.String("url", url),
		zap.String("title", meta.Title),
		zap.Int("tags", len(meta.Tags)),
	)

	return &meta, nil
}
