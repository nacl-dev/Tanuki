package downloader

import (
	"archive/zip"
	"context"
	"fmt"
	htmlstd "html"
	"io"
	"net/http"
	urlpkg "net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
	xhtml "golang.org/x/net/html"
)

// ImageGalleryEngine downloads manga/gallery pages rendered as individual images
// in the HTML reader and bundles them into a CBZ archive.
type ImageGalleryEngine struct {
	client *http.Client
	log    *zap.Logger
}

func NewImageGalleryEngine(log *zap.Logger) *ImageGalleryEngine {
	return &ImageGalleryEngine{
		client: &http.Client{},
		log:    log,
	}
}

func (e *ImageGalleryEngine) CanHandle(rawURL string) bool {
	u, err := urlpkg.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "doujins.com", "www.doujins.com":
		return true
	default:
		return false
	}
}

func (e *ImageGalleryEngine) Download(ctx context.Context, job *models.DownloadJob) error {
	dest := job.TargetDirectory
	if dest == "" {
		dest = "/downloads"
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("gallery page mkdir: %w", err)
	}

	gallery, err := e.extractGallery(ctx, job.URL)
	if err != nil {
		return err
	}
	if len(gallery.Images) == 0 {
		return newUnsupportedURLError("image-gallery", "no gallery images found")
	}

	archiveName := sanitizeArchiveName(gallery.Title)
	if archiveName == "" {
		archiveName = sanitizeArchiveName(path.Base(strings.TrimSuffix(job.URL, "/")))
	}
	if archiveName == "" {
		archiveName = "gallery"
	}

	finalPath := filepath.Join(dest, archiveName+".cbz")
	tmpPath := finalPath + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create cbz: %w", err)
	}

	zipWriter := zip.NewWriter(f)
	writeErr := e.writeGalleryArchive(ctx, zipWriter, gallery, job.URL)
	closeErr := zipWriter.Close()
	fileCloseErr := f.Close()
	if writeErr != nil {
		_ = os.Remove(tmpPath)
		return writeErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close cbz: %w", closeErr)
	}
	if fileCloseErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close cbz file: %w", fileCloseErr)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename cbz: %w", err)
	}

	sidecar := models.ImportMetadata{
		Title:     gallery.Title,
		SourceURL: job.URL,
		Tags:      gallery.Tags,
	}
	if err := writeImportMetadata(finalPath, sidecar); err != nil {
		e.log.Warn("image-gallery: write metadata sidecar failed", zap.String("path", finalPath), zap.Error(err))
	}

	e.log.Info("image-gallery: archived", zap.String("path", finalPath), zap.Int("pages", len(gallery.Images)))
	return nil
}

func (e *ImageGalleryEngine) FetchMetadata(rawURL string) (*SourceMetadata, error) {
	gallery, err := e.extractGallery(context.Background(), rawURL)
	if err != nil {
		return nil, err
	}
	return &SourceMetadata{
		Title:      gallery.Title,
		Tags:       gallery.Tags,
		TotalFiles: len(gallery.Images),
		Extra: map[string]string{
			"source_url": rawURL,
		},
	}, nil
}

type imageGallery struct {
	Title  string
	Tags   []string
	Images []string
}

func (e *ImageGalleryEngine) extractGallery(ctx context.Context, rawURL string) (*imageGallery, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("gallery page request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Tanuki)")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gallery page fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gallery page status: %d", resp.StatusCode)
	}

	doc, err := xhtml.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gallery page parse: %w", err)
	}

	u, _ := urlpkg.Parse(rawURL)
	title := strings.TrimSpace(extractHTMLTitle(doc))
	images := extractDoujinsImages(doc, u)
	tags := extractDoujinsTags(doc)
	if len(images) == 0 {
		return nil, newUnsupportedURLError("image-gallery", "reader markup not found")
	}

	return &imageGallery{Title: title, Tags: tags, Images: images}, nil
}

