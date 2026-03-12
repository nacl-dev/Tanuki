package downloader

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
)

// YtDlpEngine wraps the yt-dlp CLI tool.
type YtDlpEngine struct {
	configPath        string
	cookiesPath       string
	impersonateClient string
	log               *zap.Logger
	progress          func(id string, downloaded, total int64, files, totalFiles int)
}

// NewYtDlpEngine creates a YtDlpEngine using an optional config file.
func NewYtDlpEngine(configPath, cookiesPath, impersonateClient string, log *zap.Logger) *YtDlpEngine {
	return &YtDlpEngine{
		configPath:        configPath,
		cookiesPath:       cookiesPath,
		impersonateClient: strings.TrimSpace(impersonateClient),
		log:               log,
	}
}

func (e *YtDlpEngine) SetProgressUpdater(fn func(id string, downloaded, total int64, files, totalFiles int)) {
	e.progress = fn
}

// CanHandle returns true for video-hosting URLs that yt-dlp handles well.
func (e *YtDlpEngine) CanHandle(url string) bool {
	lower := strings.ToLower(url)
	videoHosts := []string{
		"youtube.com", "youtu.be", "twitch.tv", "vimeo.com",
		"dailymotion.com", "nicovideo.jp", "iwara.tv", "pornhub.com",
		"xvideos.com", "xhamster.com",
	}
	for _, h := range videoHosts {
		if strings.Contains(lower, h) {
			return true
		}
	}
	return false
}

// Download runs yt-dlp for the given job.
func (e *YtDlpEngine) Download(ctx context.Context, job *models.DownloadJob) error {
	dest := job.TargetDirectory
	if dest == "" {
		dest = "/downloads"
	}
	meta, _ := e.extractMetadata(job.URL)

	args := append(e.baseArgs(), []string{
		"--output", dest + "/%(title)s.%(ext)s",
		"--write-info-json",
		"--no-playlist",
		"--progress",
		"--newline",
		"--progress-template", "download:tanuki:%(progress.downloaded_bytes)s:%(progress.total_bytes_estimate)s:%(progress.total_bytes)s",
	}...)
	args = append(args, job.URL)

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("yt-dlp stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("yt-dlp stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("yt-dlp start: %w", err)
	}

	stderrLines := &logBuffer{}
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			stderrLines.Add(line)
			e.log.Debug("yt-dlp", zap.String("stderr", line))
		}
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if e.progress != nil {
			if downloaded, total, ok := parseYtDlpProgress(line); ok {
				e.progress(job.ID, downloaded, total, 0, 1)
			}
		}
		e.log.Debug("yt-dlp", zap.String("stdout", line))
	}

	if err := cmd.Wait(); err != nil {
		if unsupportedErr := classifyYtDlpError(stderrLines.Joined()); unsupportedErr != nil {
			return unsupportedErr
		}
		return fmt.Errorf("yt-dlp: %w", err)
	}

	if meta != nil {
		if outputPath, err := detectLatestDownloadedFile(dest); err == nil {
			sidecar := models.ImportMetadata{
				Title:     meta.Title,
				WorkTitle: meta.WorkTitle,
				WorkIndex: meta.WorkIndex,
				SourceURL: firstNonEmpty(strings.TrimSpace(meta.Extra["source_url"]), job.URL),
				PosterURL: strings.TrimSpace(meta.Extra["poster_url"]),
				Tags:      meta.Tags,
			}
			if err := writeImportMetadata(outputPath, sidecar); err != nil {
				e.log.Warn("yt-dlp: write metadata sidecar failed", zap.String("path", outputPath), zap.Error(err))
			}
		}
	}

	return nil
}

// FetchMetadata uses yt-dlp --dump-json to retrieve video metadata.
func (e *YtDlpEngine) FetchMetadata(url string) (*SourceMetadata, error) {
	return e.extractMetadata(url)
}

func (e *YtDlpEngine) extractMetadata(url string) (*SourceMetadata, error) {
	args := append(e.baseArgs(), "--dump-json", "--no-playlist")
	args = append(args, url)

	out, err := exec.Command("yt-dlp", args...).CombinedOutput()
	if err != nil {
		if unsupportedErr := classifyYtDlpError(string(out)); unsupportedErr != nil {
			return nil, unsupportedErr
		}
		trimmed := strings.TrimSpace(string(out))
		if trimmed != "" {
			return nil, fmt.Errorf("yt-dlp metadata: %w: %s", err, trimmed)
		}
		return nil, fmt.Errorf("yt-dlp metadata: %w", err)
	}

	var obj map[string]interface{}
	if jsonErr := json.Unmarshal(out, &obj); jsonErr != nil {
		return nil, fmt.Errorf("parse metadata: %w", jsonErr)
	}

	meta := &SourceMetadata{}
	if t, ok := obj["title"].(string); ok {
		meta.Title = t
	}
	if d, ok := obj["description"].(string); ok {
		meta.Description = d
	}
	meta.Tags = extractYtDlpTags(obj)
	meta.Extra = map[string]string{}
	if webpageURL, ok := obj["webpage_url"].(string); ok {
		meta.Extra["source_url"] = webpageURL
	}
	if thumbnail, ok := obj["thumbnail"].(string); ok {
		meta.Extra["poster_url"] = thumbnail
	}
	meta.WorkTitle, meta.WorkIndex = extractYtDlpWorkMetadata(obj)

	meta.TotalFiles = 1
	return meta, nil
}

func (e *YtDlpEngine) hasConfig() bool {
	if e.configPath == "" {
		return false
	}
	if _, err := os.Stat(e.configPath); err != nil {
		return false
	}
	return true
}

