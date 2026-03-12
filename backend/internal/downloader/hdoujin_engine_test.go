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

// --- Bug fix tests ---

func TestHDoujinCollectionTagsIgnoreFunctionValues(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body><h1>Tags Test</h1><ul><li>Romance</li><li>Comedy</li></ul></body></html>`)
	}))
	defer server.Close()

	host := mustTestHostname(t, server.URL)
	modulesDir := t.TempDir()
	// The module assigns info.Tags to a collection returned by SelectValues,
	// which carries helper methods (Count, First, Last). These must not
	// appear as tag strings in the imported metadata.
	if err := os.WriteFile(filepath.Join(modulesDir, "TagTest.lua"), []byte(fmt.Sprintf(`
function Register()
  module.Name = 'TagTest'
  module.Domains.Add('%s')
end

function GetInfo()
  info.Title = 'Tags Test'
  info.Tags = dom.SelectValues('//ul/li')
end

function GetPages()
  pages.Add('/img/1.jpg')
end
`, host)), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}

	engine := NewHDoujinLuaEngine(modulesDir, "", zap.NewNop())
	meta, err := engine.FetchMetadata(server.URL + "/")
	if err != nil {
		t.Fatalf("fetch metadata: %v", err)
	}

	for _, tag := range meta.Tags {
		if strings.Contains(tag, "function") {
			t.Fatalf("tag list contains function-valued entry: %q (all tags: %v)", tag, meta.Tags)
		}
	}
	if !slicesContains(meta.Tags, "genre:Romance") {
		t.Fatalf("expected genre:Romance in tags, got %v", meta.Tags)
	}
	if !slicesContains(meta.Tags, "genre:Comedy") {
		t.Fatalf("expected genre:Comedy in tags, got %v", meta.Tags)
	}
}

// --- Chapter helper tests ---

func TestHDoujinChaptersFirstLastClearHelpers(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>
			<h1>Series</h1>
			<a class="ch" href="/ch/1" title="Chapter 1">Ch 1</a>
			<a class="ch" href="/ch/2" title="Chapter 2">Ch 2</a>
			<a class="ch" href="/ch/3" title="Chapter 3">Ch 3</a>
		</body></html>`)
	}))
	defer server.Close()

	host := mustTestHostname(t, server.URL)
	modulesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(modulesDir, "ChHelpers.lua"), []byte(fmt.Sprintf(`
function Register()
  module.Name = 'ChHelpers'
  module.Domains.Add('%s')
end

function GetInfo()
  info.Title = 'Series'
end

function GetChapters()
  chapters.AddRange(dom.SelectElements('//a[@class="ch"]'))
  -- Verify First/Last return expected values
  local first = chapters.First()
  local last = chapters.Last()
  -- Store first/last titles so we can verify in GetInfo metadata
  info.Artist = first.Title
  info.Parody = last.Title
end

function GetPages()
  pages.Add('/img/1.jpg')
end
`, host)), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}

	engine := NewHDoujinLuaEngine(modulesDir, "", zap.NewNop())
	meta, err := engine.FetchMetadata(server.URL + "/")
	if err != nil {
		t.Fatalf("fetch metadata: %v", err)
	}

	// info.Artist was set to chapters.First().Title
	if !slicesContains(meta.Tags, "artist:Chapter 1") {
		t.Fatalf("expected first chapter title in artist tag, got %v", meta.Tags)
	}
	// info.Parody was set to chapters.Last().Title
	if !slicesContains(meta.Tags, "parody:Chapter 3") {
		t.Fatalf("expected last chapter title in parody tag, got %v", meta.Tags)
	}
	if meta.TotalFiles != 3 {
		t.Fatalf("expected 3 chapters, got %d", meta.TotalFiles)
	}
}

