package autotag

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/nacl-dev/tanuki/internal/ratelimit"
)

// SauceNAOResult holds the relevant fields extracted from a SauceNAO search hit.
type SauceNAOResult struct {
	Similarity   float64
	Title        string
	Artist       string
	Characters   []string
	Parody       string
	ExternalURLs []string
	Source       string
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

// SearchFile uploads a local image file to SauceNAO.
func (c *sauceNAOClient) SearchFile(path string, threshold float64) (*SauceNAOResult, error) {
	c.limiter.Wait()

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	_ = writer.WriteField("output_type", "2")
	_ = writer.WriteField("api_key", c.apiKey)
	_ = writer.WriteField("numres", "3")

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("saucenao open file: %w", err)
	}
	defer file.Close()

	part, err := writer.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return nil, fmt.Errorf("saucenao create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("saucenao copy file: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("saucenao close writer: %w", err)
	}

	resp, err := c.http.Post("https://saucenao.com/search.php", writer.FormDataContentType(), payload) //nolint:noctx
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
		extURLs := append(r.Header.ExtURLs, d.ExtURLs...)

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

	return nil, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