func (e *YtDlpEngine) baseArgs() []string {
	args := make([]string, 0, 8)
	if e.hasConfig() {
		args = append(args, "--config-location", e.configPath)
	}
	if strings.TrimSpace(e.cookiesPath) != "" {
		args = append(args, "--cookies", e.cookiesPath)
	}
	if strings.TrimSpace(e.impersonateClient) != "" {
		args = append(args, "--impersonate", e.impersonateClient)
		args = append(args, "--extractor-args", "generic:impersonate="+e.impersonateClient)
	}
	return args
}

func classifyYtDlpError(output string) error {
	lower := strings.ToLower(output)
	switch {
	case strings.Contains(lower, "cloudflare anti-bot challenge"):
		return newUnsupportedURLError("yt-dlp", blockedChallengeDetail("source blocked by Cloudflare anti-bot challenge"))
	case strings.Contains(lower, "unsupported url"):
		return newUnsupportedURLError("yt-dlp", strings.TrimSpace(output))
	case strings.Contains(lower, "http error 403"), strings.Contains(lower, "http error 401"), strings.Contains(lower, "http error 429"):
		return newUnsupportedURLError("yt-dlp", blockedChallengeDetail("remote source blocked yt-dlp with an HTTP challenge"))
	default:
		return nil
	}
}

func parseYtDlpProgress(line string) (int64, int64, bool) {
	const prefix = "download:tanuki:"
	if !strings.HasPrefix(line, prefix) {
		return 0, 0, false
	}

	parts := strings.Split(strings.TrimPrefix(line, prefix), ":")
	if len(parts) != 3 {
		return 0, 0, false
	}

	downloaded := parseProgressNumber(parts[0])
	estimated := parseProgressNumber(parts[1])
	total := parseProgressNumber(parts[2])
	if total <= 0 {
		total = estimated
	}
	if downloaded < 0 {
		downloaded = 0
	}
	return downloaded, total, true
}

func parseProgressNumber(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "NA") {
		return 0
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if value < 0 {
		return 0
	}
	return int64(value)
}

func detectLatestDownloadedFile(dir string) (string, error) {
	var latestPath string
	var latestTime int64

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		lower := strings.ToLower(path)
		if strings.HasSuffix(lower, ".info.json") || strings.HasSuffix(lower, ".tanuki.json") || strings.HasSuffix(lower, ".part") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().UnixNano() > latestTime {
			latestTime = info.ModTime().UnixNano()
			latestPath = path
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if latestPath == "" {
		return "", fmt.Errorf("no downloaded media file found in %s", dir)
	}
	return latestPath, nil
}

func extractYtDlpWorkMetadata(obj map[string]interface{}) (string, int) {
	workIndex := firstPositiveInt(
		ytDlpIntField(obj, "episode_number"),
		ytDlpIntField(obj, "chapter_number"),
		ytDlpIntField(obj, "track_number"),
		ytDlpIntField(obj, "playlist_index"),
	)
	if workIndex <= 0 {
		return "", 0
	}

	base := ytDlpStringField(obj, "series")
	season := ytDlpStringField(obj, "season")
	if base == "" {
		base = ytDlpStringField(obj, "album")
	}
	if base == "" {
		base = ytDlpStringField(obj, "playlist_title")
	}
	if base == "" {
		base = season
	}
	base = cleanWorkTitle(base)
	if base == "" {
		return "", 0
	}

	season = cleanWorkTitle(season)
	switch {
	case season != "" && !strings.EqualFold(season, base):
		base = cleanWorkTitle(base + " " + season)
	case season == "" && ytDlpIntField(obj, "season_number") > 0:
		base = cleanWorkTitle(base + " Season " + zeroPadInt(ytDlpIntField(obj, "season_number"), 2))
	}

	return base, workIndex
}

func extractYtDlpTags(obj map[string]interface{}) []string {
	tags := make([]string, 0, 24)
	tags = append(tags, ytDlpStringListField(obj, "tags")...)
	tags = append(tags, qualifyTags("artist", ytDlpStringListField(obj, "artist", "artists", "creator", "creators"))...)
	tags = append(tags, qualifyTags("series", ytDlpStringListField(obj, "series"))...)
	tags = append(tags, qualifyTags("genre", ytDlpStringListField(obj, "categories", "genre", "genres"))...)
	tags = append(tags, qualifyTags("language", ytDlpStringListField(obj, "language", "languages"))...)
	tags = append(tags, qualifyTags("uploader", ytDlpStringListField(obj, "uploader", "channel"))...)
	return compactStrings(tags)
}

func ytDlpStringField(obj map[string]interface{}, key string) string {
	value, ok := obj[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func ytDlpStringListField(obj map[string]interface{}, keys ...string) []string {
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		value, ok := obj[key]
		if !ok || value == nil {
			continue
		}
		switch v := value.(type) {
		case string:
			if text := strings.TrimSpace(v); text != "" {
				values = append(values, text)
			}
		case []interface{}:
			for _, item := range v {
				if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
					values = append(values, strings.TrimSpace(text))
				}
			}
		case []string:
			for _, item := range v {
				if text := strings.TrimSpace(item); text != "" {
					values = append(values, text)
				}
			}
		}
	}
	return compactStrings(values)
}

func ytDlpIntField(obj map[string]interface{}, key string) int {
	value, ok := obj[key]
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case float64:
		if v <= 0 {
			return 0
		}
		return int(v)
	case int:
		if v <= 0 {
			return 0
		}
		return v
	case int64:
		if v <= 0 {
			return 0
		}
		return int(v)
	case string:
		return parsePositiveInt(v)
	default:
		return 0
	}
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