func (e *ImageGalleryEngine) writeGalleryArchive(ctx context.Context, zw *zip.Writer, gallery *imageGallery, referer string) error {
	for i, imageURL := range gallery.Images {
		pageName, err := archiveEntryName(i+1, imageURL)
		if err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
		if err != nil {
			return fmt.Errorf("page request %d: %w", i+1, err)
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Tanuki)")
		req.Header.Set("Referer", referer)

		resp, err := e.client.Do(req)
		if err != nil {
			return fmt.Errorf("page fetch %d: %w", i+1, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("page fetch %d: status %d", i+1, resp.StatusCode)
		}

		w, err := zw.Create(pageName)
		if err != nil {
			resp.Body.Close()
			return fmt.Errorf("cbz entry %d: %w", i+1, err)
		}
		if _, err := io.Copy(w, resp.Body); err != nil {
			resp.Body.Close()
			return fmt.Errorf("cbz write %d: %w", i+1, err)
		}
		resp.Body.Close()
	}
	return nil
}

func extractHTMLTitle(node *xhtml.Node) string {
	var title string
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if title != "" {
			return
		}
		if n.Type == xhtml.ElementNode && n.Data == "title" && n.FirstChild != nil {
			title = htmlstd.UnescapeString(strings.TrimSpace(n.FirstChild.Data))
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return title
}

func extractDoujinsImages(doc *xhtml.Node, pageURL *urlpkg.URL) []string {
	seen := map[string]struct{}{}
	images := make([]string, 0, 32)

	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && n.Data == "img" {
			attrs := attrMap(n)
			if strings.Contains(attrs["class"], "swiper-lazy") || strings.HasPrefix(attrs["id"], "swiper-") {
				raw := attrs["data-src"]
				if raw == "" {
					raw = attrs["src"]
				}
				if raw != "" {
					resolved := resolveImageURL(pageURL, raw)
					if resolved != "" && strings.Contains(resolved, "static.doujins.com/") {
						if _, ok := seen[resolved]; !ok {
							seen[resolved] = struct{}{}
							images = append(images, resolved)
						}
					}
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return images
}

func attrMap(n *xhtml.Node) map[string]string {
	m := make(map[string]string, len(n.Attr))
	for _, attr := range n.Attr {
		m[attr.Key] = attr.Val
	}
	return m
}

func resolveImageURL(pageURL *urlpkg.URL, raw string) string {
	if raw == "" {
		return ""
	}
	raw = htmlstd.UnescapeString(strings.TrimSpace(raw))
	u, err := urlpkg.Parse(raw)
	if err != nil {
		return ""
	}
	if pageURL != nil {
		u = pageURL.ResolveReference(u)
	}
	return u.String()
}

func archiveEntryName(index int, rawURL string) (string, error) {
	u, err := urlpkg.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse page url: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(u.Path))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
	default:
		ext = ".jpg"
	}
	return fmt.Sprintf("%03d%s", index, ext), nil
}

func extractDoujinsTags(doc *xhtml.Node) []string {
	description := extractMetaContent(doc, "name", "description")
	if description == "" {
		return []string{"site:doujins.com"}
	}

	lower := strings.ToLower(description)
	idx := strings.Index(lower, "tags:")
	if idx < 0 {
		return []string{"site:doujins.com"}
	}

	raw := strings.TrimSpace(description[idx+len("tags:"):])
	raw = strings.TrimSuffix(raw, ".")
	raw = strings.ReplaceAll(raw, ", and ", ", ")
	raw = strings.ReplaceAll(raw, " and ", ", ")

	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts)+1)
	tags = append(tags, "site:doujins.com")
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(htmlstd.UnescapeString(part))
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		tags = append(tags, qualifyTags("genre", []string{tag})...)
	}
	return compactStrings(tags)
}

func extractMetaContent(node *xhtml.Node, attrName, attrValue string) string {
	var content string
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if content != "" {
			return
		}
		if n.Type == xhtml.ElementNode && n.Data == "meta" {
			attrs := attrMap(n)
			if strings.EqualFold(attrs[attrName], attrValue) {
				content = strings.TrimSpace(attrs["content"])
				return
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return content
}

var invalidArchiveChars = regexp.MustCompile(`[<>:"/\\|?*]+`)
var whitespaceChars = regexp.MustCompile(`\s+`)

func sanitizeArchiveName(name string) string {
	name = htmlstd.UnescapeString(strings.TrimSpace(name))
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = invalidArchiveChars.ReplaceAllString(name, " ")
	name = whitespaceChars.ReplaceAllString(name, " ")
	name = strings.Trim(name, " .")
	return name
}
