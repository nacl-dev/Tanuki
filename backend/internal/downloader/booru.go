package downloader

import (
	"context"
	"encoding/xml"
	"fmt"
	htmlstd "html"
	"net/http"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
	xhtml "golang.org/x/net/html"
)

type BooruEngine struct {
	client   *http.Client
	log      *zap.Logger
	progress func(id string, downloaded, total int64, files, totalFiles int)
}

func NewBooruEngine(log *zap.Logger) *BooruEngine {
	return &BooruEngine{
		client: &http.Client{},
		log:    log,
	}
}

func (e *BooruEngine) SetProgressUpdater(fn func(id string, downloaded, total int64, files, totalFiles int)) {
	e.progress = fn
}

func (e *BooruEngine) CanHandle(rawURL string) bool {
	u, err := urlpkg.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "gelbooru.com", "www.gelbooru.com", "ja.gelbooru.com", "safebooru.org", "www.safebooru.org":
	default:
		return false
	}

	query := u.Query()
	return query.Get("page") == "post" && query.Get("s") == "view" && query.Get("id") != ""
}

func (e *BooruEngine) Download(ctx context.Context, job *models.DownloadJob) error {
	dest := job.TargetDirectory
	if dest == "" {
		dest = "/downloads"
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("booru mkdir: %w", err)
	}

	post, err := e.fetchPost(ctx, job.URL)
	if err != nil {
		return err
	}
	if strings.TrimSpace(post.FileURL) == "" {
		return newUnsupportedURLError("booru", "post has no downloadable file")
	}

	filename := sanitizeArchiveName(post.Title())
	if filename == "" {
		filename = fmt.Sprintf("booru-%s", post.ID)
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(post.FileURL)), ".")
	if ext == "" {
		ext = "jpg"
	}
	finalPath := filepath.Join(dest, filename+"."+ext)
	tmpPath := finalPath + ".tmp"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, post.FileURL, nil)
	if err != nil {
		return fmt.Errorf("booru file request: %w", err)
	}
	req.Header.Set("User-Agent", "Tanuki/1.0")
	req.Header.Set("Referer", sourceBaseURL(job.URL))

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("booru file fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("booru file status: %d", resp.StatusCode)
	}

	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("booru create file: %w", err)
	}

	written, copyErr := copyWithProgress(resp.Body, out, resp.ContentLength, func(downloaded int64, total int64) {
		if e.progress != nil {
			e.progress(job.ID, downloaded, total, 0, 1)
		}
	})
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("booru write file: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("booru close file: %w", closeErr)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("booru rename file: %w", err)
	}

	sidecar := models.ImportMetadata{
		Title:     post.Title(),
		SourceURL: job.URL,
		PosterURL: post.PreviewURL,
		Tags:      post.AllTags(),
	}
	if err := writeImportMetadata(finalPath, sidecar); err != nil {
		e.log.Warn("booru: write metadata sidecar failed", zap.String("path", finalPath), zap.Error(err))
	}

	e.log.Info("booru: downloaded", zap.String("path", finalPath), zap.Int64("bytes", written))
	return nil
}

func (e *BooruEngine) FetchMetadata(rawURL string) (*SourceMetadata, error) {
	post, err := e.fetchPost(context.Background(), rawURL)
	if err != nil {
		return nil, err
	}

	extra := map[string]string{
		"source_url": rawURL,
	}
	if strings.TrimSpace(post.PreviewURL) != "" {
		extra["poster_url"] = post.PreviewURL
	}

	return &SourceMetadata{
		Title:      post.Title(),
		Tags:       post.AllTags(),
		TotalFiles: 1,
		Extra:      extra,
	}, nil
}

type booruPost struct {
	ID         string
	FileURL    string
	PreviewURL string
	Rating     string
	General    []string
	Artists    []string
	Characters []string
	Copyrights []string
	Meta       []string
}

func (p *booruPost) Title() string {
	copyright := firstTag(p.Copyrights)
	artist := firstTag(p.Artists)
	switch {
	case copyright != "" && artist != "":
		return fmt.Sprintf("%s by %s #%s", copyright, artist, p.ID)
	case copyright != "":
		return fmt.Sprintf("%s #%s", copyright, p.ID)
	case artist != "":
		return fmt.Sprintf("%s #%s", artist, p.ID)
	default:
		return fmt.Sprintf("Booru Post #%s", p.ID)
	}
}

