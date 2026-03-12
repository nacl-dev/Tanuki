package autotag

import (
	"testing"

	"github.com/nacl-dev/tanuki/internal/models"
)

func TestNormalizeSuggestedTagsPromotesNamespacesAndDeduplicates(t *testing.T) {
	t.Parallel()

	got := NormalizeSuggestedTags([]SuggestedTag{
		{Name: " Naruto ", Category: models.TagCategoryGeneral, Confidence: 15},
		{Name: "parody:Naruto", Category: models.TagCategoryGeneral, Confidence: 81},
		{Name: "artist:Circle Name", Category: models.TagCategoryGeneral, Confidence: 44},
		{Name: "circle name", Category: models.TagCategoryArtist, Confidence: 62},
		{Name: "", Category: models.TagCategoryGeneral, Confidence: 99},
	})

	if len(got) != 2 {
		t.Fatalf("expected 2 normalized tags, got %d", len(got))
	}

	if got[0].Name != "naruto" || got[0].Category != models.TagCategoryParody || got[0].Confidence != 81 {
		t.Fatalf("unexpected first tag: %+v", got[0])
	}
	if got[1].Name != "circle name" || got[1].Category != models.TagCategoryArtist || got[1].Confidence != 62 {
		t.Fatalf("unexpected second tag: %+v", got[1])
	}
}
