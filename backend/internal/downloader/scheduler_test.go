package downloader

import (
	"encoding/json"
	"testing"

	"github.com/nacl-dev/tanuki/internal/models"
)

func TestScheduleFingerprintIncludesDefaultTags(t *testing.T) {
	t.Parallel()

	firstRaw := json.RawMessage(`["artist:Alpha"]`)
	secondRaw := json.RawMessage(`["artist:Beta"]`)

	first := models.DownloadSchedule{
		CronExpression:  "0 3 * * *",
		URLPattern:      "https://example.com/source",
		SourceType:      "auto",
		TargetDirectory: "/downloads",
		Enabled:         true,
		DefaultTags:     &firstRaw,
	}
	second := first
	second.DefaultTags = &secondRaw

	if scheduleFingerprint(first) == scheduleFingerprint(second) {
		t.Fatal("expected default_tags to affect schedule fingerprint")
	}
}
