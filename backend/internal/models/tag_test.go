package models

import "testing"

func TestParseTag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		raw        string
		wantName   string
		wantCat    TagCategory
		namespaced bool
	}{
		{
			name:       "plain general tag",
			raw:        " Tentacles ",
			wantName:   "tentacles",
			wantCat:    TagCategoryGeneral,
			namespaced: false,
		},
		{
			name:       "artist namespace",
			raw:        "artist:John Doe",
			wantName:   "john doe",
			wantCat:    TagCategoryArtist,
			namespaced: true,
		},
		{
			name:       "series maps to parody",
			raw:        "series:Naruto",
			wantName:   "naruto",
			wantCat:    TagCategoryParody,
			namespaced: true,
		},
		{
			name:       "rating maps to meta",
			raw:        "rating:Explicit",
			wantName:   "explicit",
			wantCat:    TagCategoryMeta,
			namespaced: true,
		},
		{
			name:       "unknown namespace stays literal",
			raw:        "engine:foo",
			wantName:   "engine:foo",
			wantCat:    TagCategoryGeneral,
			namespaced: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ParseTag(tt.raw)
			if got.Name != tt.wantName {
				t.Fatalf("ParseTag(%q) name = %q, want %q", tt.raw, got.Name, tt.wantName)
			}
			if got.Category != tt.wantCat {
				t.Fatalf("ParseTag(%q) category = %q, want %q", tt.raw, got.Category, tt.wantCat)
			}
			if got.Namespaced != tt.namespaced {
				t.Fatalf("ParseTag(%q) namespaced = %v, want %v", tt.raw, got.Namespaced, tt.namespaced)
			}
		})
	}
}

func TestShouldPromoteTagCategory(t *testing.T) {
	t.Parallel()

	if !ShouldPromoteTagCategory(TagCategoryGeneral, TagCategoryArtist) {
		t.Fatal("expected general tag to be promotable to artist")
	}
	if !ShouldPromoteTagCategory(TagCategoryMeta, TagCategoryParody) {
		t.Fatal("expected meta tag to be promotable to parody")
	}
	if ShouldPromoteTagCategory(TagCategoryArtist, TagCategoryCharacter) {
		t.Fatal("did not expect artist tag to be downgraded or switched sideways")
	}
}
