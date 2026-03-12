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

func TestHDoujinLuaEngineChapterSeriesOrganizeNoStagingLeak(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manga/staging-test":
			writeHDoujinJSONPage(t, w, map[string]any{
				"props": map[string]any{
					"manga": map[string]any{
						"name": "Staging Test",
						"slug": "staging-test",
					},
					"chapters": []map[string]any{
						{"slug": "ch-02", "name": "ch 02"},
						{"slug": "ch-01", "name": "ch 01"},
					},
				},
			})
		case "/manga/staging-test/ch-01":
			writeHDoujinJSONPage(t, w, map[string]any{
				"props": map[string]any{
					"mangaName":      "Staging Test",
					"chapterName":    "ch 01",
					"chapterContent": `<img src="/img/a.jpg">`,
				},
			})
		case "/manga/staging-test/ch-02":
			writeHDoujinJSONPage(t, w, map[string]any{
				"props": map[string]any{
					"mangaName":      "Staging Test",
					"chapterName":    "ch 02",
					"chapterContent": `<img src="/img/b.jpg">`,
				},
			})
		case "/img/a.jpg", "/img/b.jpg":
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
local function getPageDataJson()
  local pageDataStr = dom:SelectValue('//div[@id="app"]/@data-page')
  local js = JavaScript.New()
  return js:Execute("pageData = " .. pageDataStr):ToJson()
end

function Register()
  module.Name = "StagingTest"
  module.Domains:Add("%s")
end

function GetInfo()
  local json = getPageDataJson()
  info.Title = json:SelectValue("props.manga.name")
  if isempty(info.Title) then
    info.Title = json:SelectValue("props.mangaName") .. " - " .. json:SelectValue("props.chapterName")
  end
end

function GetChapters()
  local json = getPageDataJson()
  local slug = json:SelectValue("props.manga.slug")
  for node in json:SelectNodes("props.chapters[*]") do
    chapters:Add("/manga/" .. slug .. "/" .. node:SelectValue("slug"), node:SelectValue("name"))
  end
  chapters:Reverse()
end

function GetPages()
  local json = getPageDataJson()
  dom = Dom.New(json:SelectValue("props.chapterContent"))
  pages:AddRange(dom:SelectValues("//img/@src"))
end
`, host)
	if err := os.WriteFile(filepath.Join(modulesDir, "StagingTest.lua"), []byte(moduleBody), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}

	targetRoot := t.TempDir()
	stagingDir := filepath.Join(targetRoot, ".tanuki-job-staging-test")
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		t.Fatalf("mkdir staging: %v", err)
	}

	engine := NewHDoujinLuaEngine(modulesDir, "", zap.NewNop())
	job := &models.DownloadJob{
		ID:              "staging-test",
		URL:             server.URL + "/manga/staging-test",
		TargetDirectory: stagingDir,
	}
	if err := engine.Download(t.Context(), job); err != nil {
		t.Fatalf("download: %v", err)
	}

	movedPaths, err := organizeDownloadedFiles(stagingDir, targetRoot)
	if err != nil {
		t.Fatalf("organize: %v", err)
	}
	if len(movedPaths) != 2 {
		t.Fatalf("expected 2 organized files, got %d", len(movedPaths))
	}
	for _, path := range movedPaths {
		if strings.Contains(path, ".tanuki-job-") {
			t.Fatalf("staging path leaked into organized output: %s", path)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("organized file missing: %s: %v", path, err)
		}
		if _, err := os.Stat(path + ".tanuki.json"); err != nil {
			t.Fatalf("companion sidecar missing for %s: %v", path, err)
		}
	}
	_ = os.RemoveAll(stagingDir)
	if _, err := os.Stat(stagingDir); err == nil {
		t.Fatal("staging directory should have been cleaned up")
	}
}

func TestHDoujinLuaEngineCollectionTagsSkipFunctions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gallery":
			fmt.Fprint(w, `<html><body><h1>Tag Test</h1><ul class="tags"><li>Romance</li><li>Comedy</li></ul></body></html>`)
		case "/img/1.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("jpeg"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	host := mustTestHostname(t, server.URL)
	modulesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(modulesDir, "TagTest.lua"), []byte(fmt.Sprintf(`
function Register()
  module.Name = 'TagTest'
  module.Domains:Add('%s')
end

function GetInfo()
  info.Title = dom.SelectValue('//h1')
  info.Tags = dom.SelectElements('//ul[@class="tags"]/li')
  info.Artist = dom.SelectValues('//ul[@class="tags"]/li')
end

function GetPages()
  pages:Add('/img/1.jpg')
end
`, host)), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}

	engine := NewHDoujinLuaEngine(modulesDir, "", zap.NewNop())
	meta, err := engine.FetchMetadata(server.URL + "/gallery")
	if err != nil {
		t.Fatalf("fetch metadata: %v", err)
	}
	for _, tag := range meta.Tags {
		if strings.Contains(strings.ToLower(tag), "function") {
			t.Fatalf("tag leak: found function reference in tags: %q (all tags: %v)", tag, meta.Tags)
		}
	}
	if !slicesContains(meta.Tags, "genre:Romance") || !slicesContains(meta.Tags, "genre:Comedy") {
		t.Fatalf("expected genre tags, got: %v", meta.Tags)
	}
	if !slicesContains(meta.Tags, "artist:Romance") || !slicesContains(meta.Tags, "artist:Comedy") {
		t.Fatalf("expected artist tags from SelectValues collection, got: %v", meta.Tags)
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

// P3: Lightweight test fixture – large gallery host with 5 pages (simulates sites like hentai-img.com).
func TestHDoujinLuaEngineLargeGalleryFixture(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gallery/12345":
			fmt.Fprint(w, `<html><body>
				<h1>Large Gallery Title</h1>
				<div class="tags"><a>Action</a><a>Romance</a></div>
				<div class="artist">Artist X</div>
				<div class="pages">
					<img src="/img/page01.jpg">
					<img src="/img/page02.jpg">
					<img src="/img/page03.jpg">
					<img src="/img/page04.jpg">
					<img src="/img/page05.jpg">
				</div>
			</body></html>`)
		case "/img/page01.jpg", "/img/page02.jpg", "/img/page03.jpg",
			"/img/page04.jpg", "/img/page05.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("jpeg-data"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	host := mustTestHostname(t, server.URL)
	modulesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(modulesDir, "LargeGallery.lua"), []byte(fmt.Sprintf(`
function Register()
  module.Name = 'LargeGallery'
  module.Domains:Add('%s')
end

function GetInfo()
  info.Title = dom.SelectValue('//h1')
  info.Tags = dom.SelectValues('//div[@class="tags"]/a')
  info.Artist = dom.SelectValues('//div[@class="artist"]')
end

function GetPages()
  pages:AddRange(page.SelectValues('//div[@class="pages"]/img/@src'))
end
`, host)), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}

	engine := NewHDoujinLuaEngine(modulesDir, "", zap.NewNop())
	meta, err := engine.FetchMetadata(server.URL + "/gallery/12345")
	if err != nil {
		t.Fatalf("fetch metadata: %v", err)
	}
	if meta.Title != "Large Gallery Title" {
		t.Fatalf("unexpected title: %q", meta.Title)
	}
	if meta.TotalFiles != 5 {
		t.Fatalf("expected 5 pages, got %d", meta.TotalFiles)
	}

	dest := t.TempDir()
	job := &models.DownloadJob{
		ID:              "large-gallery",
		URL:             server.URL + "/gallery/12345",
		TargetDirectory: dest,
	}
	if err := engine.Download(t.Context(), job); err != nil {
		t.Fatalf("download: %v", err)
	}

	archivePath := filepath.Join(dest, "Large Gallery Title.cbz")
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("expected archive: %v", err)
	}
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer reader.Close()
	if len(reader.File) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(reader.File))
	}
}

// P3: Lightweight test fixture – chapter-series host with 3 short chapters.
func TestHDoujinLuaEngineChapterSeriesFixture(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manga/test-series":
			writeHDoujinJSONPage(t, w, map[string]any{
				"props": map[string]any{
					"manga": map[string]any{
						"name": "Test Series",
						"slug": "test-series",
						"genres": []map[string]any{
							{"name": "Fantasy"},
						},
					},
					"chapters": []map[string]any{
						{"slug": "ch-03", "name": "Chapter 03"},
						{"slug": "ch-02", "name": "Chapter 02"},
						{"slug": "ch-01", "name": "Chapter 01"},
					},
				},
			})
		case "/manga/test-series/ch-01":
			writeHDoujinJSONPage(t, w, map[string]any{
				"props": map[string]any{
					"mangaName":      "Test Series",
					"chapterName":    "Chapter 01",
					"chapterContent": `<img src="/img/ch1-1.jpg"><img src="/img/ch1-2.jpg">`,
				},
			})
		case "/manga/test-series/ch-02":
			writeHDoujinJSONPage(t, w, map[string]any{
				"props": map[string]any{
					"mangaName":      "Test Series",
					"chapterName":    "Chapter 02",
					"chapterContent": `<img src="/img/ch2-1.jpg">`,
				},
			})
		case "/manga/test-series/ch-03":
			writeHDoujinJSONPage(t, w, map[string]any{
				"props": map[string]any{
					"mangaName":      "Test Series",
					"chapterName":    "Chapter 03",
					"chapterContent": `<img src="/img/ch3-1.jpg"><img src="/img/ch3-2.jpg"><img src="/img/ch3-3.jpg">`,
				},
			})
		case "/img/ch1-1.jpg", "/img/ch1-2.jpg",
			"/img/ch2-1.jpg",
			"/img/ch3-1.jpg", "/img/ch3-2.jpg", "/img/ch3-3.jpg":
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
local function getPageDataJson()
  local pageDataStr = dom:SelectValue('//div[@id="app"]/@data-page')
  local js = JavaScript.New()
  return js:Execute("pageData = " .. pageDataStr):ToJson()
end

function Register()
  module.Name = "ChapterFixture"
  module.Domains:Add("%s")
end

function GetInfo()
  local json = getPageDataJson()
  info.Title = json:SelectValue("props.manga.name")
  info.Tags = json:SelectValues("props.manga.genres[*].name")
  if isempty(info.Title) then
    info.Title = json:SelectValue("props.mangaName") .. " - " .. json:SelectValue("props.chapterName")
  end
end

function GetChapters()
  local json = getPageDataJson()
  local slug = json:SelectValue("props.manga.slug")
  for node in json:SelectNodes("props.chapters[*]") do
    chapters:Add("/manga/" .. slug .. "/" .. node:SelectValue("slug"), node:SelectValue("name"))
  end
  chapters:Reverse()
end

function GetPages()
  local json = getPageDataJson()
  dom = Dom.New(json:SelectValue("props.chapterContent"))
  pages:AddRange(dom:SelectValues("//img/@src"))
end
`, host)
	if err := os.WriteFile(filepath.Join(modulesDir, "ChapterFixture.lua"), []byte(moduleBody), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}

	engine := NewHDoujinLuaEngine(modulesDir, "", zap.NewNop())
	meta, err := engine.FetchMetadata(server.URL + "/manga/test-series")
	if err != nil {
		t.Fatalf("fetch metadata: %v", err)
	}
	if meta.Title != "Test Series" {
		t.Fatalf("unexpected title: %q", meta.Title)
	}
	if meta.TotalFiles != 3 {
		t.Fatalf("expected 3 chapters, got %d", meta.TotalFiles)
	}

	dest := t.TempDir()
	job := &models.DownloadJob{
		ID:              "chapter-fixture",
		URL:             server.URL + "/manga/test-series",
		TargetDirectory: dest,
	}
	if err := engine.Download(t.Context(), job); err != nil {
		t.Fatalf("download: %v", err)
	}

	for _, chapterName := range []string{"Test Series - Chapter 01", "Test Series - Chapter 02", "Test Series - Chapter 03"} {
		archivePath := filepath.Join(dest, chapterName+".cbz")
		if _, err := os.Stat(archivePath); err != nil {
			t.Fatalf("missing archive %s: %v", chapterName, err)
		}
		if _, err := os.Stat(archivePath + ".tanuki.json"); err != nil {
			t.Fatalf("missing sidecar for %s: %v", chapterName, err)
		}
	}

	// Verify chapter 3 has 3 pages
	reader, err := zip.OpenReader(filepath.Join(dest, "Test Series - Chapter 03.cbz"))
	if err != nil {
		t.Fatalf("open ch03 archive: %v", err)
	}
	defer reader.Close()
	if len(reader.File) != 3 {
		t.Fatalf("expected 3 entries in ch03, got %d", len(reader.File))
	}
}

