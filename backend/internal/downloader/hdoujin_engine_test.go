package downloader

import (
	"archive/zip"
	"fmt"
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
