package downloader

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	htmlstd "html"
	"io"
	"net/http"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
	xhtml "golang.org/x/net/html"
)

type Rule34ArtEngine struct {
	client   *http.Client
	log      *zap.Logger
	progress func(id string, downloaded, total int64, files, totalFiles int)
}

func NewRule34ArtEngine(cookiesPath string, log *zap.Logger) *Rule34ArtEngine {
	client, err := newHTTPClientWithCookies(cookiesPath)
	if err != nil {
		log.Warn("rule34art: cookies unavailable", zap.String("path", cookiesPath), zap.Error(err))
		client = &http.Client{}
	}
	return &Rule34ArtEngine{
		client: client,
		log:    log,
	}
}

func (e *Rule34ArtEngine) SetProgressUpdater(fn func(id string, downloaded, total int64, files, totalFiles int)) {
	e.progress = fn
}

func (e *Rule34ArtEngine) CanHandle(rawURL string) bool {
	u, err := urlpkg.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host != "rule34.art" && host != "www.rule34.art" {
		return false
	}

	path := strings.Trim(strings.ToLower(u.Path), "/")
	return strings.HasPrefix(path, "comics/") || strings.HasPrefix(path, "video/")
}

func (e *Rule34ArtEngine) Download(ctx context.Context, job *models.DownloadJob) error {
	dest := job.TargetDirectory
	if dest == "" {
		dest = "/downloads"
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("rule34art mkdir: %w", err)
	}

	page, err := e.fetchPage(ctx, job.URL)
	if err != nil {
		return err
	}

	switch page.Kind {
	case "comic":
		return e.downloadComic(ctx, job, page, dest)
	case "video":
		return e.downloadVideo(ctx, job, page, dest)
	default:
		return newUnsupportedURLError("rule34art", "unsupported page type")
	}
}

func (e *Rule34ArtEngine) FetchMetadata(rawURL string) (*SourceMetadata, error) {
	page, err := e.fetchPage(context.Background(), rawURL)
	if err != nil {
		return nil, err
	}

	totalFiles := 1
	if page.Kind == "comic" {
		totalFiles = len(page.Images)
	}

	extra := map[string]string{
		"source_url": rawURL,
	}
	if strings.TrimSpace(page.PosterURL) != "" {
		extra["poster_url"] = page.PosterURL
	}

	return &SourceMetadata{
		Title:      page.Title,
		Tags:       page.Tags,
		TotalFiles: totalFiles,
		Extra:      extra,
	}, nil
}

type rule34ArtPage struct {
	Kind      string
	Title     string
	PosterURL string
	Tags      []string
	Images    []string
	VideoURL  string
}

func (e *Rule34ArtEngine) fetchPage(ctx context.Context, rawURL string) (*rule34ArtPage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("rule34art request: %w", err)
	}
	setRule34ArtHeaders(req, sourceBaseURL(rawURL), "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rule34art fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if isRule34ArtBlockedStatus(resp.StatusCode) {
			return nil, newUnsupportedURLError("rule34art", blockedSourceDetail(resp.StatusCode))
		}
		return nil, fmt.Errorf("rule34art status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("rule34art read: %w", err)
	}

	doc, err := xhtml.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("rule34art parse: %w", err)
	}

	pageURL, _ := urlpkg.Parse(rawURL)
	lowerPath := strings.ToLower(strings.Trim(pageURL.Path, "/"))
	if strings.HasPrefix(lowerPath, "comics/") {
		return e.extractComic(ctx, pageURL, string(body), doc)
	}
	if strings.HasPrefix(lowerPath, "video/") {
		return e.extractVideo(pageURL, doc, string(body))
	}

	if hasElementWithClass(doc, "juicebox-container") {
		return e.extractComic(ctx, pageURL, string(body), doc)
	}
	if hasElement(doc, "video") {
		return e.extractVideo(pageURL, doc, string(body))
	}

	return nil, newUnsupportedURLError("rule34art", "page is not a supported comic or video page")
}

