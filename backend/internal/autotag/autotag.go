package autotag

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
)

// SuggestedTag is a tag that was identified from a reverse-image search result.
type SuggestedTag struct {
	Name       string             `json:"name"`
	Category   models.TagCategory `json:"category"`
	Confidence float64            `json:"confidence"`
}

// AutoTagResult is the combined outcome of an auto-tag attempt.
type AutoTagResult struct {
	Source        string         `json:"source"`
	Similarity    float64        `json:"similarity"`
	SuggestedTags []SuggestedTag `json:"suggested_tags"`
	SourceURL     string         `json:"source_url,omitempty"`
}

type Service struct {
	db          *database.DB
	saucenao    *sauceNAOClient
	iqdb        *iqdbClient
	threshold   float64
	iqdbEnabled bool
	log         *zap.Logger
}

type Config struct {
	SauceNAOAPIKey string
	IQDBEnabled    bool
	Threshold      float64
	RateLimitMs    int
}

func NewService(db *database.DB, cfg Config, log *zap.Logger) *Service {
	interval := time.Duration(cfg.RateLimitMs) * time.Millisecond
	return &Service{
		db:          db,
		saucenao:    newSauceNAOClient(cfg.SauceNAOAPIKey, interval),
		iqdb:        newIQDBClient(interval),
		threshold:   cfg.Threshold,
		iqdbEnabled: cfg.IQDBEnabled,
		log:         log,
	}
}

// AutoTag performs reverse-image search for the given media item and returns suggested tags.
func (s *Service) AutoTag(ctx context.Context, item *models.Media) (*AutoTagResult, error) {
	imagePath := s.imagePathForMedia(item)
	if imagePath == "" {
		return &AutoTagResult{Source: "none"}, nil
	}

	if s.saucenao.apiKey != "" {
		result, err := s.saucenao.SearchFile(imagePath, s.threshold)
		if err != nil {
			s.log.Warn("autotag: saucenao search failed", zap.String("id", item.ID), zap.Error(err))
		} else if result != nil {
			tags := s.sauceNAOToTags(result)
			return &AutoTagResult{
				Source:        "saucenao",
				Similarity:    result.Similarity,
				SuggestedTags: tags,
				SourceURL:     firstNonEmpty(result.ExternalURLs...),
			}, nil
		}
	}

	if s.iqdbEnabled {
		result, err := s.iqdb.SearchFile(imagePath, s.threshold)
		if err != nil {
			s.log.Warn("autotag: iqdb search failed", zap.String("id", item.ID), zap.Error(err))
		} else if result != nil {
			return &AutoTagResult{
				Source:        "iqdb",
				Similarity:    result.Similarity,
				SuggestedTags: nil,
				SourceURL:     result.SourceURL,
			}, nil
		}
	}

	return &AutoTagResult{Source: "none"}, nil
}

func (s *Service) ApplyTags(ctx context.Context, mediaID string, result *AutoTagResult, tags []SuggestedTag) error {
	now := time.Now()

	_, err := s.db.ExecContext(ctx, `
		UPDATE media SET
			auto_tag_status     = 'completed',
			auto_tag_source     = $1,
			auto_tag_similarity = $2,
			auto_tagged_at      = $3,
			updated_at          = $3
		WHERE id = $4
	`, result.Source, result.Similarity, now, mediaID)
	if err != nil {
		return fmt.Errorf("autotag: update media status: %w", err)
	}

	for _, t := range tags {
		tagID, err := s.ensureTag(ctx, t.Name, t.Category)
		if err != nil {
			s.log.Warn("autotag: ensure tag", zap.String("name", t.Name), zap.Error(err))
			continue
		}
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO media_tags (media_id, tag_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, mediaID, tagID); err != nil {
			s.log.Warn("autotag: insert media_tag", zap.String("tag", t.Name), zap.Error(err))
		}
	}

	return nil
}

func (s *Service) MarkFailed(ctx context.Context, mediaID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE media SET auto_tag_status = 'failed', updated_at = NOW() WHERE id = $1
	`, mediaID)
	return err
}

func (s *Service) MarkProcessing(ctx context.Context, mediaID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE media SET auto_tag_status = 'processing', updated_at = NOW() WHERE id = $1
	`, mediaID)
	return err
}

// imagePathForMedia returns a local image path suitable for reverse-image upload.
func (s *Service) imagePathForMedia(item *models.Media) string {
	candidates := []string{}
	if item.ThumbnailPath != "" {
		candidates = append(candidates, item.ThumbnailPath)
	}
	if item.Type == models.MediaTypeImage {
		candidates = append(candidates, item.FilePath)
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func (s *Service) ensureTag(ctx context.Context, name string, category models.TagCategory) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return "", fmt.Errorf("empty tag name")
	}

	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM tags WHERE name = $1 AND category = $2`, name, string(category),
	).Scan(&id)
	if err == nil {
		return id, nil
	}

	id = uuid.NewString()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO tags (id, name, category, usage_count)
		VALUES ($1, $2, $3, 0)
		ON CONFLICT (name) DO UPDATE SET category = EXCLUDED.category
		RETURNING id
	`, id, name, string(category))
	if err != nil {
		err2 := s.db.QueryRowContext(ctx,
			`SELECT id FROM tags WHERE name = $1`, name,
		).Scan(&id)
		if err2 != nil {
			return "", fmt.Errorf("ensure tag %q: %w", name, err)
		}
	}
	return id, nil
}

func (s *Service) sauceNAOToTags(r *SauceNAOResult) []SuggestedTag {
	var tags []SuggestedTag

	if r.Artist != "" {
		tags = append(tags, SuggestedTag{Name: r.Artist, Category: models.TagCategoryArtist, Confidence: r.Similarity})
	}
	for _, ch := range r.Characters {
		if ch = strings.TrimSpace(ch); ch != "" {
			tags = append(tags, SuggestedTag{Name: ch, Category: models.TagCategoryCharacter, Confidence: r.Similarity})
		}
	}
	if r.Parody != "" {
		tags = append(tags, SuggestedTag{Name: r.Parody, Category: models.TagCategoryParody, Confidence: r.Similarity})
	}
	if r.Title != "" {
		tags = append(tags, SuggestedTag{Name: r.Title, Category: models.TagCategoryMeta, Confidence: r.Similarity})
	}

	return tags
}
