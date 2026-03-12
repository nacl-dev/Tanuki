package downloader

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	htmlstd "html"
	"net/http"
	"net/http/httptest"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
)

func TestHDoujinLuaEngineFetchMetadataFromSimpleModule(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gallery":
			fmt.Fprint(w, `<html><body><h1>Example Gallery | Site</h1><ul><li>Tag One</li><li>Tag Two</li></ul><img src="/img/1.jpg"><img src="/img/2.jpg"></body></html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	host := mustTestHostname(t, server.URL)
	modulesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(modulesDir, "Example.lua"), []byte(fmt.Sprintf(`
function Register()
  module.Name = 'Example'
  module.Domains.Add('%s')
end

function GetInfo()
  info.Title = dom.SelectValue('//h1'):before('|'):trim()
  info.Tags = dom.SelectValues('//ul/li')
end

function GetPages()
  pages.AddRange(page.SelectValues('//img/@src'))
end
`, host)), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}

	engine := NewHDoujinLuaEngine(modulesDir, "", zap.NewNop())
	if !engine.CanHandle(server.URL + "/gallery") {
		t.Fatal("expected engine to match local module domain")
	}

	meta, err := engine.FetchMetadata(server.URL + "/gallery")
	if err != nil {
		t.Fatalf("fetch metadata: %v", err)
	}
	if meta.Title != "Example Gallery" {
		t.Fatalf("unexpected title: %q", meta.Title)
	}
	if meta.TotalFiles != 2 {
		t.Fatalf("expected 2 pages, got %d", meta.TotalFiles)
	}
	if !slicesContains(meta.Tags, "genre:Tag One") || !slicesContains(meta.Tags, "site:example") {
		t.Fatalf("unexpected tags: %#v", meta.Tags)
	}
}

func TestHDoujinLuaEngineDownloadUsesBeforeDownloadPage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gallery":
			fmt.Fprint(w, `<html><body><h1>Example Set</h1><a class="page" href="/post/1">1</a><a class="page" href="/post/2">2</a></body></html>`)
		case "/post/1":
			fmt.Fprint(w, `<html><body><img src="/img/1.jpg"></body></html>`)
		case "/post/2":
			fmt.Fprint(w, `<html><body><img src="/img/2.jpg"></body></html>`)
		case "/img/1.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("jpeg-one"))
		case "/img/2.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("jpeg-two"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	host := mustTestHostname(t, server.URL)
	modulesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(modulesDir, "Paged.lua"), []byte(fmt.Sprintf(`
function Register()
  module.Name = 'Paged'
  module.Domains.Add('%s')
end

function GetInfo()
  info.Title = dom.SelectValue('//h1')
  info.Tags = {'Demo'}
end

function GetPages()
  pages.AddRange(page.SelectValues('//a[contains(@class,"page")]/@href'))
end

function BeforeDownloadPage()
  page.Url = page.SelectValue('//img/@src')
end
`, host)), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}

	engine := NewHDoujinLuaEngine(modulesDir, "", zap.NewNop())
	dest := t.TempDir()
	job := &models.DownloadJob{
		ID:              "job-1",
		URL:             server.URL + "/gallery",
		TargetDirectory: dest,
	}
	if err := engine.Download(t.Context(), job); err != nil {
		t.Fatalf("download: %v", err)
	}

	archivePath := filepath.Join(dest, "Example Set.cbz")
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("expected archive to exist: %v", err)
	}
	if _, err := os.Stat(archivePath + ".tanuki.json"); err != nil {
		t.Fatalf("expected metadata sidecar to exist: %v", err)
	}

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer reader.Close()
	if len(reader.File) != 2 {
		t.Fatalf("expected 2 archive entries, got %d", len(reader.File))
	}
}

func TestHDoujinLuaEngineDownloadChapterSeriesFromJSONModule(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manga/demo-series":
			writeHDoujinJSONPage(t, w, map[string]any{
				"props": map[string]any{
					"manga": map[string]any{
						"name":       "Demo Series",
						"other_name": "Demo Alt",
						"pilot":      "Demo Summary",
						"status_id":  0,
						"slug":       "demo-series",
						"genres":     []map[string]any{{"name": "Drama"}},
						"artists":    []map[string]any{{"name": "Artist A"}},
						"characters": []map[string]any{{"name": "Lead"}},
					},
					"chapters": []map[string]any{
						{"slug": "chap-02", "name": "chap 02"},
						{"slug": "chap-01", "name": "chap 01"},
					},
				},
			})
		case "/manga/demo-series/chap-01":
			writeHDoujinJSONPage(t, w, map[string]any{
				"props": map[string]any{
					"mangaName":      "Demo Series",
					"chapterName":    "chap 01",
					"chapterContent": `<img src="/img/1a.jpg"><img src="/img/1b.jpg">`,
				},
			})
		case "/manga/demo-series/chap-02":
			writeHDoujinJSONPage(t, w, map[string]any{
				"props": map[string]any{
					"mangaName":      "Demo Series",
					"chapterName":    "chap 02",
					"chapterContent": `<img src="/img/2a.jpg"><img src="/img/2b.jpg">`,
				},
			})
		case "/img/1a.jpg", "/img/1b.jpg", "/img/2a.jpg", "/img/2b.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("jpeg"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	host := mustTestHostname(t, server.URL)
	modulesDir := t.TempDir()
	moduleBody := fmt.Sprintf(`
local publishingStatusLookup = {
  ["0"] = "Ongoing",
}

local function getPageDataJson()
  local pageDataStr = dom:SelectValue('//div[@id="app"]/@data-page')
  local js = JavaScript.New()
  local pageDataObject = js:Execute("pageData = " .. pageDataStr)
  return pageDataObject:ToJson()
end

function Register()
  module.Name = "Manhwa18"
  module.Adult = true
  module.Language = "en"
  module.Type = "Manhwa"
  module.Domains:Add("%s")
end

function GetInfo()
  local json = getPageDataJson()

  info.Title = json:SelectValue("props.manga.name")
  info.AlternativeTitle = json:SelectValue("props.manga.other_name")
  info.Summary = json:SelectValue("props.manga.pilot")
  info.Status = publishingStatusLookup[tostring(json:SelectValue("props.manga.status_id"))]
  info.Tags = json:SelectValues("props.manga.genres[*].name")
  info.Artist = json:SelectValues("props.manga.artists[*].name")
  info.Characters = json:SelectValues("props.manga.characters[*].name")

  if isempty(info.Title) then
    local mangaName = json:SelectValue("props.mangaName")
    local chapterName = json:SelectValue("props.chapterName")
    info.Title = mangaName .. " - " .. chapterName
  end
end

function GetChapters()
  local json = getPageDataJson()
  local gallerySlug = json:SelectValue("props.manga.slug")
  for chapterNode in json:SelectNodes("props.chapters[*]") do
    local chapterSlug = chapterNode:SelectValue("slug")
    local chapterUrl = "/manga/" .. gallerySlug .. "/" .. chapterSlug
    local chapterTitle = chapterNode:SelectValue("name")
    chapters:Add(chapterUrl, chapterTitle)
  end
  chapters:Reverse()
end

function GetPages()
  local json = getPageDataJson()
  dom = Dom.New(json:SelectValue("props.chapterContent"))
  pages:AddRange(dom:SelectValues("//img/@src"))
end
`, host)
	if err := os.WriteFile(filepath.Join(modulesDir, "Manhwa18.lua"), []byte(moduleBody), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}

	engine := NewHDoujinLuaEngine(modulesDir, "", zap.NewNop())
	meta, err := engine.FetchMetadata(server.URL + "/manga/demo-series")
	if err != nil {
		t.Fatalf("fetch metadata: %v", err)
	}
	if meta.Title != "Demo Series" {
		t.Fatalf("unexpected title: %q", meta.Title)
	}
	if meta.TotalFiles != 2 {
		t.Fatalf("expected 2 chapters, got %d", meta.TotalFiles)
	}

	dest := t.TempDir()
	job := &models.DownloadJob{
		ID:              "job-series",
		URL:             server.URL + "/manga/demo-series",
		TargetDirectory: dest,
	}
	if err := engine.Download(t.Context(), job); err != nil {
		t.Fatalf("download series: %v", err)
	}

	firstArchive := filepath.Join(dest, "Demo Series - chap 01.cbz")
	secondArchive := filepath.Join(dest, "Demo Series - chap 02.cbz")
	for _, archivePath := range []string{firstArchive, secondArchive} {
		if _, err := os.Stat(archivePath); err != nil {
			t.Fatalf("expected archive %s to exist: %v", archivePath, err)
		}
		if _, err := os.Stat(archivePath + ".tanuki.json"); err != nil {
			t.Fatalf("expected sidecar for %s: %v", archivePath, err)
		}
	}

	sidecarBody, err := os.ReadFile(firstArchive + ".tanuki.json")
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	if !strings.Contains(string(sidecarBody), `"work_title": "Demo Series"`) {
		t.Fatalf("expected work_title in sidecar, got %s", string(sidecarBody))
	}
}

func writeHDoujinJSONPage(t *testing.T, w http.ResponseWriter, payload map[string]any) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	fmt.Fprintf(w, `<html><body><div id="app" data-page="%s"></div></body></html>`, htmlstd.EscapeString(string(body)))
}

func mustTestHostname(t *testing.T, raw string) string {
	t.Helper()
	u, err := urlpkg.Parse(raw)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	return u.Hostname()
}

func slicesContains(values []string, needle string) bool {
	for _, value := range values {
		if strings.EqualFold(value, needle) {
			return true
		}
	}
	return false
}