func (e *Rule34ArtEngine) extractComic(ctx context.Context, pageURL *urlpkg.URL, body string, doc *xhtml.Node) (*rule34ArtPage, error) {
	xmlURL := extractRule34ArtJuiceboxURL(pageURL, body)
	if xmlURL == "" {
		return nil, newUnsupportedURLError("rule34art", "comic gallery xml not found")
	}

	gallery, err := e.fetchJuiceboxGallery(ctx, xmlURL)
	if err != nil {
		return nil, err
	}
	if len(gallery.Images) == 0 {
		return nil, newUnsupportedURLError("rule34art", "comic gallery has no images")
	}

	title := extractPageHeader(doc)
	if title == "" {
		title = cleanRule34ArtTitle(extractHTMLTitle(doc))
	}

	tags := compactStrings([]string{})
	tags = append(tags, buildRule34ArtComicTags(
		extractTextByClass(doc, "lang-cont"),
		extractFieldItemsByClass(doc, "field--name-field-com-author"),
		extractFieldItemsByClass(doc, "field--name-field-com-section"),
	)...)

	poster := gallery.Images[0].ThumbURL
	if poster == "" {
		poster = gallery.Images[0].UnstyledSrc
	}

	images := make([]string, 0, len(gallery.Images))
	for _, img := range gallery.Images {
		if strings.TrimSpace(img.UnstyledSrc) != "" {
			images = append(images, img.UnstyledSrc)
		}
	}

	return &rule34ArtPage{
		Kind:      "comic",
		Title:     title,
		PosterURL: poster,
		Tags:      tags,
		Images:    images,
	}, nil
}

func (e *Rule34ArtEngine) extractVideo(pageURL *urlpkg.URL, doc *xhtml.Node, body string) (*rule34ArtPage, error) {
	sources := extractVideoSources(doc, pageURL)
	if len(sources) == 0 {
		return nil, newUnsupportedURLError("rule34art", "video source not found")
	}
	sort.SliceStable(sources, func(i, j int) bool {
		return sources[i].Quality < sources[j].Quality
	})

	title := extractPageHeader(doc)
	if title == "" {
		title = cleanRule34ArtTitle(extractHTMLTitle(doc))
	}

	tags := buildRule34ArtVideoTags(
		extractFieldItemsByClass(doc, "field--name-field-vid-tags"),
		extractFieldItemsByClass(doc, "field--name-field-vid-author"),
		extractFieldItemsByClass(doc, "field--name-field-vid-section"),
	)

	return &rule34ArtPage{
		Kind:      "video",
		Title:     title,
		PosterURL: extractRule34ArtPoster(pageURL, body, doc),
		Tags:      tags,
		VideoURL:  sources[len(sources)-1].URL,
	}, nil
}

func buildRule34ArtComicTags(language string, authors, sections []string) []string {
	tags := compactStrings([]string{})
	tags = append(tags, qualifyTags("language", []string{language})...)
	tags = append(tags, qualifyTags("artist", authors)...)
	tags = append(tags, qualifyTags("site", sections)...)
	return compactStrings(tags)
}

func buildRule34ArtVideoTags(rawTags, authors, sections []string) []string {
	tags := append([]string{}, qualifyTags("genre", rawTags)...)
	tags = append(tags, qualifyTags("artist", authors)...)
	tags = append(tags, qualifyTags("site", sections)...)
	return compactStrings(tags)
}

func (e *Rule34ArtEngine) downloadComic(ctx context.Context, job *models.DownloadJob, page *rule34ArtPage, dest string) error {
	archiveName := sanitizeArchiveName(page.Title)
	if archiveName == "" {
		archiveName = sanitizeArchiveName(pathBaseWithoutExt(job.URL))
	}
	if archiveName == "" {
		archiveName = "rule34art-comic"
	}

	finalPath := filepath.Join(dest, archiveName+".cbz")
	tmpPath := finalPath + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("rule34art create cbz: %w", err)
	}

	zw := zip.NewWriter(f)
	for i, imageURL := range page.Images {
		entryName, err := archiveEntryName(i+1, imageURL)
		if err != nil {
			zw.Close()
			f.Close()
			_ = os.Remove(tmpPath)
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
		if err != nil {
			zw.Close()
			f.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("rule34art image request %d: %w", i+1, err)
		}
		setRule34ArtHeaders(req, job.URL, "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")

		resp, err := e.client.Do(req)
		if err != nil {
			zw.Close()
			f.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("rule34art image fetch %d: %w", i+1, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			zw.Close()
			f.Close()
			_ = os.Remove(tmpPath)
			if isRule34ArtBlockedStatus(resp.StatusCode) {
				return newUnsupportedURLError("rule34art", blockedSourceDetail(resp.StatusCode))
			}
			return fmt.Errorf("rule34art image status %d: %d", i+1, resp.StatusCode)
		}

		w, err := zw.Create(entryName)
		if err != nil {
			resp.Body.Close()
			zw.Close()
			f.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("rule34art cbz entry %d: %w", i+1, err)
		}
		if _, err := io.Copy(w, resp.Body); err != nil {
			resp.Body.Close()
			zw.Close()
			f.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("rule34art cbz write %d: %w", i+1, err)
		}
		resp.Body.Close()

		if e.progress != nil {
			e.progress(job.ID, 0, 0, i+1, len(page.Images))
		}
	}

	if err := zw.Close(); err != nil {
		f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rule34art close cbz: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rule34art close file: %w", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rule34art rename cbz: %w", err)
	}

	if err := writeImportMetadata(finalPath, models.ImportMetadata{
		Title:     page.Title,
		SourceURL: job.URL,
		PosterURL: page.PosterURL,
		Tags:      page.Tags,
	}); err != nil {
		e.log.Warn("rule34art: write comic metadata failed", zap.String("path", finalPath), zap.Error(err))
	}

	e.log.Info("rule34art: comic archived", zap.String("path", finalPath), zap.Int("pages", len(page.Images)))
	return nil
}

