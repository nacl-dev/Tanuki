package dedup

import (
	"context"
	"fmt"

	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
)

// DuplicateItem is a media item that was found to be a near-duplicate.
type DuplicateItem struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Type          string  `json:"type"`
	FileSize      int64   `json:"file_size"`
	ThumbnailPath string  `json:"thumbnail_path"`
	Similarity    float64 `json:"similarity"` // 0–100
	Distance      int     `json:"distance"`   // Hamming distance
}

// DuplicateGroup groups a reference item with its near-duplicate matches.
type DuplicateGroup struct {
	GroupID   int             `json:"group_id"`
	Reference DuplicateItem   `json:"reference"`
	Matches   []DuplicateItem `json:"matches"`
	Count     int             `json:"count"`
}

// Service orchestrates perceptual-hash computation and duplicate detection.
type Service struct {
	db        *database.DB
	threshold int
	log       *zap.Logger
}

// NewService constructs a dedup Service.
func NewService(db *database.DB, threshold int, log *zap.Logger) *Service {
	return &Service{db: db, threshold: threshold, log: log}
}

// ComputeAndStore calculates the pHash for the given media item and persists it.
func (s *Service) ComputeAndStore(ctx context.Context, item *models.Media) error {
	// Only images and video thumbnails are hashable via the image decoder.
	// For videos, we use the generated thumbnail.
	filePath := item.FilePath
	if item.Type == models.MediaTypeVideo || item.Type == models.MediaTypeManga ||
		item.Type == models.MediaTypeComic || item.Type == models.MediaTypeDoujinshi {
		if item.ThumbnailPath == "" {
			return nil // nothing to hash yet
		}
		filePath = item.ThumbnailPath
	}

	hash, err := ComputeFromFile(ctx, filePath)
	if err != nil {
		return fmt.Errorf("dedup: compute phash for %s: %w", item.ID, err)
	}

	hashInt := int64(hash) //nolint:gosec // intentional bit reinterpretation
	_, err = s.db.ExecContext(ctx, `
		UPDATE media SET phash = $1, phash_computed_at = NOW(), updated_at = NOW()
		WHERE id = $2
	`, hashInt, item.ID)
	return err
}

// FindDuplicates returns all media items whose pHash is within the configured
// Hamming-distance threshold of the given item's pHash.
func (s *Service) FindDuplicates(ctx context.Context, mediaID string) ([]DuplicateItem, error) {
	// Fetch the reference item's hash
	var refHash *int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT phash FROM media WHERE id = $1 AND deleted_at IS NULL`, mediaID,
	).Scan(&refHash); err != nil {
		return nil, fmt.Errorf("dedup: fetch reference hash: %w", err)
	}
	if refHash == nil {
		return nil, nil // no hash computed yet
	}

	// Fetch all other hashed items
	var candidates []struct {
		models.Media
		PHashVal int64 `db:"phash_val"`
	}
	if err := s.db.SelectContext(ctx, &candidates, `
		SELECT id, title, type, file_size, thumbnail_path, phash AS phash_val
		FROM media
		WHERE deleted_at IS NULL AND phash IS NOT NULL AND id != $1
	`, mediaID); err != nil {
		return nil, fmt.Errorf("dedup: fetch candidates: %w", err)
	}

	ref := uint64(*refHash) //nolint:gosec
	var results []DuplicateItem
	for _, c := range candidates {
		candidate := uint64(c.PHashVal) //nolint:gosec
		dist := HammingDistance(ref, candidate)
		if dist <= s.threshold {
			results = append(results, DuplicateItem{
				ID:            c.Media.ID,
				Title:         c.Media.Title,
				Type:          string(c.Media.Type),
				FileSize:      c.Media.FileSize,
				ThumbnailPath: c.Media.ThumbnailPath,
				Similarity:    Similarity(dist),
				Distance:      dist,
			})
		}
	}
	return results, nil
}

// ListDuplicateGroups returns all duplicate groups across the entire library.
// Each group consists of one reference item and its near-duplicates.
func (s *Service) ListDuplicateGroups(ctx context.Context) ([]DuplicateGroup, error) {
	var items []struct {
		ID            string `db:"id"`
		Title         string `db:"title"`
		Type          string `db:"type"`
		FileSize      int64  `db:"file_size"`
		ThumbnailPath string `db:"thumbnail_path"`
		PHashVal      int64  `db:"phash_val"`
	}
	if err := s.db.SelectContext(ctx, &items, `
		SELECT id, title, type, file_size, thumbnail_path, phash AS phash_val
		FROM media
		WHERE deleted_at IS NULL AND phash IS NOT NULL
		ORDER BY created_at ASC
	`); err != nil {
		return nil, fmt.Errorf("dedup: fetch hashed items: %w", err)
	}

	// Union-find grouping
	parent := make([]int, len(items))
	for i := range parent {
		parent[i] = i
	}

	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) {
		pa, pb := find(a), find(b)
		if pa != pb {
			parent[pa] = pb
		}
	}

	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			a := uint64(items[i].PHashVal) //nolint:gosec
			b := uint64(items[j].PHashVal) //nolint:gosec
			if IsDuplicate(a, b, s.threshold) {
				union(i, j)
			}
		}
	}

	// Collect groups
	groupMap := make(map[int][]int)
	for i := range items {
		root := find(i)
		groupMap[root] = append(groupMap[root], i)
	}

	var groups []DuplicateGroup
	gid := 0
	for _, members := range groupMap {
		if len(members) < 2 {
			continue
		}
		gid++
		ref := members[0]
		refItem := DuplicateItem{
			ID:            items[ref].ID,
			Title:         items[ref].Title,
			Type:          items[ref].Type,
			FileSize:      items[ref].FileSize,
			ThumbnailPath: items[ref].ThumbnailPath,
			Similarity:    100,
		}
		var matches []DuplicateItem
		for _, m := range members[1:] {
			a := uint64(items[ref].PHashVal)  //nolint:gosec
			b := uint64(items[m].PHashVal)    //nolint:gosec
			dist := HammingDistance(a, b)
			matches = append(matches, DuplicateItem{
				ID:            items[m].ID,
				Title:         items[m].Title,
				Type:          items[m].Type,
				FileSize:      items[m].FileSize,
				ThumbnailPath: items[m].ThumbnailPath,
				Similarity:    Similarity(dist),
				Distance:      dist,
			})
		}
		groups = append(groups, DuplicateGroup{
			GroupID:   gid,
			Reference: refItem,
			Matches:   matches,
			Count:     len(members),
		})
	}
	return groups, nil
}