func TestHDoujinChaptersFilterDuplicates(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>
			<h1>Dupes</h1>
			<a class="ch" href="/ch/1">Ch 1</a>
			<a class="ch" href="/ch/2">Ch 2</a>
			<a class="ch" href="/ch/1">Ch 1 Dupe</a>
			<a class="ch" href="/ch/3">Ch 3</a>
			<a class="ch" href="/ch/2">Ch 2 Dupe</a>
		</body></html>`)
	}))
	defer server.Close()

	host := mustTestHostname(t, server.URL)
	modulesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(modulesDir, "DupTest.lua"), []byte(fmt.Sprintf(`
function Register()
  module.Name = 'DupTest'
  module.Domains.Add('%s')
end

function GetInfo()
  info.Title = 'Dupes'
end

function GetChapters()
  chapters.AddRange(dom.SelectElements('//a[@class="ch"]'))
  chapters.FilterDuplicates()
end

function GetPages()
  pages.Add('/img/1.jpg')
end
`, host)), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}

	engine := NewHDoujinLuaEngine(modulesDir, "", zap.NewNop())
	meta, err := engine.FetchMetadata(server.URL + "/")
	if err != nil {
		t.Fatalf("fetch metadata: %v", err)
	}

	if meta.TotalFiles != 3 {
		t.Fatalf("expected 3 unique chapters after FilterDuplicates, got %d", meta.TotalFiles)
	}
}

func TestHDoujinChaptersClear(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>
			<h1>Clear Test</h1>
			<a class="ch" href="/ch/1">Ch 1</a>
			<a class="ch" href="/ch/2">Ch 2</a>
			<a class="real" href="/ch/3">Real Ch</a>
		</body></html>`)
	}))
	defer server.Close()

	host := mustTestHostname(t, server.URL)
	modulesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(modulesDir, "ClearTest.lua"), []byte(fmt.Sprintf(`
function Register()
  module.Name = 'ClearTest'
  module.Domains.Add('%s')
end

function GetInfo()
  info.Title = 'Clear Test'
end

function GetChapters()
  -- Add wrong chapters first, then clear and add the right ones
  chapters.AddRange(dom.SelectElements('//a[@class="ch"]'))
  chapters.Clear()
  chapters.AddRange(dom.SelectElements('//a[@class="real"]'))
end

function GetPages()
  pages.Add('/img/1.jpg')
end
`, host)), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}

	engine := NewHDoujinLuaEngine(modulesDir, "", zap.NewNop())
	meta, err := engine.FetchMetadata(server.URL + "/")
	if err != nil {
		t.Fatalf("fetch metadata: %v", err)
	}

	if meta.TotalFiles != 1 {
		t.Fatalf("expected 1 chapter after Clear + re-add, got %d", meta.TotalFiles)
	}
}

// --- Chapter-series fixture tests ---

func TestHDoujinChapterListParsingFromMockHTML(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/series":
			fmt.Fprint(w, `<html><body>
				<h1>Test Manga</h1>
				<div class="chapters">
					<a href="/series/ch-3" title="Chapter 3">Chapter 3</a>
					<a href="/series/ch-2" title="Chapter 2">Chapter 2</a>
					<a href="/series/ch-1" title="Chapter 1">Chapter 1</a>
				</div>
			</body></html>`)
		case "/series/ch-1", "/series/ch-2", "/series/ch-3":
			fmt.Fprintf(w, `<html><body><img src="/img%s.jpg"></body></html>`, r.URL.Path)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	host := mustTestHostname(t, server.URL)
	modulesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(modulesDir, "ChParse.lua"), []byte(fmt.Sprintf(`
function Register()
  module.Name = 'ChParse'
  module.Domains.Add('%s')
end

function GetInfo()
  info.Title = dom.SelectValue('//h1')
end

function GetChapters()
  chapters.AddRange(dom.SelectElements('//div[@class="chapters"]/a'))
  chapters.Reverse()
end

function GetPages()
  pages.AddRange(dom.SelectValues('//img/@src'))
end
`, host)), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}

	engine := NewHDoujinLuaEngine(modulesDir, "", zap.NewNop())
	meta, err := engine.FetchMetadata(server.URL + "/series")
	if err != nil {
		t.Fatalf("fetch metadata: %v", err)
	}
	if meta.Title != "Test Manga" {
		t.Fatalf("unexpected title: %q", meta.Title)
	}
	if meta.TotalFiles != 3 {
		t.Fatalf("expected 3 chapters, got %d", meta.TotalFiles)
	}
}

