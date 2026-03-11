package downloader

import "testing"

func TestExtractPornComicsComicID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		rawURL string
		want   string
	}{
		{rawURL: "https://porncomics.eu/comics/sultry-summer-book-3-14815", want: "14815"},
		{rawURL: "https://www.porncomics.eu/comics/example-title-42", want: "42"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.rawURL, func(t *testing.T) {
			t.Parallel()
			got, err := extractPornComicsComicID(test.rawURL)
			if err != nil {
				t.Fatalf("extractPornComicsComicID returned error: %v", err)
			}
			if got != test.want {
				t.Fatalf("extractPornComicsComicID = %q, want %q", got, test.want)
			}
		})
	}
}

func TestExtractPornComicsComicIDRejectsUnsupportedPath(t *testing.T) {
	t.Parallel()

	if _, err := extractPornComicsComicID("https://porncomics.eu/authors/incognitymous-568"); err == nil {
		t.Fatal("expected error for unsupported url")
	}
}