// P4: Test runtime helpers – Fail, SetParameter, DecodeBase64, StripParameters, Paginator.

func newHelperTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body><h1>Helper Test</h1><img src="/img/1.jpg"></body></html>`)
	}))
}

func TestHDoujinLuaEngineSetParameter(t *testing.T) {
	t.Parallel()
	server := newHelperTestServer()
	defer server.Close()
	host := mustTestHostname(t, server.URL)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SetParam.lua"), []byte(fmt.Sprintf(`
function Register()
  module.Name = 'SetParam'
  module.Domains:Add('%s')
end

function GetInfo()
  local result = SetParameter('https://example.com/path?a=1', 'page', '5')
  info.Title = result
end

function GetPages()
  pages:Add('/img/1.jpg')
end
`, host)), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}
	engine := NewHDoujinLuaEngine(dir, "", zap.NewNop())
	meta, err := engine.FetchMetadata(server.URL + "/test")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !strings.Contains(meta.Title, "page=5") {
		t.Fatalf("SetParameter did not set page param: %q", meta.Title)
	}
}

func TestHDoujinLuaEngineDecodeBase64(t *testing.T) {
	t.Parallel()
	server := newHelperTestServer()
	defer server.Close()
	host := mustTestHostname(t, server.URL)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "B64.lua"), []byte(fmt.Sprintf(`
