package api

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nacl-dev/tanuki/internal/models"
	"github.com/nacl-dev/tanuki/internal/tagrules"
)

func (h *TagHandler) ListAliases(c *gin.Context) {
	var rows []models.TagAlias
	if err := h.db.Select(&rows, `
		SELECT id, alias_name, tag_id, created_at
		FROM tag_aliases
		ORDER BY alias_name ASC
	`); err != nil {
		respondError(c, http.StatusInternalServerError, "query aliases: "+err.Error())
		return
	}
	if err := h.hydrateAliasRules(rows); err != nil {
		respondError(c, http.StatusInternalServerError, "load alias targets: "+err.Error())
		return
	}
	respondOK(c, rows, &Meta{Total: len(rows)})
}

func (h *TagHandler) CreateAlias(c *gin.Context) {
	var body struct {
		AliasName string `json:"alias_name" binding:"required"`
		Target    string `json:"target" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	aliasName := tagrules.NormalizeAliasName(body.AliasName)
	if aliasName == "" {
		respondError(c, http.StatusBadRequest, "alias_name is required")
		return
	}

	target, ok := h.resolveRuleTarget(c, body.Target)
	if !ok {
		return
	}

	if aliasName == target.Expression() || aliasName == target.Name {
		respondError(c, http.StatusBadRequest, "alias must differ from the canonical tag")
		return
	}

	var conflicting models.Tag
	if err := h.db.Get(&conflicting, `
		SELECT id, name, category, usage_count
		FROM tags
		WHERE name = $1
		LIMIT 1
	`, aliasName); err == nil && conflicting.ID != target.ID {
		respondError(c, http.StatusBadRequest, "alias conflicts with an existing tag name")
		return
	}

	id := uuid.NewString()
	if _, err := h.db.Exec(`
		INSERT INTO tag_aliases (id, alias_name, tag_id)
		VALUES ($1, $2, $3)
	`, id, aliasName, target.ID); err != nil {
		if isUniqueViolation(err) {
			respondError(c, http.StatusConflict, "alias already exists")
			return
		}
		respondError(c, http.StatusInternalServerError, "create alias: "+err.Error())
		return
	}

	rule := models.TagAlias{
		ID:        id,
		AliasName: aliasName,
		TagID:     target.ID,
		Tag:       target,
		CreatedAt: time.Now(),
	}
	respondOK(c, rule, nil)
}

func (h *TagHandler) DeleteAlias(c *gin.Context) {
	res, err := h.db.Exec(`DELETE FROM tag_aliases WHERE id = $1`, c.Param("id"))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "delete alias: "+err.Error())
		return
	}
	rows, err := res.RowsAffected()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "delete alias: "+err.Error())
		return
	}
	if rows == 0 {
		respondError(c, http.StatusNotFound, "alias not found")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *TagHandler) PreviewMerge(c *gin.Context) {
	var body struct {
		Source string `json:"source" binding:"required"`
		Target string `json:"target" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	preview, err := h.buildMergePreview(c, body.Source, body.Target)
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			respondError(c, http.StatusBadRequest, "source tag not found")
		default:
			respondError(c, http.StatusInternalServerError, "preview merge: "+err.Error())
		}
		return
	}
	if preview.Source.ID == preview.Target.ID {
		respondError(c, http.StatusBadRequest, "source and target must differ")
		return
	}
	respondOK(c, preview, nil)
}