func (p *booruPost) AllTags() []string {
	tags := make([]string, 0, len(p.General)+len(p.Artists)+len(p.Characters)+len(p.Copyrights)+len(p.Meta)+1)
	tags = append(tags, p.General...)
	tags = append(tags, p.Artists...)
	tags = append(tags, p.Characters...)
	tags = append(tags, p.Copyrights...)
	tags = append(tags, p.Meta...)

	switch strings.TrimSpace(strings.ToLower(p.Rating)) {
	case "g", "safe":
		tags = append(tags, "rating:safe")
	case "s", "sensitive":
		tags = append(tags, "rating:sensitive")
	case "q", "questionable":
		tags = append(tags, "rating:questionable")
	case "e", "explicit":
		tags = append(tags, "rating:explicit")
	}

	return compactStrings(tags)
}

func (e *BooruEngine) fetchPost(ctx context.Context, rawURL string) (*booruPost, error) {
	u, err := urlpkg.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse booru url: %w", err)
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "safebooru.org", "www.safebooru.org":
		return e.fetchSafebooruPost(ctx, u)
	case "gelbooru.com", "www.gelbooru.com", "ja.gelbooru.com":
		return e.fetchGelbooruPost(ctx, rawURL)
	default:
		return nil, newUnsupportedURLError("booru", "unsupported booru host")
	}
}

type safebooruResponse struct {
	Posts []struct {
		ID         string `xml:"id,attr"`
		FileURL    string `xml:"file_url,attr"`
		PreviewURL string `xml:"preview_url,attr"`
		Rating     string `xml:"rating,attr"`
		Tags       string `xml:"tags,attr"`
	} `xml:"post"`
}

func (e *BooruEngine) fetchSafebooruPost(ctx context.Context, u *urlpkg.URL) (*booruPost, error) {
	query := u.Query()
	postID := query.Get("id")
	apiURL := fmt.Sprintf("%s://%s/index.php?page=dapi&s=post&q=index&id=%s", u.Scheme, u.Host, postID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("safebooru api request: %w", err)
	}
	req.Header.Set("User-Agent", "Tanuki/1.0")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("safebooru api fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("safebooru api status: %d", resp.StatusCode)
	}

	var data safebooruResponse
	if err := xml.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("safebooru api decode: %w", err)
	}
	if len(data.Posts) == 0 {
		return nil, newUnsupportedURLError("booru", "safebooru post not found")
	}

	item := data.Posts[0]
	return &booruPost{
		ID:         item.ID,
		FileURL:    strings.TrimSpace(item.FileURL),
		PreviewURL: strings.TrimSpace(item.PreviewURL),
		Rating:     strings.TrimSpace(item.Rating),
		General:    parseBooruTagString(item.Tags),
	}, nil
}

func (e *BooruEngine) fetchGelbooruPost(ctx context.Context, rawURL string) (*booruPost, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("gelbooru page request: %w", err)
	}
	req.Header.Set("User-Agent", "Tanuki/1.0")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gelbooru page fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gelbooru page status: %d", resp.StatusCode)
	}

	doc, err := xhtml.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gelbooru page parse: %w", err)
	}

	post := &booruPost{}
	if u, parseErr := urlpkg.Parse(rawURL); parseErr == nil {
		post.ID = u.Query().Get("id")
	}
	post.FileURL = firstNonEmpty(
		extractMetaContent(doc, "property", "og:image"),
		extractOriginalImageLink(doc),
		extractImageSrc(doc),
	)
	post.PreviewURL = firstNonEmpty(
		extractMetaContent(doc, "property", "og:image"),
		extractMetaContent(doc, "name", "twitter:image"),
	)
	post.Rating = extractGelbooruRating(doc)
	post.Artists = extractGelbooruTags(doc, "tag-type-artist")
	post.Characters = extractGelbooruTags(doc, "tag-type-character")
	post.Copyrights = extractGelbooruTags(doc, "tag-type-copyright")
	post.Meta = extractGelbooruTags(doc, "tag-type-metadata")
	post.General = extractGelbooruKeywords(doc)

	post.General = subtractTags(post.General, post.Artists, post.Characters, post.Copyrights, post.Meta)
	return post, nil
}