func TestHDoujinChapterMetadataExtraction(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manga/test":
			fmt.Fprint(w, `<html><body>
				<h1>Metadata Manga</h1>
				<span class="artist">Test Artist</span>
				<div class="chapters">
					<a href="/manga/test/ch-1" title="Chapter 1 - Intro">Chapter 1 - Intro</a>
				</div>
			</body></html>`)
		case "/manga/test/ch-1":
			fmt.Fprint(w, `<html><body>
				<h1>Metadata Manga - Chapter 1 - Intro</h1>
				<img src="/img/1.jpg">
			</body></html>`)
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
	if err := os.WriteFile(filepath.Join(modulesDir, "MetaCh.lua"), []byte(fmt.Sprintf(`
function Register()
  module.Name = 'MetaCh'
  module.Domains.Add('%s')
end

function GetInfo()
  info.Title = dom.SelectValue('//h1')
  info.Artist = dom.SelectValue('//span[@class="artist"]')
end

function GetChapters()
  chapters.AddRange(dom.SelectElements('//div[@class="chapters"]/a'))
end

function GetPages()
  pages.AddRange(dom.SelectValues('//img/@src'))
end
`, host)), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}

	engine := NewHDoujinLuaEngine(modulesDir, "", zap.NewNop())

	dest := t.TempDir()
	job := &models.DownloadJob{
		ID:              "meta-ch-job",
		URL:             server.URL + "/manga/test",
		TargetDirectory: dest,
	}
	if err := engine.Download(t.Context(), job); err != nil {
		t.Fatalf("download: %v", err)
	}

	archivePath := filepath.Join(dest, "Metadata Manga - Chapter 1 - Intro.cbz")
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("expected archive: %v", err)
	}

	sidecarBody, err := os.ReadFile(archivePath + ".tanuki.json")
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	sidecar := string(sidecarBody)
	if !strings.Contains(sidecar, `"work_title": "Metadata Manga"`) {
		t.Fatalf("expected work_title in sidecar, got %s", sidecar)
	}
	if !strings.Contains(sidecar, "artist:Test Artist") {
		t.Fatalf("expected artist tag in sidecar, got %s", sidecar)
	}
}

