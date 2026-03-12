package autotag

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
	"github.com/nacl-dev/tanuki/internal/tagrules"
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
			tags := NormalizeSuggestedTags(s.sauceNAOToTags(result))
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
	tags = NormalizeSuggestedTags(tags)
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

	expressions := make([]string, 0, len(tags))
	for _, t := range tags {
		expressions = append(expressions, models.FormatTagExpression(t.Name, t.Category))
	}

	resolved, err := tagrules.NewService(s.db).ResolveOrCreate(ctx, expressions)
	if err != nil {
		return fmt.Errorf("autotag: resolve tags: %w", err)
	}

	for _, tag := range resolved {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO media_tags (media_id, tag_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, mediaID, tag.ID); err != nil {
			s.log.Warn("autotag: insert media_tag", zap.String("tag", tag.Name), zap.Error(err))
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

// NormalizeSuggestedTags trims, deduplicates, and normalizes suggested tags so
// manual namespace expressions and source-derived tags follow the same rules.
func NormalizeSuggestedTags(tags []SuggestedTag) []SuggestedTag {
	type normalizedTag struct {
		tag   SuggestedTag
		order int
	}

	byName := make(map[string]normalizedTag, len(tags))
	order := make([]string, 0, len(tags))

	for idx, tag := range tags {
		expression := models.FormatTagExpression(tag.Name, tag.Category)
		parsed := models.ParseTag(expression)
		if parsed.Name == "" {
			continue
		}

		key := parsed.Name
		next := SuggestedTag{
			Name:       parsed.Name,
			Category:   parsed.Category,
			Confidence: tag.Confidence,
		}

		existing, ok := byName[key]
		if !ok {
			byName[key] = normalizedTag{tag: next, order: idx}
			order = append(order, key)
			continue
		}

		if models.ShouldPromoteTagCategory(existing.tag.Category, next.Category) {
			existing.tag.Category = next.Category
		}
		if next.Confidence > existing.tag.Confidence {
			existing.tag.Confidence = next.Confidence
		}
		if existing.tag.Name == "" {
			existing.tag.Name = next.Name
		}
		byName[key] = existing
	}

	sort.SliceStable(order, func(i, j int) bool {
		return byName[order[i]].order < byName[order[j]].order
	})

	normalized := make([]SuggestedTag, 0, len(order))
	for _, key := range order {
		normalized = append(normalized, byName[key].tag)
	}
	return normalized
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
