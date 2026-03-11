package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
)

type CollectionHandler struct {
	db *database.DB
}

type collectionCountRow struct {
	CollectionID string `db:"collection_id"`
	ItemCount    int    `db:"item_count"`
}

type collectionPreviewRow struct {
	CollectionID  string           `db:"collection_id"`
	ID            string           `db:"id"`
	Title         string           `db:"title"`
	Type          models.MediaType `db:"type"`
	ThumbnailPath string           `db:"thumbnail_path"`
	UpdatedAt     time.Time        `db:"updated_at"`
}

type collectionTagRow struct {
	MediaID    string             `db:"media_id"`
	ID         string             `db:"id"`
	Name       string             `db:"name"`
	Category   models.TagCategory `db:"category"`
	UsageCount int                `db:"usage_count"`
}

const collectionMatchPredicateSQL = `
	cm.media_id IS NOT NULL
	OR (
		(c.auto_type IS NOT NULL OR c.auto_title <> '' OR c.auto_tag <> '' OR c.auto_favorite IS NOT NULL OR c.auto_min_rating IS NOT NULL)
		AND (c.auto_type IS NULL OR c.auto_type = '' OR m.type = c.auto_type)
		AND (c.auto_title = '' OR m.title ILIKE ('%' || c.auto_title || '%'))
		AND (c.auto_favorite IS NULL OR m.favorite = c.auto_favorite)
		AND (c.auto_min_rating IS NULL OR m.rating >= c.auto_min_rating)
		AND (
			c.auto_tag = ''
			OR EXISTS (
				SELECT 1
				FROM media_tags mt
				JOIN tags t ON t.id = mt.tag_id
				WHERE mt.media_id = m.id AND LOWER(t.name) = LOWER(c.auto_tag)
			)
		)
	)
`

func (h *CollectionHandler) List(c *gin.Context) {
	userID := c.GetString("userID")

	var collections []models.Collection
	if err := h.db.Select(&collections, `
		SELECT c.*, 0 AS item_count
		FROM collections c
		WHERE c.user_id = $1 OR c.user_id IS NULL
		ORDER BY LOWER(c.name) ASC, c.created_at ASC
	`, userID); err != nil {
		respondError(c, http.StatusInternalServerError, "query collections: "+err.Error())
		return
	}

	if err := h.hydrateCollectionSummaries(collections, 5); err != nil {
		respondError(c, http.StatusInternalServerError, "load collection summaries: "+err.Error())
		return
	}

	respondOK(c, collections, &Meta{Total: len(collections)})
}

func (h *CollectionHandler) Get(c *gin.Context) {
	userID := c.GetString("userID")
	id := c.Param("id")

	var collection models.Collection
	if err := h.db.Get(&collection, `
		SELECT c.*, 0 AS item_count
		FROM collections c
		WHERE c.id = $1 AND (c.user_id = $2 OR c.user_id IS NULL)
	`, id, userID); err != nil {
		respondError(c, http.StatusNotFound, "collection not found")
		return
	}

	items, err := h.listCollectionItems(collection.ID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "load collection items: "+err.Error())
		return
	}
	collection.Items = items
	collection.ItemCount = len(items)

	respondOK(c, collection, nil)
}

func (h *CollectionHandler) Create(c *gin.Context) {
	userID := c.GetString("userID")

	var body struct {
		Name          string `json:"name" binding:"required"`
		Description   string `json:"description"`
		AutoType      string `json:"auto_type"`
		AutoTitle     string `json:"auto_title"`
		AutoTag       string `json:"auto_tag"`
		AutoFavorite  *bool  `json:"auto_favorite"`
		AutoMinRating *int   `json:"auto_min_rating"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	id := uuid.NewString()
	if _, err := h.db.Exec(`
		INSERT INTO collections (id, user_id, name, description, auto_type, auto_title, auto_tag, auto_favorite, auto_min_rating)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, id, userID, body.Name, body.Description, normalizeCollectionAutoType(body.AutoType), strings.TrimSpace(body.AutoTitle), strings.TrimSpace(body.AutoTag), body.AutoFavorite, normalizeCollectionAutoMinRating(body.AutoMinRating)); err != nil {
		respondError(c, http.StatusInternalServerError, "create collection: "+err.Error())
		return
	}

	c.Params = append(c.Params, gin.Param{Key: "id", Value: id})
	h.Get(c)
}

