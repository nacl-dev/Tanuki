package downloader

import (
	"encoding/json"
	"testing"
)

func TestNormalizeDownloadAutoTags(t *testing.T) {
	t.Parallel()

	got := NormalizeDownloadAutoTags([]string{" artist:Foo ", "artist:foo", "", "series:Bar"})
	want := []string{"artist:Foo", "series:Bar"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestEncodeAndDecodeDownloadAutoTags(t *testing.T) {
	t.Parallel()

	raw, err := EncodeDownloadAutoTags([]string{"artist:Foo", "artist:foo", "series:Bar"})
	if err != nil {
		t.Fatalf("EncodeDownloadAutoTags returned error: %v", err)
	}
	if raw == nil {
		t.Fatal("expected encoded payload")
	}

	var stored []string
	if err := json.Unmarshal(*raw, &stored); err != nil {
		t.Fatalf("unmarshal encoded payload: %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("stored len = %d, want 2", len(stored))
	}

	got := DecodeDownloadAutoTags(raw)
	want := []string{"artist:Foo", "series:Bar"}
	if len(got) != len(want) {
		t.Fatalf("decoded len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
