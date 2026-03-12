package downloader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type hdoujinModuleCandidate struct {
	Module         string `json:"module"`
	Site           string `json:"site"`
	URL            string `json:"url"`
	ExpectedEngine string `json:"expected_engine"`
	Coverage       string `json:"coverage"`
}

func TestHDoujinModuleCandidatesResolveToNativeEngines(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(filepath.Join("testdata", "hdoujin_module_candidates.json"))
	if err != nil {
		t.Fatalf("read fixture manifest: %v", err)
	}

	var candidates []hdoujinModuleCandidate
	if err := json.Unmarshal(body, &candidates); err != nil {
		t.Fatalf("decode fixture manifest: %v", err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected at least one HDoujin fixture candidate")
	}

	engines := []Engine{
		NewRule34ArtEngine("", nil),
		NewPornComicsEngine("", nil),
		NewYtDlpEngine("", "", "", nil),
		NewHentai0Engine(nil),
		NewImageGalleryEngine(nil),
		NewDanbooruEngine(nil),
		NewBooruEngine(nil),
		NewGalleryDLEngine("", "", nil),
	}

	for _, candidate := range candidates {
		candidate := candidate
		t.Run(candidate.Module, func(t *testing.T) {
			engine := ParseEngine(engines, candidate.URL)
			if engine == nil {
				t.Fatalf("no engine matched %s", candidate.URL)
			}
			got := fmt.Sprintf("%T", engine)
			if got != candidate.ExpectedEngine {
				t.Fatalf("engine mismatch for %s: got %q want %q", candidate.URL, got, candidate.ExpectedEngine)
			}
		})
	}
}
