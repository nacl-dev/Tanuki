package autotag

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// iqdbClient queries iqdb.org via its URL-search form.
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

// Search queries IQDB for the given image URL.
// Returns the best result above the threshold, or nil if none found.
func (c *iqdbClient) Search(imageURL string, threshold float64) (*IQDBResult, error) {
	c.limiter.Wait()

	params := url.Values{}
	params.Set("url", imageURL)

	resp, err := c.http.PostForm("https://iqdb.org/", params) //nolint:noctx
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

// parseIQDBResponse extracts the best match from IQDB's HTML response using
// lightweight regex-based parsing (avoids adding an HTML parser dependency).
func parseIQDBResponse(html string, threshold float64) (*IQDBResult, error) {
	// IQDB renders matches inside <div class="pages"> sections.
	// Each best-match section contains a similarity percentage like "95% similarity".
	simRe := regexp.MustCompile(`(\d+)%\s*similarity`)
	urlRe  := regexp.MustCompile(`href="(https?://[^"]+)"`)

	// Skip the "your image" section (first table)
	rest := html
	if idx := strings.Index(rest, "Best match"); idx >= 0 {
		rest = rest[idx:]
	} else if idx := strings.Index(rest, "best match"); idx >= 0 {
		rest = rest[idx:]
	} else {
		return nil, nil // no matches
	}

	// Extract similarity
	simMatches := simRe.FindStringSubmatch(rest)
	if len(simMatches) < 2 {
		return nil, nil
	}
	sim, err := strconv.ParseFloat(simMatches[1], 64)
	if err != nil || sim < threshold {
		return nil, nil
	}

	// Extract first external URL in the match block
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