function Register()
  module.Name = 'B64'
  module.Domains:Add('%s')
end

function GetInfo()
  info.Title = DecodeBase64('SGVsbG8gV29ybGQ=')
end

function GetPages()
  pages:Add('/img/1.jpg')
end
`, host)), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}
	engine := NewHDoujinLuaEngine(dir, "", zap.NewNop())
	meta, err := engine.FetchMetadata(server.URL + "/test")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if meta.Title != "Hello World" {
		t.Fatalf("DecodeBase64 failed: %q", meta.Title)
	}
}

func TestHDoujinLuaEngineStripParameters(t *testing.T) {
	t.Parallel()
	server := newHelperTestServer()
	defer server.Close()
	host := mustTestHostname(t, server.URL)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Strip.lua"), []byte(fmt.Sprintf(`
function Register()
  module.Name = 'Strip'
  module.Domains:Add('%s')
end

function GetInfo()
  info.Title = StripParameters('https://example.com/path?a=1&b=2#frag')
end

function GetPages()
  pages:Add('/img/1.jpg')
end
`, host)), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}
	engine := NewHDoujinLuaEngine(dir, "", zap.NewNop())
	meta, err := engine.FetchMetadata(server.URL + "/test")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if meta.Title != "https://example.com/path" {
		t.Fatalf("StripParameters failed: %q", meta.Title)
	}
}

func TestHDoujinLuaEngineFail(t *testing.T) {
	t.Parallel()
	server := newHelperTestServer()
	defer server.Close()
	host := mustTestHostname(t, server.URL)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "FailTest.lua"), []byte(fmt.Sprintf(`
