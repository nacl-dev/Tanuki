package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	htmlstd "html"
	"io"
	"net/http"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
)

type Hentai0Engine struct {
	client   *http.Client
	log      *zap.Logger
	progress func(id string, downloaded, total int64, files, totalFiles int)
}

func NewHentai0Engine(log *zap.Logger) *Hentai0Engine {
	return &Hentai0Engine{
		client: &http.Client{},
		log:    log,
	}
}

func (e *Hentai0Engine) SetProgressUpdater(fn func(id string, downloaded, total int64, files, totalFiles int)) {
	e.progress = fn
}

func (e *Hentai0Engine) CanHandle(rawURL string) bool {
	u, err := urlpkg.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "hentai0.com" || host == "www.hentai0.com"
}

func (e *Hentai0Engine) Download(ctx context.Context, job *models.DownloadJob) error {
	dest := job.TargetDirectory
	if dest == "" {
		dest = "/downloads"
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("hentai0 mkdir: %w", err)
	}

	video, err := e.extractVideo(ctx, job.URL)
	if err != nil {
		return err
	}
	if len(video.Sources) == 0 {
		return newUnsupportedURLError("hentai0", "no playable video sources found")
	}

	sourceURL := video.Sources[len(video.Sources)-1]
	fileName := sanitizeArchiveName(video.Title)
	if fileName == "" {
		fileName = sanitizeArchiveName(pathBaseWithoutExt(job.URL))
	}
	if fileName == "" {
		fileName = "video"
	}

	ext := ".mp4"
	if parsed, err := urlpkg.Parse(sourceURL); err == nil {
		if sourceExt := strings.ToLower(filepath.Ext(parsed.Path)); sourceExt != "" {
			ext = sourceExt
		}
	}

	finalPath := filepath.Join(dest, fileName+ext)
	tmpPath := finalPath + ".tmp"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return fmt.Errorf("hentai0 video request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Tanuki)")
	req.Header.Set("Referer", job.URL)

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("hentai0 video fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hentai0 video status: %d", resp.StatusCode)
	}

	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("hentai0 create file: %w", err)
	}

	written, copyErr := copyWithProgress(resp.Body, out, resp.ContentLength, func(downloaded int64, total int64) {
		if e.progress != nil {
			e.progress(job.ID, downloaded, total, 0, 1)
		}
	})
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("hentai0 write file: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("hentai0 close file: %w", closeErr)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("hentai0 rename file: %w", err)
	}

	e.log.Info("hentai0: downloaded", zap.String("path", finalPath), zap.Int64("bytes", written))
	sidecar := models.ImportMetadata{
		Title:     video.Title,
		SourceURL: video.SourceURL,
		PosterURL: video.PosterURL,
		Tags:      video.Tags,
	}
	if err := writeImportMetadata(finalPath, sidecar); err != nil {
		e.log.Warn("hentai0: write metadata sidecar failed", zap.String("path", finalPath), zap.Error(err))
	}
	return nil
}

func (e *Hentai0Engine) FetchMetadata(rawURL string) (*SourceMetadata, error) {
	video, err := e.extractVideo(context.Background(), rawURL)
	if err != nil {
		return nil, err
	}
	return &SourceMetadata{
		Title:      video.Title,
		TotalFiles: 1,
		Tags:       video.Tags,
		Extra: map[string]string{
			"source_url": video.SourceURL,
			"poster_url": video.PosterURL,
		},
	}, nil
}

type hentai0Video struct {
	Title     string
	SourceURL string
	PosterURL string
	Tags      []string
	Sources   []string
}

type hentai0VideoData struct {
	FullName string `json:"fullName"`
	Name     string `json:"name"`
	Episode  int    `json:"episode"`
	Poster   string `json:"poster"`
	Tags     []struct {
		Name string `json:"name"`
	} `json:"tags"`
}

var (
	hentai0VideoSourceRe = regexp.MustCompile(`:video_source="([^"]+)"`)
	hentai0VideoDataRe   = regexp.MustCompile(`:video="([^"]+)"`)
)

func (e *Hentai0Engine) extractVideo(ctx context.Context, rawURL string) (*hentai0Video, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("hentai0 page request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Tanuki)")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hentai0 page fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hentai0 page status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("hentai0 page read: %w", err)
	}
	html := string(body)

	sourceMatch := hentai0VideoSourceRe.FindStringSubmatch(html)
	if len(sourceMatch) < 2 {
		return nil, newUnsupportedURLError("hentai0", "video_source not found")
	}

	var sources []string
	if err := json.Unmarshal([]byte(htmlstd.UnescapeString(sourceMatch[1])), &sources); err != nil {
		return nil, fmt.Errorf("hentai0 parse sources: %w", err)
	}
	for i := range sources {
		sources[i] = htmlstd.UnescapeString(strings.TrimSpace(sources[i]))
	}

	video := &hentai0Video{
		Sources:   compactStrings(sources),
		SourceURL: rawURL,
	}

	dataMatch := hentai0VideoDataRe.FindStringSubmatch(html)
	if len(dataMatch) >= 2 {
		var data hentai0VideoData
		if err := json.Unmarshal([]byte(htmlstd.UnescapeString(dataMatch[1])), &data); err == nil {
			switch {
			case strings.TrimSpace(data.FullName) != "":
				video.Title = strings.TrimSpace(data.FullName)
			case strings.TrimSpace(data.Name) != "" && data.Episode > 0:
				video.Title = fmt.Sprintf("%s - %02d", strings.TrimSpace(data.Name), data.Episode)
			default:
				video.Title = strings.TrimSpace(data.Name)
			}
			if strings.TrimSpace(data.Poster) != "" {
				if pageURL, parseErr := urlpkg.Parse(rawURL); parseErr == nil {
					video.PosterURL = fmt.Sprintf("%s://%s/assets/screens/poster/%s",
						pageURL.Scheme,
						pageURL.Host,
						strings.TrimLeft(strings.TrimSpace(data.Poster), "/"),
					)
				}
			}
			for _, tag := range data.Tags {
				if name := strings.TrimSpace(tag.Name); name != "" {
					video.Tags = append(video.Tags, name)
				}
			}
		}
	}

	video.Tags = compactStrings(video.Tags)
	return video, nil
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func pathBaseWithoutExt(rawURL string) string {
	u, err := urlpkg.Parse(rawURL)
	if err != nil {
		return ""
	}
	base := filepath.Base(strings.TrimSuffix(u.Path, "/"))
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func writeImportMetadata(mediaPath string, metadata models.ImportMetadata) error {
	if strings.TrimSpace(metadata.Title) == "" &&
		strings.TrimSpace(metadata.SourceURL) == "" &&
		strings.TrimSpace(metadata.PosterURL) == "" &&
		len(metadata.Tags) == 0 {
		return nil
	}

	body, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(mediaPath+".tanuki.json", body, 0o644)
}
