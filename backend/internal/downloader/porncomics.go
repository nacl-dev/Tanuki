package downloader

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	urlpkg "net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
)

const (
	pornComicsAPIBase    = "https://porncomics.eu/api/comics/"
	pornComicsImagesBase = "https://porncomics.eu/images/"
	pornComicsPageSuffix = "-w1536-q75.jpg"
)

var pornComicsIDRe = regexp.MustCompile(`-(\d+)$`)

type PornComicsEngine struct {
	client   *http.Client
	log      *zap.Logger
	progress func(id string, downloaded, total int64, files, totalFiles int)
}

func NewPornComicsEngine(cookiesPath string, log *zap.Logger) *PornComicsEngine {
	client, err := newHTTPClientWithCookies(cookiesPath)
	if err != nil {
		log.Warn("porncomics: cookies unavailable", zap.String("path", cookiesPath), zap.Error(err))
		client = &http.Client{}
	}

	return &PornComicsEngine{
		client: client,
		log:    log,
	}
}

func (e *PornComicsEngine) SetProgressUpdater(fn func(id string, downloaded, total int64, files, totalFiles int)) {
	e.progress = fn
}

func (e *PornComicsEngine) CanHandle(rawURL string) bool {
	u, err := urlpkg.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host != "porncomics.eu" && host != "www.porncomics.eu" {
		return false
	}
	path := strings.Trim(strings.ToLower(u.Path), "/")
	return strings.HasPrefix(path, "comics/")
}

func (e *PornComicsEngine) Download(ctx context.Context, job *models.DownloadJob) error {
	dest := job.TargetDirectory
	if dest == "" {
		dest = "/downloads"
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("porncomics mkdir: %w", err)
	}

	comic, err := e.fetchComic(ctx, job.URL)
	if err != nil {
		return err
	}
	if len(comic.Pages) == 0 {
		return newUnsupportedURLError("porncomics", "comic has no pages")
	}

	archiveName := sanitizeArchiveName(comic.Name)
	if archiveName == "" {
		archiveName = sanitizeArchiveName(path.Base(strings.TrimSuffix(job.URL, "/")))
	}
	if archiveName == "" {
		archiveName = "porncomics-comic"
	}

	finalPath := filepath.Join(dest, archiveName+".cbz")
	tmpPath := finalPath + ".tmp"

	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("porncomics create cbz: %w", err)
	}

	zipWriter := zip.NewWriter(file)
	for idx, page := range comic.Pages {
		imageURL := pornComicsPageImageURL(page.PageID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
		if err != nil {
			zipWriter.Close()
			file.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("porncomics page request %d: %w", idx+1, err)
		}
		setPornComicsHeaders(req, job.URL, "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")

		resp, err := e.client.Do(req)
		if err != nil {
			zipWriter.Close()
			file.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("porncomics page fetch %d: %w", idx+1, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			zipWriter.Close()
			file.Close()
			_ = os.Remove(tmpPath)
			if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusTooManyRequests {
				return newUnsupportedURLError("porncomics", blockedSourceDetail(resp.StatusCode))
			}
			return fmt.Errorf("porncomics page status %d: %d", idx+1, resp.StatusCode)
		}

		entryName := fmt.Sprintf("%03d.jpg", page.PageNumber)
		writer, err := zipWriter.Create(entryName)
		if err != nil {
			resp.Body.Close()
			zipWriter.Close()
			file.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("porncomics cbz entry %d: %w", idx+1, err)
		}
		if _, err := io.Copy(writer, resp.Body); err != nil {
			resp.Body.Close()
			zipWriter.Close()
			file.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("porncomics cbz write %d: %w", idx+1, err)
		}
		resp.Body.Close()

		if e.progress != nil {
			e.progress(job.ID, 0, 0, idx+1, len(comic.Pages))
		}
	}

	if err := zipWriter.Close(); err != nil {
		file.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("porncomics close cbz: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("porncomics close file: %w", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("porncomics rename cbz: %w", err)
	}

	if err := writeImportMetadata(finalPath, models.ImportMetadata{
		Title:     comic.Name,
		SourceURL: job.URL,
		Tags:      comic.Tags(),
	}); err != nil {
		e.log.Warn("porncomics: write metadata sidecar failed", zap.String("path", finalPath), zap.Error(err))
	}

	e.log.Info("porncomics: comic archived", zap.String("path", finalPath), zap.Int("pages", len(comic.Pages)))
	return nil
}

func (e *PornComicsEngine) FetchMetadata(rawURL string) (*SourceMetadata, error) {
	comic, err := e.fetchComic(context.Background(), rawURL)
	if err != nil {
		return nil, err
	}
	return &SourceMetadata{
		Title:      comic.Name,
		Tags:       comic.Tags(),
		TotalFiles: len(comic.Pages),
		Extra: map[string]string{
			"source_url": rawURL,
		},
	}, nil
}

type pornComicsAPIResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Comics []pornComicsComic `json:"comics"`
	} `json:"data"`
}

