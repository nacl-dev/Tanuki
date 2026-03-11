package downloader

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCookieJarFromNetscapeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cookies.txt")
	content := "# Netscape HTTP Cookie File\n" +
		".rule34.art\tTRUE\t/\tTRUE\t2147483647\tcf_clearance\tsecret-token\n" +
		"#HttpOnly_.rule34.art\tTRUE\t/\tTRUE\t2147483647\tsessionid\tabc123\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write cookies file: %v", err)
	}

	jar, err := loadCookieJar(path)
	if err != nil {
		t.Fatalf("loadCookieJar returned error: %v", err)
	}

	reqURL := "https://rule34.art/"
	cookies := jar.Cookies(mustParseURL(t, reqURL))
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies for %s, got %d", reqURL, len(cookies))
	}
}

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse url %q: %v", rawURL, err)
	}
	return parsed
}