func (e *Rule34ArtEngine) downloadVideo(ctx context.Context, job *models.DownloadJob, page *rule34ArtPage, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, page.VideoURL, nil)
	if err != nil {
		return fmt.Errorf("rule34art video request: %w", err)
	}
	setRule34ArtHeaders(req, job.URL, "video/*,*/*;q=0.8")

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("rule34art video fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if isRule34ArtBlockedStatus(resp.StatusCode) {
			return newUnsupportedURLError("rule34art", blockedSourceDetail(resp.StatusCode))
		}
		return fmt.Errorf("rule34art video status: %d", resp.StatusCode)
	}

	fileName := sanitizeArchiveName(page.Title)
	if fileName == "" {
		fileName = sanitizeArchiveName(pathBaseWithoutExt(job.URL))
	}
	if fileName == "" {
		fileName = "rule34art-video"
	}

	ext := strings.ToLower(filepath.Ext(page.VideoURL))
	if ext == "" {
		ext = ".mp4"
	}
	finalPath := filepath.Join(dest, fileName+ext)
	tmpPath := finalPath + ".tmp"

	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("rule34art create video: %w", err)
	}

	written, copyErr := copyWithProgress(resp.Body, out, resp.ContentLength, func(downloaded int64, total int64) {
		if e.progress != nil {
			e.progress(job.ID, downloaded, total, 0, 1)
		}
	})
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rule34art write video: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rule34art close video: %w", closeErr)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rule34art rename video: %w", err)
	}

	if err := writeImportMetadata(finalPath, models.ImportMetadata{
		Title:     page.Title,
		SourceURL: job.URL,
		PosterURL: page.PosterURL,
		Tags:      page.Tags,
	}); err != nil {
		e.log.Warn("rule34art: write video metadata failed", zap.String("path", finalPath), zap.Error(err))
	}

	e.log.Info("rule34art: video downloaded", zap.String("path", finalPath), zap.Int64("bytes", written))
	return nil
}

type rule34ArtJuicebox struct {
	Images []struct {
		UnstyledSrc string `xml:"unstyled_src,attr"`
		ThumbURL    string `xml:"thumbURL,attr"`
	} `xml:"image"`
}

func (e *Rule34ArtEngine) fetchJuiceboxGallery(ctx context.Context, rawURL string) (*rule34ArtJuicebox, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("rule34art gallery request: %w", err)
	}
	setRule34ArtHeaders(req, sourceBaseURL(rawURL), "application/xml,text/xml;q=0.9,*/*;q=0.8")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rule34art gallery fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if isRule34ArtBlockedStatus(resp.StatusCode) {
			return nil, newUnsupportedURLError("rule34art", blockedSourceDetail(resp.StatusCode))
		}
		return nil, fmt.Errorf("rule34art gallery status: %d", resp.StatusCode)
	}

	var gallery rule34ArtJuicebox
	if err := xml.NewDecoder(resp.Body).Decode(&gallery); err != nil {
		return nil, fmt.Errorf("rule34art gallery decode: %w", err)
	}
	return &gallery, nil
}

var (
	rule34ArtConfigURLRe = regexp.MustCompile(`configUrl":"([^"]+)"`)
	rule34ArtPosterRe    = regexp.MustCompile(`posterImage:\s*"([^"]+)"`)
)