type pornComicsComic struct {
	ComicID    int               `json:"comicId"`
	Name       string            `json:"name"`
	PageCount  int               `json:"pageCnt"`
	Authors    []pornComicsLabel `json:"authors"`
	Sections   []pornComicsLabel `json:"sections"`
	Characters []pornComicsLabel `json:"characters"`
	Categories []pornComicsLabel `json:"categories"`
	Pages      []pornComicsPage  `json:"pages"`
}

type pornComicsLabel struct {
	Name string `json:"name"`
}

type pornComicsPage struct {
	PageID     int `json:"pageId"`
	PageNumber int `json:"pageNumber"`
}

func (c pornComicsComic) Tags() []string {
	values := make([]string, 0, len(c.Authors)+len(c.Sections)+len(c.Characters)+len(c.Categories))
	for _, item := range c.Authors {
		values = append(values, qualifyTags("artist", []string{item.Name})...)
	}
	for _, item := range c.Sections {
		values = append(values, qualifyTags("site", []string{item.Name})...)
	}
	for _, item := range c.Characters {
		values = append(values, qualifyTags("character", []string{item.Name})...)
	}
	for _, item := range c.Categories {
		values = append(values, qualifyTags("genre", []string{item.Name})...)
	}
	return uniqueNonEmptyStrings(values)
}

func (e *PornComicsEngine) fetchComic(ctx context.Context, rawURL string) (*pornComicsComic, error) {
	comicID, err := extractPornComicsComicID(rawURL)
	if err != nil {
		return nil, newUnsupportedURLError("porncomics", err.Error())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pornComicsAPIBase+comicID, nil)
	if err != nil {
		return nil, fmt.Errorf("porncomics api request: %w", err)
	}
	setPornComicsHeaders(req, rawURL, "application/json,text/plain,*/*")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("porncomics api fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusTooManyRequests {
			return nil, newUnsupportedURLError("porncomics", blockedSourceDetail(resp.StatusCode))
		}
		if resp.StatusCode == http.StatusNotFound {
			return nil, newUnsupportedURLError("porncomics", "comic not found")
		}
		return nil, fmt.Errorf("porncomics api status: %d", resp.StatusCode)
	}

	var payload pornComicsAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("porncomics api decode: %w", err)
	}
	if !payload.Success || len(payload.Data.Comics) == 0 {
		return nil, newUnsupportedURLError("porncomics", "comic payload missing")
	}

	comic := payload.Data.Comics[0]
	sort.SliceStable(comic.Pages, func(i, j int) bool {
		return comic.Pages[i].PageNumber < comic.Pages[j].PageNumber
	})
	return &comic, nil
}

func extractPornComicsComicID(rawURL string) (string, error) {
	u, err := urlpkg.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid url")
	}
	host := strings.ToLower(u.Hostname())
	if host != "porncomics.eu" && host != "www.porncomics.eu" {
		return "", fmt.Errorf("unsupported porncomics url")
	}
	trimmedPath := strings.Trim(strings.ToLower(u.Path), "/")
	if !strings.HasPrefix(trimmedPath, "comics/") {
		return "", fmt.Errorf("unsupported porncomics url")
	}
	if pathBase := path.Base(strings.TrimSuffix(u.Path, "/")); pathBase != "." && pathBase != "/" {
		match := pornComicsIDRe.FindStringSubmatch(pathBase)
		if len(match) >= 2 {
			return match[1], nil
		}
	}
	return "", fmt.Errorf("unsupported porncomics url")
}

func pornComicsPageImageURL(pageID int) string {
	return fmt.Sprintf("%s%d%s", pornComicsImagesBase, pageID, pornComicsPageSuffix)
}

func setPornComicsHeaders(req *http.Request, referer, accept string) {
	if req == nil {
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if strings.TrimSpace(accept) != "" {
		req.Header.Set("Accept", accept)
	}
	if strings.TrimSpace(referer) != "" {
		req.Header.Set("Referer", referer)
		req.Header.Set("Origin", sourceBaseURL(referer))
	}
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}
