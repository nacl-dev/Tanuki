package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
)

type DanbooruEngine struct {
	client   *http.Client
	log      *zap.Logger
	progress func(id string, downloaded, total int64, files, totalFiles int)
}

func NewDanbooruEngine(log *zap.Logger) *DanbooruEngine {
	return &DanbooruEngine{
		client: &http.Client{},
		log:    log,
	}
}

func (e *DanbooruEngine) SetProgressUpdater(fn func(id string, downloaded, total int64, files, totalFiles int)) {
	e.progress = fn
}

func (e *DanbooruEngine) CanHandle(rawURL string) bool {
	u, err := urlpkg.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host != "danbooru.donmai.us" && host != "safebooru.donmai.us" {
		return false
	}
	return danbooruPostIDRe.MatchString(strings.Trim(u.Path, "/"))
}

func (e *DanbooruEngine) Download(ctx context.Context, job *models.DownloadJob) error {
	dest := job.TargetDirectory
	if dest == "" {
		dest = "/downloads"
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("danbooru mkdir: %w", err)
	}

	post, err := e.fetchPost(ctx, job.URL)
	if err != nil {
		return err
	}

	fileURL := strings.TrimSpace(post.FileURL)
	if fileURL == "" && len(post.MediaAsset.Variants) > 0 {
		for _, variant := range post.MediaAsset.Variants {
			if variant.Type == "original" && strings.TrimSpace(variant.URL) != "" {
				fileURL = strings.TrimSpace(variant.URL)
				break
			}
		}
	}
	if fileURL == "" {
		return newUnsupportedURLError("danbooru", "post has no downloadable file url")
	}

	filename := sanitizeArchiveName(post.DisplayTitle())
	if filename == "" {
		filename = fmt.Sprintf("danbooru-%d", post.ID)
	}
	ext := strings.ToLower(strings.TrimSpace(post.FileExt))
	if ext == "" {
		if parsed, parseErr := urlpkg.Parse(fileURL); parseErr == nil {
			ext = strings.TrimPrefix(strings.ToLower(filepath.Ext(parsed.Path)), ".")
		}
	}
	if ext == "" {
		ext = "jpg"
	}

	finalPath := filepath.Join(dest, filename+"."+ext)
	tmpPath := finalPath + ".tmp"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return fmt.Errorf("danbooru file request: %w", err)
	}
	req.Header.Set("User-Agent", "Tanuki/1.0")
	req.Header.Set("Referer", sourceBaseURL(job.URL))

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("danbooru file fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("danbooru file status: %d", resp.StatusCode)
	}

	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("danbooru create file: %w", err)
	}

	written, copyErr := copyWithProgress(resp.Body, out, resp.ContentLength, func(downloaded int64, total int64) {
		if e.progress != nil {
			e.progress(job.ID, downloaded, total, 0, 1)
		}
	})
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("danbooru write file: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("danbooru close file: %w", closeErr)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("danbooru rename file: %w", err)
	}

	sidecar := models.ImportMetadata{
		Title:     post.DisplayTitle(),
		SourceURL: job.URL,
		PosterURL: strings.TrimSpace(post.PreviewFileURL),
		Tags:      post.AllTags(),
	}
	if err := writeImportMetadata(finalPath, sidecar); err != nil {
		e.log.Warn("danbooru: write metadata sidecar failed", zap.String("path", finalPath), zap.Error(err))
	}

	e.log.Info("danbooru: downloaded", zap.String("path", finalPath), zap.Int64("bytes", written))
	return nil
}

func (e *DanbooruEngine) FetchMetadata(rawURL string) (*SourceMetadata, error) {
	post, err := e.fetchPost(context.Background(), rawURL)
	if err != nil {
		return nil, err
	}

	extra := map[string]string{
		"source_url": rawURL,
	}
	if strings.TrimSpace(post.PreviewFileURL) != "" {
		extra["poster_url"] = strings.TrimSpace(post.PreviewFileURL)
	}

	return &SourceMetadata{
		Title:      post.DisplayTitle(),
		Tags:       post.AllTags(),
		TotalFiles: 1,
		Extra:      extra,
	}, nil
}

