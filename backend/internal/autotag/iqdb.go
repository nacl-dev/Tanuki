package autotag

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nacl-dev/tanuki/internal/ratelimit"
)

// IQDBResult holds data extracted from an IQDB search match.
type IQDBResult struct {
	Similarity float64
	SourceURL  string
	Tags       []string
}

// iqdbClient queries iqdb.org via image upload.
type iqdbClient struct {
	limiter *ratelimit.Limiter
	http    *http.Client
}

func newIQDBClient(rateInterval time.Duration) *iqdbClient {
	return &iqdbClient{
		limiter: ratelimit.New(rateInterval),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// SearchFile uploads the given image file to IQDB.
func (c *iqdbClient) SearchFile(path string, threshold float64) (*IQDBResult, error) {
	c.limiter.Wait()

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("iqdb open file: %w", err)
	}
	defer file.Close()

	part, err := writer.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return nil, fmt.Errorf("iqdb create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("iqdb copy file: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("iqdb close writer: %w", err)
	}

	resp, err := c.http.Post("https://iqdb.org/", writer.FormDataContentType(), payload) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("iqdb request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("iqdb read body: %w", err)
	}

	return parseIQDBResponse(string(body), threshold)
}

func parseIQDBResponse(html string, threshold float64) (*IQDBResult, error) {
	simRe := regexp.MustCompile(`(\d+)%\s*similarity`)
	urlRe := regexp.MustCompile(`href="(https?://[^"]+)"`)

	rest := html
	if idx := strings.Index(rest, "Best match"); idx >= 0 {
		rest = rest[idx:]
	} else if idx := strings.Index(rest, "best match"); idx >= 0 {
		rest = rest[idx:]
	} else {
		return nil, nil
	}

	simMatches := simRe.FindStringSubmatch(rest)
	if len(simMatches) < 2 {
		return nil, nil
	}
	sim, err := strconv.ParseFloat(simMatches[1], 64)
	if err != nil || sim < threshold {
		return nil, nil
	}

	urlMatches := urlRe.FindStringSubmatch(rest)
	sourceURL := ""
	if len(urlMatches) >= 2 {
		sourceURL = urlMatches[1]
	}

	return &IQDBResult{
		Similarity: sim,
		SourceURL:  sourceURL,
	}, nil
}