func (h *TagHandler) Merge(c *gin.Context) {
	var body struct {
		Source      string `json:"source" binding:"required"`
		Target      string `json:"target" binding:"required"`
		CreateAlias *bool  `json:"create_alias"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	preview, err := h.buildMergePreview(c, body.Source, body.Target)
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			respondError(c, http.StatusBadRequest, "source tag not found")
		default:
			respondError(c, http.StatusInternalServerError, "prepare merge: "+err.Error())
		}
		return
	}
	if preview.Source.ID == preview.Target.ID {
		respondError(c, http.StatusBadRequest, "source and target must differ")
		return
	}

	createAlias := body.CreateAlias == nil || *body.CreateAlias
	tx, err := h.db.Beginx()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "begin tx: "+err.Error())
		return
	}
	defer tx.Rollback() //nolint:errcheck

	type aliasRow struct {
		AliasName string `db:"alias_name"`
	}
	var aliases []aliasRow
	if err := tx.Select(&aliases, `SELECT alias_name FROM tag_aliases WHERE tag_id = $1 ORDER BY alias_name ASC`, preview.Source.ID); err != nil {
		respondError(c, http.StatusInternalServerError, "load source aliases: "+err.Error())
		return
	}

	type implicationRow struct {
		TagID        string `db:"tag_id"`
		ImpliedTagID string `db:"implied_tag_id"`
	}
	var outbound []implicationRow
	if err := tx.Select(&outbound, `SELECT tag_id, implied_tag_id FROM tag_implications WHERE tag_id = $1`, preview.Source.ID); err != nil {
		respondError(c, http.StatusInternalServerError, "load source implications: "+err.Error())
		return
	}
	var inbound []implicationRow
	if err := tx.Select(&inbound, `SELECT tag_id, implied_tag_id FROM tag_implications WHERE implied_tag_id = $1`, preview.Source.ID); err != nil {
		respondError(c, http.StatusInternalServerError, "load inbound implications: "+err.Error())
		return
	}

	targetID := preview.Target.ID
	if targetID == "" {
		parsed := models.ParseTag(body.Target)
		if parsed.Name == "" {
			respondError(c, http.StatusBadRequest, "target tag is required")
			return
		}
		targetID = uuid.NewString()
		if _, err := tx.Exec(`
			INSERT INTO tags (id, name, category, usage_count)
			VALUES ($1, $2, $3, 0)
		`, targetID, parsed.Name, parsed.Category); err != nil {
			respondError(c, http.StatusInternalServerError, "create target tag: "+err.Error())
			return
		}
		preview.Target = models.Tag{
			ID:       targetID,
			Name:     parsed.Name,
			Category: parsed.Category,
		}
	}

	if _, err := tx.Exec(`
		INSERT INTO media_tags (media_id, tag_id)
		SELECT media_id, $2
		FROM media_tags
		WHERE tag_id = $1
		ON CONFLICT (media_id, tag_id) DO NOTHING
	`, preview.Source.ID, targetID); err != nil {
		respondError(c, http.StatusInternalServerError, "merge media tags: "+err.Error())
		return
	}

	for _, alias := range aliases {
		if _, err := tx.Exec(`
			INSERT INTO tag_aliases (id, alias_name, tag_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (alias_name) DO NOTHING
		`, uuid.NewString(), alias.AliasName, targetID); err != nil {
			respondError(c, http.StatusInternalServerError, "move alias: "+err.Error())
			return
		}
	}

	if createAlias {
		if _, err := tx.Exec(`
			INSERT INTO tag_aliases (id, alias_name, tag_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (alias_name) DO NOTHING
		`, uuid.NewString(), NormalizeRuleAlias(preview.Source), targetID); err != nil {
			respondError(c, http.StatusInternalServerError, "create merge alias: "+err.Error())
			return
		}
	}

	for _, rule := range outbound {
		if rule.ImpliedTagID == targetID {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO tag_implications (id, tag_id, implied_tag_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (tag_id, implied_tag_id) DO NOTHING
		`, uuid.NewString(), targetID, rule.ImpliedTagID); err != nil {
			respondError(c, http.StatusInternalServerError, "move outbound implication: "+err.Error())
			return
		}
	}

	for _, rule := range inbound {
		if rule.TagID == targetID {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO tag_implications (id, tag_id, implied_tag_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (tag_id, implied_tag_id) DO NOTHING
		`, uuid.NewString(), rule.TagID, targetID); err != nil {
			respondError(c, http.StatusInternalServerError, "move inbound implication: "+err.Error())
			return
		}
	}

	if _, err := tx.Exec(`DELETE FROM tags WHERE id = $1`, preview.Source.ID); err != nil {
		respondError(c, http.StatusInternalServerError, "delete source tag: "+err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		respondError(c, http.StatusInternalServerError, "commit merge: "+err.Error())
		return
	}

	respondOK(c, gin.H{
		"source":           preview.Source,
		"target":           preview.Target,
		"preview":          preview,
		"created_alias":    createAlias,
		"moved_media_tags": preview.SourceMediaCount - preview.OverlappingMediaCount,
	}, nil)
}

func (h *TagHandler) ListImplications(c *gin.Context) {
	var rows []models.TagImplication
	if err := h.db.Select(&rows, `
		SELECT id, tag_id, implied_tag_id, created_at
		FROM tag_implications
		ORDER BY created_at DESC
	`); err != nil {
		respondError(c, http.StatusInternalServerError, "query implications: "+err.Error())
		return
	}
	if err := h.hydrateImplicationRules(rows); err != nil {
		respondError(c, http.StatusInternalServerError, "load implication tags: "+err.Error())
		return
	}
	respondOK(c, rows, &Meta{Total: len(rows)})
}

func (h *TagHandler) CreateImplication(c *gin.Context) {
	var body struct {
		Source  string `json:"source" binding:"required"`
		Implied string `json:"implied" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	source, ok := h.resolveRuleTarget(c, body.Source)
	if !ok {
		return
	}
	implied, ok := h.resolveRuleTarget(c, body.Implied)
	if !ok {
		return
	}
	if source.ID == implied.ID {
		respondError(c, http.StatusBadRequest, "a tag cannot imply itself")
		return
	}

	id := uuid.NewString()
	if _, err := h.db.Exec(`
		INSERT INTO tag_implications (id, tag_id, implied_tag_id)
		VALUES ($1, $2, $3)
	`, id, source.ID, implied.ID); err != nil {
		if isUniqueViolation(err) {
			respondError(c, http.StatusConflict, "implication already exists")
			return
		}
		respondError(c, http.StatusInternalServerError, "create implication: "+err.Error())
		return
	}

	rule := models.TagImplication{
		ID:           id,
		TagID:        source.ID,
		ImpliedTagID: implied.ID,
		Tag:          source,
		ImpliedTag:   implied,
		CreatedAt:    time.Now(),
	}
	respondOK(c, rule, nil)
}

func (h *TagHandler) DeleteImplication(c *gin.Context) {
	res, err := h.db.Exec(`DELETE FROM tag_implications WHERE id = $1`, c.Param("id"))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "delete implication: "+err.Error())
		return
	}
	rows, err := res.RowsAffected()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "delete implication: "+err.Error())
		return
	}
	if rows == 0 {
		respondError(c, http.StatusNotFound, "implication not found")
		return
	}
	c.Status(http.StatusNoContent)
}