type danbooruPost struct {
	ID                 int    `json:"id"`
	FileURL            string `json:"file_url"`
	PreviewFileURL     string `json:"preview_file_url"`
	FileExt            string `json:"file_ext"`
	Site               string `json:"-"`
	Rating             string `json:"rating"`
	TagStringGeneral   string `json:"tag_string_general"`
	TagStringArtist    string `json:"tag_string_artist"`
	TagStringCharacter string `json:"tag_string_character"`
	TagStringCopyright string `json:"tag_string_copyright"`
	TagStringMeta      string `json:"tag_string_meta"`
	MediaAsset         struct {
		Variants []struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"variants"`
	} `json:"media_asset"`
}

func (p *danbooruPost) DisplayTitle() string {
	copyright := firstTagFromString(p.TagStringCopyright)
	artist := firstTagFromString(p.TagStringArtist)
	switch {
	case copyright != "" && artist != "":
		return fmt.Sprintf("%s by %s #%d", copyright, artist, p.ID)
	case copyright != "":
		return fmt.Sprintf("%s #%d", copyright, p.ID)
	case artist != "":
		return fmt.Sprintf("%s #%d", artist, p.ID)
	default:
		return fmt.Sprintf("Danbooru Post #%d", p.ID)
	}
}

func (p *danbooruPost) AllTags() []string {
	tags := make([]string, 0, 32)
	tags = append(tags, tagsFromString(p.TagStringGeneral)...)
	tags = append(tags, qualifyTags("artist", tagsFromString(p.TagStringArtist))...)
	tags = append(tags, qualifyTags("character", tagsFromString(p.TagStringCharacter))...)
	tags = append(tags, qualifyTags("series", tagsFromString(p.TagStringCopyright))...)
	tags = append(tags, qualifyTags("meta", tagsFromString(p.TagStringMeta))...)
	tags = append(tags, qualifyTags("site", []string{p.Site})...)

	switch strings.TrimSpace(strings.ToLower(p.Rating)) {
	case "g":
		tags = append(tags, "rating:safe")
	case "s":
		tags = append(tags, "rating:sensitive")
	case "q":
		tags = append(tags, "rating:questionable")
	case "e":
		tags = append(tags, "rating:explicit")
	}

	return compactStrings(tags)
}

func (e *DanbooruEngine) fetchPost(ctx context.Context, rawURL string) (*danbooruPost, error) {
	postID, apiURL, err := danbooruAPIURL(rawURL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("danbooru api request: %w", err)
	}
	req.Header.Set("User-Agent", "Tanuki/1.0")
	req.Header.Set("Referer", sourceBaseURL(rawURL))

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("danbooru api fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("danbooru api status: %d", resp.StatusCode)
	}

	var post danbooruPost
	if err := json.NewDecoder(resp.Body).Decode(&post); err != nil {
		return nil, fmt.Errorf("danbooru api decode: %w", err)
	}
	post.Site = "danbooru"
	if post.ID == 0 {
		post.ID = postID
	}
	return &post, nil
}

var danbooruPostIDRe = regexp.MustCompile(`^posts/(\d+)(?:\.json)?$`)

func danbooruAPIURL(rawURL string) (int, string, error) {
	u, err := urlpkg.Parse(rawURL)
	if err != nil {
		return 0, "", fmt.Errorf("parse danbooru url: %w", err)
	}
	matches := danbooruPostIDRe.FindStringSubmatch(strings.Trim(u.Path, "/"))
	if len(matches) < 2 {
		return 0, "", newUnsupportedURLError("danbooru", "unsupported post url")
	}
	postID := matches[1]
	apiURL := fmt.Sprintf("%s://%s/posts/%s.json", u.Scheme, u.Host, postID)
	var id int
	_, _ = fmt.Sscanf(postID, "%d", &id)
	return id, apiURL, nil
}

func tagsFromString(raw string) []string {
	parts := strings.Fields(strings.TrimSpace(raw))
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(strings.ReplaceAll(part, "_", " "))
		if tag == "" {
			continue
		}
		tags = append(tags, tag)
	}
	return tags
}

func firstTagFromString(raw string) string {
	tags := tagsFromString(raw)
	if len(tags) == 0 {
		return ""
	}
	return tags[0]
}

func sourceBaseURL(rawURL string) string {
	u, err := urlpkg.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return fmt.Sprintf("%s://%s/", u.Scheme, u.Host)
}