func (h *CollectionHandler) Update(c *gin.Context) {
	userID := c.GetString("userID")
	id := c.Param("id")

	var body struct {
		Name          *string `json:"name"`
		Description   *string `json:"description"`
		AutoType      *string `json:"auto_type"`
		AutoTitle     *string `json:"auto_title"`
		AutoTag       *string `json:"auto_tag"`
		AutoFavorite  *bool   `json:"auto_favorite"`
		AutoMinRating *int    `json:"auto_min_rating"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	var autoType any
	if body.AutoType != nil {
		autoType = normalizeCollectionAutoType(*body.AutoType)
	}
	var autoTag any
	if body.AutoTag != nil {
		autoTag = strings.TrimSpace(*body.AutoTag)
	}
	var autoTitle any
	if body.AutoTitle != nil {
		autoTitle = strings.TrimSpace(*body.AutoTitle)
	}
	autoMinRating := normalizeCollectionAutoMinRating(body.AutoMinRating)

	res, err := h.db.Exec(`
		UPDATE collections SET
			name = COALESCE($3, name),
			description = COALESCE($4, description),
			auto_type = COALESCE($5, auto_type),
			auto_title = COALESCE($6, auto_title),
			auto_tag = COALESCE($7, auto_tag),
			auto_favorite = COALESCE($8, auto_favorite),
			auto_min_rating = COALESCE($9, auto_min_rating),
			updated_at = NOW()
		WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)
	`, id, userID, body.Name, body.Description, autoType, autoTitle, autoTag, body.AutoFavorite, autoMinRating)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "update collection: "+err.Error())
		return
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		respondError(c, http.StatusNotFound, "collection not found")
		return
	}

	h.Get(c)
}

func (h *CollectionHandler) Delete(c *gin.Context) {
	userID := c.GetString("userID")
	id := c.Param("id")

	res, err := h.db.Exec(`DELETE FROM collections WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)`, id, userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "delete collection: "+err.Error())
		return
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		respondError(c, http.StatusNotFound, "collection not found")
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *CollectionHandler) AddMedia(c *gin.Context) {
	userID := c.GetString("userID")
	id := c.Param("id")

	var body struct {
		MediaID string `json:"media_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if !h.collectionOwnedByUser(id, userID) {
		respondError(c, http.StatusNotFound, "collection not found")
		return
	}

	if _, err := h.db.Exec(`
		INSERT INTO media_collections (collection_id, media_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, id, body.MediaID); err != nil {
		respondError(c, http.StatusInternalServerError, "add media to collection: "+err.Error())
		return
	}

	if _, err := h.db.Exec(`UPDATE collections SET updated_at = NOW() WHERE id = $1`, id); err != nil {
		respondError(c, http.StatusInternalServerError, "touch collection: "+err.Error())
		return
	}

	h.Get(c)
}

func (h *CollectionHandler) RemoveMedia(c *gin.Context) {
	userID := c.GetString("userID")
	id := c.Param("id")
	mediaID := c.Param("mediaId")

	if !h.collectionOwnedByUser(id, userID) {
		respondError(c, http.StatusNotFound, "collection not found")
		return
	}

	if _, err := h.db.Exec(`DELETE FROM media_collections WHERE collection_id = $1 AND media_id = $2`, id, mediaID); err != nil {
		respondError(c, http.StatusInternalServerError, "remove media from collection: "+err.Error())
		return
	}
	_, _ = h.db.Exec(`UPDATE collections SET updated_at = NOW() WHERE id = $1`, id)

	h.Get(c)
}

func (h *CollectionHandler) ListForMedia(c *gin.Context) {
	userID := c.GetString("userID")
	mediaID := c.Param("id")

	var collections []models.Collection
	if err := h.db.Select(&collections, `
		SELECT c.*, CASE WHEN EXISTS (
			SELECT 1 FROM media_collections cm WHERE cm.collection_id = c.id AND cm.media_id = $2
		) THEN 1 ELSE 0 END AS item_count
		FROM collections c
		WHERE c.user_id = $1 OR c.user_id IS NULL
		ORDER BY LOWER(c.name) ASC, c.created_at ASC
	`, userID, mediaID); err != nil {
		respondError(c, http.StatusInternalServerError, "query media collections: "+err.Error())
		return
	}

	respondOK(c, collections, &Meta{Total: len(collections)})
}

func (h *CollectionHandler) collectionOwnedByUser(id, userID string) bool {
	var exists bool
	_ = h.db.Get(&exists, `SELECT EXISTS(SELECT 1 FROM collections WHERE id = $1 AND (user_id = $2 OR user_id IS NULL))`, id, userID)
	return exists
}

func (h *CollectionHandler) countCollectionItems(collectionID string) (int, error) {
	rows, err := h.loadCollectionCounts([]string{collectionID})
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return rows[0].ItemCount, nil
}

func (h *CollectionHandler) listCollectionItems(collectionID string) ([]models.Media, error) {
	var items []models.Media
	err := h.db.Select(&items, fmt.Sprintf(`
		SELECT items.*
		FROM (
			SELECT DISTINCT m.*
			FROM media m
			LEFT JOIN media_collections cm
				ON cm.media_id = m.id AND cm.collection_id = $1
			CROSS JOIN collections c
			WHERE c.id = $1
			  AND m.deleted_at IS NULL
			  AND (%s)
		) AS items
		ORDER BY LOWER(items.title) ASC, items.title ASC, items.id ASC
	`, collectionMatchPredicateSQL), collectionID)
	if err != nil {
		return nil, err
	}

	if err := h.loadTagsForMedia(items); err != nil {
		return nil, err
	}

	return items, nil
}

func (h *CollectionHandler) hydrateCollectionSummaries(collections []models.Collection, previewLimit int) error {
	if len(collections) == 0 {
		return nil
	}

	ids := make([]string, 0, len(collections))
	byID := make(map[string]*models.Collection, len(collections))
	for i := range collections {
		ids = append(ids, collections[i].ID)
		byID[collections[i].ID] = &collections[i]
	}

	counts, err := h.loadCollectionCounts(ids)
	if err != nil {
		return err
	}
	for _, row := range counts {
		if collection := byID[row.CollectionID]; collection != nil {
			collection.ItemCount = row.ItemCount
		}
	}

	if previewLimit <= 0 {
		return nil
	}

	previews, err := h.loadCollectionPreviews(ids, previewLimit)
	if err != nil {
		return err
	}
	for collectionID, items := range previews {
		if collection := byID[collectionID]; collection != nil {
			collection.Items = items
		}
	}

	return nil
}

func (h *CollectionHandler) loadCollectionCounts(collectionIDs []string) ([]collectionCountRow, error) {
	if len(collectionIDs) == 0 {
		return nil, nil
	}

	var rows []collectionCountRow
	query := fmt.Sprintf(`
		WITH target_collections AS (
			SELECT *
			FROM collections
			WHERE id = ANY($1)
		),
		collection_items AS (
			SELECT DISTINCT c.id AS collection_id, m.id AS media_id
			FROM target_collections c
			JOIN media m
				ON m.deleted_at IS NULL
			LEFT JOIN media_collections cm
				ON cm.media_id = m.id AND cm.collection_id = c.id
			WHERE %s
		)
		SELECT collection_id, COUNT(*) AS item_count
		FROM collection_items
		GROUP BY collection_id
	`, collectionMatchPredicateSQL)

	if err := h.db.Select(&rows, query, pq.Array(collectionIDs)); err != nil {
		return nil, err
	}
	return rows, nil
}

func (h *CollectionHandler) loadCollectionPreviews(collectionIDs []string, limit int) (map[string][]models.Media, error) {
	if len(collectionIDs) == 0 || limit <= 0 {
		return map[string][]models.Media{}, nil
	}

	var rows []collectionPreviewRow
	query := fmt.Sprintf(`
		WITH target_collections AS (
			SELECT *
			FROM collections
			WHERE id = ANY($1)
		),
		ranked_items AS (
			SELECT
				c.id AS collection_id,
				m.id,
				m.title,
				m.type,
				m.thumbnail_path,
				m.updated_at,
				ROW_NUMBER() OVER (PARTITION BY c.id ORDER BY LOWER(m.title) ASC, m.title ASC, m.id ASC) AS rn
			FROM target_collections c
			JOIN media m
				ON m.deleted_at IS NULL
			LEFT JOIN media_collections cm
				ON cm.media_id = m.id AND cm.collection_id = c.id
			WHERE %s
		)
		SELECT collection_id, id, title, type, thumbnail_path, updated_at
		FROM ranked_items
		WHERE rn <= $2
		ORDER BY collection_id, rn
	`, collectionMatchPredicateSQL)

	if err := h.db.Select(&rows, query, pq.Array(collectionIDs), limit); err != nil {
		return nil, err
	}

	previews := make(map[string][]models.Media, len(collectionIDs))
	for _, row := range rows {
		previews[row.CollectionID] = append(previews[row.CollectionID], models.Media{
			ID:            row.ID,
			Title:         row.Title,
			Type:          row.Type,
			ThumbnailPath: row.ThumbnailPath,
			UpdatedAt:     row.UpdatedAt,
		})
	}

	return previews, nil
}

func (h *CollectionHandler) loadTagsForMedia(items []models.Media) error {
	if len(items) == 0 {
		return nil
	}

	ids := make([]string, 0, len(items))
	byID := make(map[string]*models.Media, len(items))
	for i := range items {
		ids = append(ids, items[i].ID)
		byID[items[i].ID] = &items[i]
	}

	var rows []collectionTagRow
	if err := h.db.Select(&rows, `
		SELECT mt.media_id, t.id, t.name, t.category, t.usage_count
		FROM media_tags mt
		JOIN tags t ON t.id = mt.tag_id
		WHERE mt.media_id = ANY($1)
		ORDER BY t.name ASC
	`, pq.Array(ids)); err != nil {
		return err
	}

	for _, row := range rows {
		if media := byID[row.MediaID]; media != nil {
			media.Tags = append(media.Tags, models.Tag{
				ID:         row.ID,
				Name:       row.Name,
				Category:   row.Category,
				UsageCount: row.UsageCount,
			})
		}
	}

	return nil
}

func normalizeCollectionAutoType(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", string(models.MediaTypeVideo), string(models.MediaTypeImage), string(models.MediaTypeManga), string(models.MediaTypeComic), string(models.MediaTypeDoujinshi):
		return value
	default:
		return ""
	}
}

func normalizeCollectionAutoMinRating(value *int) *int {
	if value == nil {
		return nil
	}
	if *value < 1 || *value > 5 {
		return nil
	}
	return value
}