func TestHDoujinEmptyChapterList(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body><h1>Empty Series</h1></body></html>`)
	}))
	defer server.Close()

	host := mustTestHostname(t, server.URL)
	modulesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(modulesDir, "Empty.lua"), []byte(fmt.Sprintf(`
function Register()
  module.Name = 'Empty'
  module.Domains.Add('%s')
end

function GetInfo()
  info.Title = dom.SelectValue('//h1')
end

function GetChapters()
  -- intentionally empty: no chapters found on this page
  chapters.AddRange(dom.SelectElements('//div[@class="missing"]/a'))
end
`, host)), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}

	engine := NewHDoujinLuaEngine(modulesDir, "", zap.NewNop())
	_, err := engine.FetchMetadata(server.URL + "/")
	if err == nil {
		t.Fatal("expected error for empty chapter list with no pages")
	}
	if !strings.Contains(err.Error(), "did not yield") {
		t.Fatalf("expected 'did not yield' error, got: %v", err)
	}
}

func TestHDoujinSingleChapterWork(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oneshot":
			fmt.Fprint(w, `<html><body>
				<h1>Oneshot Title</h1>
				<div class="chapters">
					<a href="/oneshot/ch-1" title="Oneshot">Oneshot</a>
				</div>
			</body></html>`)
		case "/oneshot/ch-1":
			fmt.Fprint(w, `<html><body><img src="/img/1.jpg"><img src="/img/2.jpg"></body></html>`)
		case "/img/1.jpg", "/img/2.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("jpeg"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	host := mustTestHostname(t, server.URL)
	modulesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(modulesDir, "Oneshot.lua"), []byte(fmt.Sprintf(`
function Register()
  module.Name = 'Oneshot'
  module.Domains.Add('%s')
end

function GetInfo()
  info.Title = dom.SelectValue('//h1')
end

function GetChapters()
  chapters.AddRange(dom.SelectElements('//div[@class="chapters"]/a'))
end

function GetPages()
  pages.AddRange(dom.SelectValues('//img/@src'))
end
`, host)), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}

	engine := NewHDoujinLuaEngine(modulesDir, "", zap.NewNop())

	dest := t.TempDir()
	job := &models.DownloadJob{
		ID:              "oneshot-job",
		URL:             server.URL + "/oneshot",
		TargetDirectory: dest,
	}
	if err := engine.Download(t.Context(), job); err != nil {
		t.Fatalf("download: %v", err)
	}

	archivePath := filepath.Join(dest, "Oneshot Title - Oneshot.cbz")
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("expected archive: %v", err)
	}

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer reader.Close()
	if len(reader.File) != 2 {
		t.Fatalf("expected 2 pages in single-chapter archive, got %d", len(reader.File))
	}
}

func TestHDoujinReversedChapterOrdering(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/reversed":
			fmt.Fprint(w, `<html><body>
				<h1>Reversed Series</h1>
				<div class="chapters">
					<a href="/reversed/ch-3" title="Chapter 3">Ch 3</a>
					<a href="/reversed/ch-2" title="Chapter 2">Ch 2</a>
					<a href="/reversed/ch-1" title="Chapter 1">Ch 1</a>
				</div>
			</body></html>`)
		case "/reversed/ch-1", "/reversed/ch-2", "/reversed/ch-3":
			fmt.Fprint(w, `<html><body><img src="/img/page.jpg"></body></html>`)
		case "/img/page.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("jpeg"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	host := mustTestHostname(t, server.URL)
	modulesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(modulesDir, "Reversed.lua"), []byte(fmt.Sprintf(`
function Register()
  module.Name = 'Reversed'
  module.Domains.Add('%s')
end

function GetInfo()
  info.Title = dom.SelectValue('//h1')
end

function GetChapters()
  chapters.AddRange(dom.SelectElements('//div[@class="chapters"]/a'))
  chapters.Reverse()
end

function GetPages()
  pages.AddRange(dom.SelectValues('//img/@src'))
end
`, host)), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}

	engine := NewHDoujinLuaEngine(modulesDir, "", zap.NewNop())

	dest := t.TempDir()
	job := &models.DownloadJob{
		ID:              "reversed-job",
		URL:             server.URL + "/reversed",
		TargetDirectory: dest,
	}
	if err := engine.Download(t.Context(), job); err != nil {
		t.Fatalf("download: %v", err)
	}

	// After Reverse(), ch-1 should be first -> first archive is "Reversed Series - Chapter 1"
	firstArchive := filepath.Join(dest, "Reversed Series - Chapter 1.cbz")
	if _, err := os.Stat(firstArchive); err != nil {
		t.Fatalf("expected first archive (Chapter 1) after Reverse: %v", err)
	}
	lastArchive := filepath.Join(dest, "Reversed Series - Chapter 3.cbz")
	if _, err := os.Stat(lastArchive); err != nil {
		t.Fatalf("expected last archive (Chapter 3) after Reverse: %v", err)
	}

	// Verify work index in sidecar for ordering
	sidecarBody, err := os.ReadFile(firstArchive + ".tanuki.json")
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	if !strings.Contains(string(sidecarBody), `"work_title": "Reversed Series"`) {
		t.Fatalf("expected work_title in sidecar, got %s", string(sidecarBody))
	}
}