function Register()
  module.Name = 'FailTest'
  module.Domains:Add('%s')
end

function GetInfo()
  Fail('intentional error')
end

function GetPages()
  pages:Add('/img/1.jpg')
end
`, host)), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}
	engine := NewHDoujinLuaEngine(dir, "", zap.NewNop())
	_, err := engine.FetchMetadata(server.URL + "/test")
	if err == nil {
		t.Fatal("expected Fail() to produce an error")
	}
	if !strings.Contains(err.Error(), "intentional error") {
		t.Fatalf("expected error to contain message, got: %v", err)
	}
}

func TestHDoujinLuaEnginePaginator(t *testing.T) {
	t.Parallel()
	server := newHelperTestServer()
	defer server.Close()
	host := mustTestHostname(t, server.URL)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Pager.lua"), []byte(fmt.Sprintf(`
function Register()
  module.Name = 'Pager'
  module.Domains:Add('%s')
end

function GetInfo()
  local pager = Paginator.New('https://example.com/list', 1, 1)
  local firstUrl = pager:GetUrl()
  pager:Next()
  local secondUrl = pager:GetUrl()
  info.Title = firstUrl .. '|' .. secondUrl
end

function GetPages()
  pages:Add('/img/1.jpg')
end
`, host)), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}
	engine := NewHDoujinLuaEngine(dir, "", zap.NewNop())
	meta, err := engine.FetchMetadata(server.URL + "/test")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	parts := strings.Split(meta.Title, "|")
	if len(parts) != 2 {
		t.Fatalf("expected 2 URLs, got: %q", meta.Title)
	}
	if !strings.Contains(parts[0], "page=1") {
		t.Fatalf("first URL should have page=1: %q", parts[0])
	}
	if !strings.Contains(parts[1], "page=2") {
		t.Fatalf("second URL should have page=2: %q", parts[1])
	}
}
