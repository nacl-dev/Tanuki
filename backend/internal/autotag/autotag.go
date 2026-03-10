package autotag

import (
	"context"
	"fmt"
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
	Confidence float64            `json:"confidence"` // 0–100
}

// AutoTagResult is the combined outcome of an auto-tag attempt.
type AutoTagResult struct {
	Source        string         `json:"source"`     // "saucenao" | "iqdb" | "none"
	Similarity    float64        `json:"similarity"` // 0–100
	SuggestedTags []SuggestedTag `json:"suggested_tags"`
	SourceURL     string         `json:"source_url,omitempty"`
}

// Service orchestrates auto-tagging for media items.
type Service struct {
	db         *database.DB
	saucenao   *sauceNAOClient
	iqdb       *iqdbClient
	threshold  float64
	iqdbEnabled bool
	log        *zap.Logger
}

// Config holds settings for the auto-tag service.
type Config struct {
	SauceNAOAPIKey  string
	IQDBEnabled     bool
	Threshold       float64 // 0–100
	RateLimitMs     int
}

// NewService constructs an auto-tag Service.
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
// It does NOT persist anything – the caller decides whether to apply the suggestions.
func (s *Service) AutoTag(ctx context.Context, item *models.Media) (*AutoTagResult, error) {
	imageURL := s.imageURLForMedia(item)
	if imageURL == "" {
		return &AutoTagResult{Source: "none"}, nil
	}

	// 1. Try SauceNAO first (if API key is configured)
	if s.saucenao.apiKey != "" {
		result, err := s.saucenao.Search(imageURL, s.threshold)
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

	// 2. Fall back to IQDB
	if s.iqdbEnabled {
		result, err := s.iqdb.Search(imageURL, s.threshold)
		if err != nil {
			s.log.Warn("autotag: iqdb search failed", zap.String("id", item.ID), zap.Error(err))
		} else if result != nil {
			return &AutoTagResult{
				Source:        "iqdb",
				Similarity:    result.Similarity,
				SuggestedTags: nil, // IQDB doesn't return structured tags
				SourceURL:     result.SourceURL,
			}, nil
		}
	}

	return &AutoTagResult{Source: "none"}, nil
}

// ApplyTags persists the given tag list on the media item, creating missing tags as needed.
// It also updates the auto_tag_* columns on the media row.
func (s *Service) ApplyTags(ctx context.Context, mediaID string, result *AutoTagResult, tags []SuggestedTag) error {
	now := time.Now()

	// Update auto-tag metadata
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

// MarkFailed records an auto-tag failure for the given media item.
func (s *Service) MarkFailed(ctx context.Context, mediaID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE media SET auto_tag_status = 'failed', updated_at = NOW() WHERE id = $1
	`, mediaID)
	return err
}

// MarkProcessing marks the item as currently being auto-tagged.
func (s *Service) MarkProcessing(ctx context.Context, mediaID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE media SET auto_tag_status = 'processing', updated_at = NOW() WHERE id = $1
	`, mediaID)
	return err
}

// imageURLForMedia returns a publicly-accessible URL for the media item's thumbnail or file.
// Since items are served locally, we use the local file path via the API.
func (s *Service) imageURLForMedia(item *models.Media) string {
	// We construct a localhost API URL that the worker can reach.
	// This requires the API server to be running, but that's standard in our setup.
	// Use thumbnail when available (smaller, faster to upload/analyse).
	if item.ThumbnailPath != "" {
		return fmt.Sprintf("http://localhost:8080/api/media/%s/thumbnail", item.ID)
	}
	if item.Type == models.MediaTypeImage {
		return fmt.Sprintf("http://localhost:8080/api/media/%s/file", item.ID)
	}
	return ""
}

// ensureTag returns the ID of an existing tag matching name+category, creating it if absent.
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

	// Create new tag
	id = uuid.NewString()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO tags (id, name, category, usage_count)
		VALUES ($1, $2, $3, 0)
		ON CONFLICT (name) DO UPDATE SET category = EXCLUDED.category
		RETURNING id
	`, id, name, string(category))
	if err != nil {
		// Re-fetch in case of concurrent insert
		err2 := s.db.QueryRowContext(ctx,
			`SELECT id FROM tags WHERE name = $1`, name,
		).Scan(&id)
		if err2 != nil {
			return "", fmt.Errorf("ensure tag %q: %w", name, err)
		}
	}
	return id, nil
}

// sauceNAOToTags converts a SauceNAO result into structured SuggestedTags.
func (s *Service) sauceNAOToTags(r *SauceNAOResult) []SuggestedTag {
	var tags []SuggestedTag

	if r.Artist != "" {
		tags = append(tags, SuggestedTag{
			Name:       r.Artist,
			Category:   models.TagCategoryArtist,
			Confidence: r.Similarity,
		})
	}

	for _, ch := range r.Characters {
		if ch = strings.TrimSpace(ch); ch != "" {
			tags = append(tags, SuggestedTag{
				Name:       ch,
				Category:   models.TagCategoryCharacter,
				Confidence: r.Similarity,
			})
		}
	}

	if r.Parody != "" {
		tags = append(tags, SuggestedTag{
			Name:       r.Parody,
			Category:   models.TagCategoryParody,
			Confidence: r.Similarity,
		})
	}

	if r.Title != "" {
		tags = append(tags, SuggestedTag{
			Name:       r.Title,
			Category:   models.TagCategoryMeta,
			Confidence: r.Similarity,
		})
	}

	return tags
}
