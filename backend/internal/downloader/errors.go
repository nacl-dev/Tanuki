package downloader

import "fmt"

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