type tagMergePreview struct {
	Source                     models.Tag `json:"source"`
	Target                     models.Tag `json:"target"`
	TargetCreated              bool       `json:"target_created"`
	SourceMediaCount           int        `json:"source_media_count"`
	TargetMediaCount           int        `json:"target_media_count"`
	OverlappingMediaCount      int        `json:"overlapping_media_count"`
	SourceAliasCount           int        `json:"source_alias_count"`
	SourceOutboundImplications int        `json:"source_outbound_implications"`
	SourceInboundImplications  int        `json:"source_inbound_implications"`
}

func (h *TagHandler) buildMergePreview(c *gin.Context, sourceRaw, targetRaw string) (tagMergePreview, error) {
	source, err := h.ruleService().FindExistingByExpression(c.Request.Context(), sourceRaw)
	if err != nil {
		return tagMergePreview{}, err
	}
	target, err := h.ruleService().FindExistingByExpression(c.Request.Context(), targetRaw)
	targetCreated := false
	if err != nil {
		if err != sql.ErrNoRows {
			return tagMergePreview{}, err
		}
		parsed := models.ParseTag(targetRaw)
		if parsed.Name == "" {
			return tagMergePreview{}, sql.ErrNoRows
		}
		target = models.Tag{
			Name:     parsed.Name,
			Category: parsed.Category,
		}
		targetCreated = true
	}

	preview := tagMergePreview{
		Source:        source,
		Target:        target,
		TargetCreated: targetCreated,
	}

	if err := h.db.Get(&preview.SourceMediaCount, `
		SELECT COUNT(*)
		FROM media_tags mt
		JOIN media m ON m.id = mt.media_id
		WHERE mt.tag_id = $1 AND m.deleted_at IS NULL
	`, source.ID); err != nil {
		return tagMergePreview{}, err
	}
	if target.ID != "" {
		if err := h.db.Get(&preview.TargetMediaCount, `
			SELECT COUNT(*)
			FROM media_tags mt
			JOIN media m ON m.id = mt.media_id
			WHERE mt.tag_id = $1 AND m.deleted_at IS NULL
		`, target.ID); err != nil {
			return tagMergePreview{}, err
		}
		if err := h.db.Get(&preview.OverlappingMediaCount, `
			SELECT COUNT(*)
			FROM media_tags src
			JOIN media_tags dst ON dst.media_id = src.media_id AND dst.tag_id = $2
			JOIN media m ON m.id = src.media_id
			WHERE src.tag_id = $1 AND m.deleted_at IS NULL
		`, source.ID, target.ID); err != nil {
			return tagMergePreview{}, err
		}
	}
	if err := h.db.Get(&preview.SourceAliasCount, `SELECT COUNT(*) FROM tag_aliases WHERE tag_id = $1`, source.ID); err != nil {
		return tagMergePreview{}, err
	}
	if err := h.db.Get(&preview.SourceOutboundImplications, `SELECT COUNT(*) FROM tag_implications WHERE tag_id = $1`, source.ID); err != nil {
		return tagMergePreview{}, err
	}
	if err := h.db.Get(&preview.SourceInboundImplications, `SELECT COUNT(*) FROM tag_implications WHERE implied_tag_id = $1`, source.ID); err != nil {
		return tagMergePreview{}, err
	}

	return preview, nil
}

func NormalizeRuleAlias(tag models.Tag) string {
	return tagrules.NormalizeAliasName(tag.Expression())
}
