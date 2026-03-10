// Package autotag provides reverse-image-search based auto-tagging via SauceNAO and IQDB.
package autotag

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/nacl-dev/tanuki/internal/ratelimit"
)

// SauceNAOResult holds the relevant fields extracted from a SauceNAO search hit.
type SauceNAOResult struct {
	Similarity float64
	Title      string
	Artist     string
	Characters []string
	Parody     string // series / parody
	ExternalURLs []string
	Source     string // raw source string
}

// sauceNAOClient calls the SauceNAO JSON API.
type sauceNAOClient struct {
	apiKey  string
	limiter *ratelimit.Limiter
	http    *http.Client
}

// newSauceNAOClient creates a client with the given API key and rate-limit interval.
func newSauceNAOClient(apiKey string, rateInterval time.Duration) *sauceNAOClient {
	return &sauceNAOClient{
		apiKey:  apiKey,
		limiter: ratelimit.New(rateInterval),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Search queries SauceNAO for the given image URL.
// Returns the best result above the similarity threshold, or nil if none found.
func (c *sauceNAOClient) Search(imageURL string, threshold float64) (*SauceNAOResult, error) {
	c.limiter.Wait()

	params := url.Values{}
	params.Set("output_type", "2") // JSON output
	params.Set("api_key", c.apiKey)
	params.Set("url", imageURL)
	params.Set("numres", "3")

	apiURL := "https://saucenao.com/search.php?" + params.Encode()

	resp, err := c.http.Get(apiURL) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("saucenao request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("saucenao read body: %w", err)
	}

	return parseSauceNAOResponse(body, threshold)
}

// ─── SauceNAO response types ──────────────────────────────────────────────────

type sauceNAOResponse struct {
	Header  sauceNAOHeader   `json:"header"`
	Results []sauceNAOResult `json:"results"`
}

type sauceNAOHeader struct {
	Status int `json:"status"`
}

type sauceNAOResult struct {
	Header sauceNAOResultHeader `json:"header"`
	Data   sauceNAOResultData   `json:"data"`
}

type sauceNAOResultHeader struct {
	Similarity string   `json:"similarity"`
	Thumbnail  string   `json:"thumbnail"`
	IndexName  string   `json:"index_name"`
	ExtURLs    []string `json:"ext_urls"`
}

type sauceNAOResultData struct {
	Title     string   `json:"title"`
	Author    string   `json:"author"`
	Creator   string   `json:"creator"`
	Artist    string   `json:"artist"`
	Member    string   `json:"member_name"`
	Character []string `json:"characters"`
	Material  string   `json:"material"`
	Source    string   `json:"source"`
	ExtURLs   []string `json:"ext_urls"`
}

func parseSauceNAOResponse(body []byte, threshold float64) (*SauceNAOResult, error) {
	var resp sauceNAOResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("saucenao parse: %w", err)
	}
	if resp.Header.Status != 0 {
		return nil, fmt.Errorf("saucenao api error status %d", resp.Header.Status)
	}

	for _, r := range resp.Results {
		var sim float64
		fmt.Sscanf(r.Header.Similarity, "%f", &sim) //nolint:errcheck
		if sim < threshold {
			continue
		}

		d := r.Data
		artist := firstNonEmpty(d.Artist, d.Author, d.Creator, d.Member)
		title := firstNonEmpty(d.Title, d.Source)

		extURLs := append(r.Header.ExtURLs, d.ExtURLs...) //nolint:gocritic

		return &SauceNAOResult{
			Similarity:   sim,
			Title:        title,
			Artist:       artist,
			Characters:   d.Character,
			Parody:       d.Material,
			ExternalURLs: extURLs,
			Source:       d.Source,
		}, nil
	}

	return nil, nil // no match above threshold
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
