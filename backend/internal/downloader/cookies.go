package downloader

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	urlpkg "net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func newHTTPClientWithCookies(cookiesPath string) (*http.Client, error) {
	jar, err := loadCookieJar(cookiesPath)
	if err != nil {
		return &http.Client{}, err
	}
	return &http.Client{Jar: jar}, nil
}

func loadCookieJar(cookiesPath string) (http.CookieJar, error) {
	if strings.TrimSpace(cookiesPath) == "" {
		return nil, nil
	}

	file, err := os.Open(cookiesPath)
	if err != nil {
		return nil, fmt.Errorf("open cookies file: %w", err)
	}
	defer file.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "#HttpOnly_") {
			continue
		}

		httpOnly := false
		if strings.HasPrefix(line, "#HttpOnly_") {
			httpOnly = true
			line = strings.TrimPrefix(line, "#HttpOnly_")
		}

		parts := strings.Split(line, "\t")
		if len(parts) != 7 {
			continue
		}

		host := strings.TrimSpace(strings.TrimPrefix(parts[0], "."))
		path := strings.TrimSpace(parts[2])
		secure := strings.EqualFold(strings.TrimSpace(parts[3]), "TRUE")
		name := strings.TrimSpace(parts[5])
		value := parts[6]
		if host == "" || name == "" {
			continue
		}
		if path == "" {
			path = "/"
		}

		scheme := "http"
		if secure {
			scheme = "https"
		}
		u, err := urlpkg.Parse(fmt.Sprintf("%s://%s%s", scheme, host, path))
		if err != nil {
			return nil, fmt.Errorf("parse cookies file line %d: %w", lineNumber, err)
		}

		cookie := &http.Cookie{
			Name:     name,
			Value:    value,
			Path:     path,
			Domain:   host,
			Secure:   secure,
			HttpOnly: httpOnly,
		}
		if expiresUnix, err := strconv.ParseInt(strings.TrimSpace(parts[4]), 10, 64); err == nil && expiresUnix > 0 {
			cookie.Expires = time.Unix(expiresUnix, 0).UTC()
		}

		jar.SetCookies(u, []*http.Cookie{cookie})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read cookies file: %w", err)
	}

	return jar, nil
}