func extractGelbooruTags(doc *xhtml.Node, className string) []string {
	tags := []string{}
	seen := map[string]struct{}{}

	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && n.Data == "li" {
			attrs := attrMap(n)
			if !strings.Contains(attrs["class"], className) {
				goto children
			}
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == xhtml.ElementNode && child.Data == "a" {
					href := attrMap(child)["href"]
					if strings.Contains(href, "page=post") && strings.Contains(href, "tags=") {
						name := strings.TrimSpace(htmlstd.UnescapeString(extractNodeText(child)))
						name = strings.ReplaceAll(name, "_", " ")
						if name != "" {
							key := strings.ToLower(name)
							if _, ok := seen[key]; !ok {
								seen[key] = struct{}{}
								tags = append(tags, name)
							}
						}
						break
					}
				}
			}
		}
	children:
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return tags
}

func extractGelbooruKeywords(doc *xhtml.Node) []string {
	keywords := extractMetaContent(doc, "name", "keywords")
	if keywords == "" {
		return nil
	}
	idx := strings.Index(keywords, "-")
	if idx >= 0 {
		keywords = keywords[idx+1:]
	}
	parts := strings.Split(keywords, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(htmlstd.UnescapeString(part))
		if tag == "" {
			continue
		}
		if strings.EqualFold(tag, "anime") ||
			strings.EqualFold(tag, "doujinshi") ||
			strings.EqualFold(tag, "hentai") ||
			strings.EqualFold(tag, "porn") ||
			strings.EqualFold(tag, "sex") ||
			strings.EqualFold(tag, "japanese hentai") ||
			strings.EqualFold(tag, "anime hentai") ||
			strings.EqualFold(tag, "rule34") ||
			strings.EqualFold(tag, "rule 34") ||
			strings.EqualFold(tag, "imageboard") {
			continue
		}
		out = append(out, strings.ReplaceAll(tag, "_", " "))
	}
	return compactStrings(out)
}

func extractOriginalImageLink(doc *xhtml.Node) string {
	var found string
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if found != "" {
			return
		}
		if n.Type == xhtml.ElementNode && n.Data == "a" {
			attrs := attrMap(n)
			if strings.Contains(strings.ToLower(extractNodeText(n)), "original image") {
				found = strings.TrimSpace(attrs["href"])
				return
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return normalizeBooruURL(found)
}

func extractImageSrc(doc *xhtml.Node) string {
	var found string
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if found != "" {
			return
		}
		if n.Type == xhtml.ElementNode && n.Data == "img" {
			attrs := attrMap(n)
			if attrs["id"] == "image" {
				found = strings.TrimSpace(attrs["src"])
				return
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return normalizeBooruURL(found)
}

func extractGelbooruRating(doc *xhtml.Node) string {
	var found string
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if found != "" {
			return
		}
		if n.Type == xhtml.ElementNode && n.Data == "section" {
			attrs := attrMap(n)
			if attrs["id"] == "image-container" {
				found = strings.TrimSpace(attrs["data-rating"])
				return
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return found
}

func extractNodeText(node *xhtml.Node) string {
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.TextNode {
			b.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return b.String()
}

func parseBooruTagString(raw string) []string {
	parts := strings.Fields(strings.TrimSpace(raw))
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(strings.ReplaceAll(part, "_", " "))
		if tag == "" {
			continue
		}
		tags = append(tags, tag)
	}
	return compactStrings(tags)
}

func subtractTags(source []string, groups ...[]string) []string {
	blocked := map[string]struct{}{}
	for _, group := range groups {
		for _, tag := range group {
			blocked[strings.ToLower(strings.TrimSpace(tag))] = struct{}{}
		}
	}
	out := make([]string, 0, len(source))
	for _, tag := range source {
		if _, ok := blocked[strings.ToLower(strings.TrimSpace(tag))]; ok {
			continue
		}
		out = append(out, tag)
	}
	return compactStrings(out)
}

func normalizeBooruURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	return strings.ReplaceAll(raw, "//images", "/images")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func firstTag(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}
