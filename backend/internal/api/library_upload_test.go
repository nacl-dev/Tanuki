package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nacl-dev/tanuki/internal/models"
)

func TestSanitizeInboxRelativePath(t *testing.T) {
	t.Parallel()

	relativePath, err := sanitizeInboxRelativePath(`folder\sub\clip.mp4`, "clip.mp4")
	if err != nil {
		t.Fatalf("sanitizeInboxRelativePath returned error: %v", err)
	}
	expected := filepath.Join("folder", "sub", "clip.mp4")
	if relativePath != expected {
		t.Fatalf("expected %q, got %q", expected, relativePath)
	}

	if _, err := sanitizeInboxRelativePath("../escape.txt", "escape.txt"); err == nil {
		t.Fatal("expected path traversal to be rejected")
	}
}

func TestUploadInboxStoresFilesInBatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	inboxDir := t.TempDir()
	handler := &LibraryHandler{inboxPath: inboxDir}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.WriteField("batch_name", "remote drop"); err != nil {
		t.Fatalf("write batch_name: %v", err)
	}
	if err := writer.WriteField("paths", "set-a/clip.mp4"); err != nil {
		t.Fatalf("write path 1: %v", err)
	}
	fileOne, err := writer.CreateFormFile("files", "clip.mp4")
	if err != nil {
		t.Fatalf("create form file 1: %v", err)
	}
	if _, err := fileOne.Write([]byte("video-data")); err != nil {
		t.Fatalf("write form file 1: %v", err)
	}

	if err := writer.WriteField("paths", "set-a/poster.jpg"); err != nil {
		t.Fatalf("write path 2: %v", err)
	}
	fileTwo, err := writer.CreateFormFile("files", "poster.jpg")
	if err != nil {
		t.Fatalf("create form file 2: %v", err)
	}
	if _, err := fileTwo.Write([]byte("image-data")); err != nil {
		t.Fatalf("write form file 2: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/library/inbox/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = req

	handler.UploadInbox(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data inboxUploadResult `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.BatchName != "remote drop" {
		t.Fatalf("expected batch name %q, got %q", "remote drop", response.Data.BatchName)
	}
	if response.Data.SourcePath != "inbox/remote drop" {
		t.Fatalf("expected source path %q, got %q", "inbox/remote drop", response.Data.SourcePath)
	}
	if response.Data.FileCount != 2 {
		t.Fatalf("expected file count 2, got %d", response.Data.FileCount)
	}

	firstSaved := filepath.Join(inboxDir, "remote drop", "set-a", "clip.mp4")
	secondSaved := filepath.Join(inboxDir, "remote drop", "set-a", "poster.jpg")
	if _, err := os.Stat(firstSaved); err != nil {
		t.Fatalf("expected first file to exist: %v", err)
	}
	if _, err := os.Stat(secondSaved); err != nil {
		t.Fatalf("expected second file to exist: %v", err)
	}
}

func TestUploadInboxRejectsTraversalPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	inboxDir := t.TempDir()
	handler := &LibraryHandler{inboxPath: inboxDir}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("paths", "../escape.txt"); err != nil {
		t.Fatalf("write path: %v", err)
	}
	file, err := writer.CreateFormFile("files", "escape.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := file.Write([]byte("blocked")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/library/inbox/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = req

	handler.UploadInbox(context)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
	}

	entries, err := os.ReadDir(inboxDir)
	if err != nil {
		t.Fatalf("read inbox dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no files to be created, found %d entries", len(entries))
	}
}

func TestUploadInboxWritesDefaultTagSidecars(t *testing.T) {
	gin.SetMode(gin.TestMode)

	inboxDir := t.TempDir()
	handler := &LibraryHandler{inboxPath: inboxDir}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("batch_name", "tagged drop"); err != nil {
		t.Fatalf("write batch_name: %v", err)
	}
	if err := writer.WriteField("default_tags", "artist:Inbox"); err != nil {
		t.Fatalf("write default_tags: %v", err)
	}
	if err := writer.WriteField("default_tags", "series:Batch"); err != nil {
		t.Fatalf("write default_tags: %v", err)
	}
	if err := writer.WriteField("paths", "set-a/clip.mp4"); err != nil {
		t.Fatalf("write path: %v", err)
	}

	file, err := writer.CreateFormFile("files", "clip.mp4")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := file.Write([]byte("video-data")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/library/inbox/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = req

	handler.UploadInbox(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data inboxUploadResult `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data.DefaultTags) != 2 {
		t.Fatalf("expected default tags in response, got %#v", response.Data.DefaultTags)
	}

	sidecarPath := filepath.Join(inboxDir, "tagged drop", "set-a", "clip.mp4.tanuki.json")
	bodyBytes, err := os.ReadFile(sidecarPath)
	if err != nil {
		t.Fatalf("expected sidecar to exist: %v", err)
	}

	var metadata models.ImportMetadata
	if err := json.Unmarshal(bodyBytes, &metadata); err != nil {
		t.Fatalf("decode sidecar: %v", err)
	}
	if len(metadata.Tags) != 2 {
		t.Fatalf("expected 2 tags in sidecar, got %#v", metadata.Tags)
	}
}
