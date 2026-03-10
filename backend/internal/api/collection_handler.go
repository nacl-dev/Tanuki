package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
)

type CollectionHandler struct {
	db *database.DB
}

func (h *CollectionHandler) List(c *gin.Context) {
	userID := c.GetString("userID")

	var collections []models.Collection
	if err := h.db.Select(&collections, `
		SELECT c.*, COUNT(cm.media_id) AS item_count
		FROM collections c
		LEFT JOIN media_collections cm ON cm.collection_id = c.id
		WHERE c.user_id = $1 OR c.user_id IS NULL
		GROUP BY c.id
		ORDER BY c.updated_at DESC, c.name ASC
	`, userID); err != nil {
		respondError(c, http.StatusInternalServerError, "query collections: "+err.Error())
		return
	}

	respondOK(c, collections, &Meta{Total: len(collections)})
}

func (h *CollectionHandler) Get(c *gin.Context) {
	userID := c.GetString("userID")
	id := c.Param("id")

	var collection models.Collection
	if err := h.db.Get(&collection, `
		SELECT c.*, COUNT(cm.media_id) AS item_count
		FROM collections c
		LEFT JOIN media_collections cm ON cm.collection_id = c.id
		WHERE c.id = $1 AND (c.user_id = $2 OR c.user_id IS NULL)
		GROUP BY c.id
	`, id, userID); err != nil {
		respondError(c, http.StatusNotFound, "collection not found")
		return
	}

	if err := h.db.Select(&collection.Items, `
		SELECT m.* FROM media m
		JOIN media_collections cm ON cm.media_id = m.id
		WHERE cm.collection_id = $1 AND m.deleted_at IS NULL
		ORDER BY cm.created_at DESC
	`, id); err != nil {
		respondError(c, http.StatusInternalServerError, "load collection items: "+err.Error())
		return
	}

	respondOK(c, collection, nil)
}

func (h *CollectionHandler) Create(c *gin.Context) {
	userID := c.GetString("userID")

	var body struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	id := uuid.NewString()
	if _, err := h.db.Exec(`
		INSERT INTO collections (id, user_id, name, description)
		VALUES ($1, $2, $3, $4)
	`, id, userID, body.Name, body.Description); err != nil {
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
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	res, err := h.db.Exec(`
		UPDATE collections SET
			name = COALESCE($3, name),
			description = COALESCE($4, description),
			updated_at = NOW()
		WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)
	`, id, userID, body.Name, body.Description)
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
		SELECT c.*, CASE WHEN cm.media_id IS NULL THEN 0 ELSE 1 END AS item_count
		FROM collections c
		LEFT JOIN media_collections cm ON cm.collection_id = c.id AND cm.media_id = $2
		WHERE c.user_id = $1 OR c.user_id IS NULL
		ORDER BY c.updated_at DESC, c.name ASC
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
