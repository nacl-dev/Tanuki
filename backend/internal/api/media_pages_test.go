package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nacl-dev/tanuki/internal/models"
)

func TestListPageNamesForMediaPathSupportsDirectoryGalleries(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "gallery")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir gallery: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "002.png"), []byte("two"), 0o644); err != nil {
		t.Fatalf("write second image: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "001.jpg"), []byte("one"), 0o644); err != nil {
		t.Fatalf("write first image: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("ignored"), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}

	names, err := listPageNamesForMediaPath(dir, root)
	if err != nil {
		t.Fatalf("listPageNamesForMediaPath: %v", err)
	}

	if len(names) != 2 {
		t.Fatalf("expected 2 images, got %d", len(names))
	}
	if names[0] != "001.jpg" || names[1] != "002.png" {
		t.Fatalf("unexpected page order: %#v", names)
	}
}

func TestServePageForMediaPathSupportsDirectoryGalleries(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	root := t.TempDir()
	dir := filepath.Join(root, "gallery")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir gallery: %v", err)
	}
	wantBody := []byte("page-two")
	if err := os.WriteFile(filepath.Join(dir, "001.png"), []byte("page-one"), 0o644); err != nil {
		t.Fatalf("write first page: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "002.png"), wantBody, 0o644); err != nil {
		t.Fatalf("write second page: %v", err)
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/media/test/pages/1", nil)

	if err := servePageForMediaPath(context, dir, root, 1); err != nil {
		t.Fatalf("servePageForMediaPath: %v", err)
	}

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("expected image/png content type, got %q", got)
	}
	if got := recorder.Body.String(); got != string(wantBody) {
		t.Fatalf("unexpected page body %q", got)
	}
}

func TestClassifyMobileContentKindSupportsDirectoryDoujinshi(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "gallery")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir gallery: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "001.png"), []byte("page"), 0o644); err != nil {
		t.Fatalf("write page: %v", err)
	}

	item := models.Media{
		Type:     models.MediaTypeDoujinshi,
		FilePath: dir,
	}

	if got := classifyMobileContentKind(item, root); got != "pages" {
		t.Fatalf("expected pages content kind, got %q", got)
	}
}
