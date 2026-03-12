package tagrules

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/nacl-dev/tanuki/internal/models"
)

type Store interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func NormalizeAliasName(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func (s *Service) ResolveOrCreate(ctx context.Context, rawTags []string) ([]models.Tag, error) {
	parsedByName := make(map[string]models.ParsedTag)
	for _, raw := range rawTags {
		parsed := models.ParseTag(raw)
		if parsed.Name == "" {
			continue
		}
		existing, ok := parsedByName[parsed.Name]
		if !ok || models.ShouldPromoteTagCategory(existing.Category, parsed.Category) {
			parsedByName[parsed.Name] = parsed
		}
	}

	resolved := make([]models.Tag, 0, len(parsedByName))
	seen := make(map[string]struct{}, len(parsedByName))
	for _, parsed := range parsedByName {
		tag, err := s.resolveParsed(ctx, parsed)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[tag.ID]; ok {
			continue
		}
		seen[tag.ID] = struct{}{}
		resolved = append(resolved, tag)
	}

	return s.expandImplications(ctx, resolved)
}

func (s *Service) CanonicalizeExpression(ctx context.Context, raw string) (string, error) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return "", nil
	}

	parsed := models.ParseTag(normalized)
	tag, err := s.findByExpression(ctx, parsed.Raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if err == nil {
		return tag.Expression(), nil
	}

	tag, err = s.findByExpression(ctx, parsed.Name)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if err == nil {
		return tag.Expression(), nil
	}

	return parsed.Raw, nil
}

func (s *Service) ResolveExistingOrCreate(ctx context.Context, raw string) (models.Tag, error) {
	parsed := models.ParseTag(raw)
	if parsed.Name == "" {
		return models.Tag{}, sql.ErrNoRows
	}
	return s.resolveParsed(ctx, parsed)
}

func (s *Service) FindExistingByExpression(ctx context.Context, raw string) (models.Tag, error) {
	parsed := models.ParseTag(strings.TrimSpace(raw))
	if parsed.Name == "" {
		return models.Tag{}, sql.ErrNoRows
	}

	tag, err := s.findByExpression(ctx, parsed.Raw)
	if err == nil || !errors.Is(err, sql.ErrNoRows) {
		return tag, err
	}

	if parsed.Raw != parsed.Name {
		return s.findByExpression(ctx, parsed.Name)
	}
	return models.Tag{}, sql.ErrNoRows
}

func (s *Service) resolveParsed(ctx context.Context, parsed models.ParsedTag) (models.Tag, error) {
	tag, err := s.findTagByName(ctx, parsed.Name)
	if err == nil {
		if models.ShouldPromoteTagCategory(tag.Category, parsed.Category) {
			if _, updateErr := s.store.ExecContext(ctx, `UPDATE tags SET category = $2 WHERE id = $1`, tag.ID, parsed.Category); updateErr != nil {
				return models.Tag{}, updateErr
			}
			tag.Category = parsed.Category
		}
		return tag, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return models.Tag{}, err
	}

	tag, err = s.findTagByAlias(ctx, parsed.Raw)
	if err == nil {
		return tag, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return models.Tag{}, err
	}

	id := uuid.NewString()
	if _, err := s.store.ExecContext(ctx, `
		INSERT INTO tags (id, name, category, usage_count)
		VALUES ($1, $2, $3, 0)
		ON CONFLICT (name) DO NOTHING
	`, id, parsed.Name, parsed.Category); err != nil {
		return models.Tag{}, err
	}

	return s.findTagByName(ctx, parsed.Name)
}

func (s *Service) expandImplications(ctx context.Context, base []models.Tag) ([]models.Tag, error) {
	queue := make([]models.Tag, 0, len(base))
	seen := make(map[string]models.Tag, len(base))
	for _, tag := range base {
		seen[tag.ID] = tag
		queue = append(queue, tag)
	}

	for len(queue) > 0 {
		tag := queue[0]
		queue = queue[1:]

		var implied []models.Tag
		if err := s.store.SelectContext(ctx, &implied, `
			SELECT t.id, t.name, t.category, t.usage_count
			FROM tag_implications ti
			JOIN tags t ON t.id = ti.implied_tag_id
			WHERE ti.tag_id = $1
			ORDER BY t.name ASC
		`, tag.ID); err != nil {
			return nil, err
		}

		for _, impliedTag := range implied {
			if _, ok := seen[impliedTag.ID]; ok {
				continue
			}
			seen[impliedTag.ID] = impliedTag
			queue = append(queue, impliedTag)
		}
	}

	out := make([]models.Tag, 0, len(seen))
	for _, tag := range seen {
		out = append(out, tag)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category == out[j].Category {
			return out[i].Name < out[j].Name
		}
		return out[i].Expression() < out[j].Expression()
	})
	return out, nil
}

func (s *Service) findByExpression(ctx context.Context, expression string) (models.Tag, error) {
	var tag models.Tag
	err := s.store.GetContext(ctx, &tag, `
		SELECT t.id, t.name, t.category, t.usage_count
		FROM tags t
		WHERE LOWER(t.name) = LOWER($1)
		   OR EXISTS (
				SELECT 1
				FROM tag_aliases ta
				WHERE ta.tag_id = t.id
				  AND LOWER(ta.alias_name) = LOWER($1)
		   )
		LIMIT 1
	`, NormalizeAliasName(expression))
	return tag, err
}

func (s *Service) findTagByName(ctx context.Context, name string) (models.Tag, error) {
	var tag models.Tag
	err := s.store.GetContext(ctx, &tag, `
		SELECT id, name, category, usage_count
		FROM tags
		WHERE LOWER(name) = LOWER($1)
		LIMIT 1
	`, strings.ToLower(strings.TrimSpace(name)))
	return tag, err
}

func (s *Service) findTagByAlias(ctx context.Context, alias string) (models.Tag, error) {
	var tag models.Tag
	err := s.store.GetContext(ctx, &tag, `
		SELECT t.id, t.name, t.category, t.usage_count
		FROM tag_aliases ta
		JOIN tags t ON t.id = ta.tag_id
		WHERE ta.alias_name = $1
		LIMIT 1
	`, NormalizeAliasName(alias))
	return tag, err
}
