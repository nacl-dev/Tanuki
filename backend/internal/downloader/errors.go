package downloader

import (
	"fmt"
	"strings"
)

type unsupportedURLError struct {
	engine string
	detail string
}

func (e *unsupportedURLError) Error() string {
	if e.detail == "" {
		return fmt.Sprintf("%s: unsupported URL", e.engine)
	}
	return fmt.Sprintf("%s: unsupported URL: %s", e.engine, e.detail)
}

func newUnsupportedURLError(engine, detail string) error {
	return &unsupportedURLError{engine: engine, detail: detail}
}

func isUnsupportedURLError(err error) bool {
	_, ok := err.(*unsupportedURLError)
	return ok
}

func blockedSourceDetail(status int) string {
	return fmt.Sprintf("remote source blocked the request with status %d; export browser cookies and set DOWNLOADER_COOKIES_FILE if this source requires browser validation", status)
}

func blockedChallengeDetail(summary string) string {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		summary = "remote source blocked the request"
	}
	return fmt.Sprintf("%s; export browser cookies and set DOWNLOADER_COOKIES_FILE if this source requires browser validation", summary)
}