func extractRule34ArtJuiceboxURL(pageURL *urlpkg.URL, body string) string {
	match := rule34ArtConfigURLRe.FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	raw := strings.TrimSpace(match[1])
	raw = strings.ReplaceAll(raw, `\/`, `/`)
	raw = strings.ReplaceAll(raw, `\u0026`, `&`)
	return resolveImageURL(pageURL, raw)
}

func extractRule34ArtPoster(pageURL *urlpkg.URL, body string, doc *xhtml.Node) string {
	match := rule34ArtPosterRe.FindStringSubmatch(body)
	if len(match) >= 2 {
		raw := strings.TrimSpace(match[1])
		raw = strings.ReplaceAll(raw, `\/`, `/`)
		if resolved := resolveImageURL(pageURL, raw); resolved != "" {
			return resolved
		}
	}

	var poster string
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if poster != "" {
			return
		}
		if n.Type == xhtml.ElementNode && n.Data == "video" {
			attrs := attrMap(n)
			if value := strings.TrimSpace(attrs["poster"]); value != "" {
				poster = resolveImageURL(pageURL, value)
				return
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return poster
}

func extractPageHeader(doc *xhtml.Node) string {
	var title string
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if title != "" {
			return
		}
		if n.Type == xhtml.ElementNode && n.Data == "h1" {
			attrs := attrMap(n)
			if strings.Contains(attrs["class"], "page-header") {
				title = strings.TrimSpace(extractNodeText(n))
				return
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return htmlstd.UnescapeString(title)
}

func hasElement(doc *xhtml.Node, name string) bool {
	found := false
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if found {
			return
		}
		if n.Type == xhtml.ElementNode && n.Data == name {
			found = true
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return found
}

func hasElementWithClass(doc *xhtml.Node, classFragment string) bool {
	found := false
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if found {
			return
		}
		if n.Type == xhtml.ElementNode && strings.Contains(attrMap(n)["class"], classFragment) {
			found = true
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return found
}

func extractTextByClass(doc *xhtml.Node, classFragment string) string {
	var text string
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if text != "" {
			return
		}
		if n.Type == xhtml.ElementNode && strings.Contains(attrMap(n)["class"], classFragment) {
			text = strings.TrimSpace(extractNodeText(n))
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return text
}

func extractFieldItemsByClass(doc *xhtml.Node, classFragment string) []string {
	var root *xhtml.Node
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if root != nil {
			return
		}
		if n.Type == xhtml.ElementNode && strings.Contains(attrMap(n)["class"], classFragment) {
			root = n
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if root == nil {
		return nil
	}

	values := make([]string, 0, 8)
	var collect func(*xhtml.Node)
	collect = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && strings.Contains(attrMap(n)["class"], "field--item") {
			if value := strings.TrimSpace(extractNodeText(n)); value != "" {
				values = append(values, value)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			collect(child)
		}
	}
	collect(root)
	return compactStrings(values)
}

type rule34ArtVideoSource struct {
	URL     string
	Quality int
}

func extractVideoSources(doc *xhtml.Node, pageURL *urlpkg.URL) []rule34ArtVideoSource {
	sources := make([]rule34ArtVideoSource, 0, 4)
	seen := map[string]struct{}{}

	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && n.Data == "source" {
			attrs := attrMap(n)
			resolved := resolveImageURL(pageURL, attrs["src"])
			if resolved != "" {
				if _, ok := seen[resolved]; !ok {
					seen[resolved] = struct{}{}
					sources = append(sources, rule34ArtVideoSource{
						URL:     resolved,
						Quality: parseRule34ArtQuality(attrs["title"]),
					})
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return sources
}

func parseRule34ArtQuality(value string) int {
	value = strings.TrimSpace(strings.TrimSuffix(strings.ToLower(value), "p"))
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return n
}

func cleanRule34ArtTitle(value string) string {
	value = htmlstd.UnescapeString(strings.TrimSpace(value))
	for _, suffix := range []string{
		" Porn comic, Cartoon porn comics, Rule 34 comic",
		" Cartoon porn video, Rule 34 animated",
		" | Rule 34 animated, Porn animations",
	} {
		value = strings.TrimSuffix(value, suffix)
	}
	return strings.TrimSpace(value)
}

func setRule34ArtHeaders(req *http.Request, referer, accept string) {
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
	}
}

func isRule34ArtBlockedStatus(status int) bool {
	return status == http.StatusForbidden ||
		status == http.StatusUnauthorized ||
		status == http.StatusTooManyRequests
}
