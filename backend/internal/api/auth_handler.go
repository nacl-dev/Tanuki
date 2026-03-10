package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nacl-dev/tanuki/internal/auth"
	"github.com/nacl-dev/tanuki/internal/config"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
)

// AuthHandler handles user registration, login, and profile management.
type AuthHandler struct {
	db  *database.DB
	cfg *config.Config
}

// ─── Request / Response types ─────────────────────────────────────────────────

type registerRequest struct {
	Username    string `json:"username"     binding:"required"`
	Email       string `json:"email"        binding:"required"`
	Password    string `json:"password"     binding:"required"`
	DisplayName string `json:"display_name"`
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type updateProfileRequest struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
}

type loginResponse struct {
	Token string       `json:"token"`
	User  *models.User `json:"user"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// Register creates a new user account.
// POST /api/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if !h.cfg.RegistrationEnabled {
		respondError(c, http.StatusForbidden, "registration is disabled")
		return
	}

	// Determine role: first user becomes admin.
	role := models.RoleUser
	var count int
	if err := h.db.Get(&count, `SELECT COUNT(*) FROM users`); err != nil {
		respondError(c, http.StatusInternalServerError, "database error")
		return
	}
	if count == 0 {
		role = models.RoleAdmin
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to hash password")
		return
	}

	var user models.User
	err = h.db.QueryRowx(`
		INSERT INTO users (username, email, password_hash, display_name, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, username, email, password_hash, display_name, role, is_active, created_at, updated_at
	`, req.Username, req.Email, hash, req.DisplayName, string(role)).StructScan(&user)
	if err != nil {
		if isUniqueViolation(err) {
			respondError(c, http.StatusConflict, "username or email already taken")
			return
		}
		respondError(c, http.StatusInternalServerError, "failed to create user")
		return
	}

	respondOK(c, &user, nil)
}

// Login authenticates a user and returns a JWT.
// POST /api/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	var user models.User
	err := h.db.QueryRowx(`
		SELECT id, username, email, password_hash, display_name, role, is_active, created_at, updated_at
		FROM users
		WHERE (username = $1 OR email = $1) AND is_active = TRUE
	`, req.Username).StructScan(&user)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := auth.CheckPassword(user.PasswordHash, req.Password); err != nil {
		respondError(c, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := auth.GenerateToken(user.ID, string(user.Role), h.cfg.JWTSecret, h.cfg.JWTExpiryHours)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to generate token")
		return
	}

	respondOK(c, loginResponse{Token: token, User: &user}, nil)
}

// Me returns the currently authenticated user.
// GET /api/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	userID, _ := c.Get("userID")

	var user models.User
	err := h.db.QueryRowx(`
		SELECT id, username, email, password_hash, display_name, role, is_active, created_at, updated_at
		FROM users WHERE id = $1
	`, userID).StructScan(&user)
	if err != nil {
		respondError(c, http.StatusNotFound, "user not found")
		return
	}

	respondOK(c, &user, nil)
}

// UpdateMe updates the current user's profile.
// PATCH /api/auth/me
func (h *AuthHandler) UpdateMe(c *gin.Context) {
	userID, _ := c.Get("userID")

	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Fetch existing user
	var user models.User
	if err := h.db.QueryRowx(`
		SELECT id, username, email, password_hash, display_name, role, is_active, created_at, updated_at
		FROM users WHERE id = $1
	`, userID).StructScan(&user); err != nil {
		respondError(c, http.StatusNotFound, "user not found")
		return
	}

	if req.DisplayName != "" {
		user.DisplayName = req.DisplayName
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Password != "" {
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "failed to hash password")
			return
		}
		user.PasswordHash = hash
	}

	user.UpdatedAt = time.Now()

	_, err := h.db.Exec(`
		UPDATE users SET display_name=$1, email=$2, password_hash=$3, updated_at=$4
		WHERE id=$5
	`, user.DisplayName, user.Email, user.PasswordHash, user.UpdatedAt, userID)
	if err != nil {
		if isUniqueViolation(err) {
			respondError(c, http.StatusConflict, "email already taken")
			return
		}
		respondError(c, http.StatusInternalServerError, "failed to update user")
		return
	}

	respondOK(c, &user, nil)
}

// ─── Admin handlers ───────────────────────────────────────────────────────────

// ListUsers returns all users (admin only).
// GET /api/admin/users
func (h *AuthHandler) ListUsers(c *gin.Context) {
	var users []models.User
	if err := h.db.Select(&users, `
		SELECT id, username, email, password_hash, display_name, role, is_active, created_at, updated_at
		FROM users ORDER BY created_at ASC
	`); err != nil {
		respondError(c, http.StatusInternalServerError, "database error")
		return
	}
	respondOK(c, users, nil)
}

// UpdateUser updates any user's profile (admin only).
// PATCH /api/admin/users/:id
func (h *AuthHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")

	type adminUpdateRequest struct {
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Role        string `json:"role"`
		IsActive    *bool  `json:"is_active"`
	}

	var req adminUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	var user models.User
	if err := h.db.QueryRowx(`
		SELECT id, username, email, password_hash, display_name, role, is_active, created_at, updated_at
		FROM users WHERE id = $1
	`, id).StructScan(&user); err != nil {
		respondError(c, http.StatusNotFound, "user not found")
		return
	}

	if req.DisplayName != "" {
		user.DisplayName = req.DisplayName
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Role == string(models.RoleAdmin) || req.Role == string(models.RoleUser) {
		user.Role = models.UserRole(req.Role)
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}
	user.UpdatedAt = time.Now()

	_, err := h.db.Exec(`
		UPDATE users SET display_name=$1, email=$2, role=$3, is_active=$4, updated_at=$5
		WHERE id=$6
	`, user.DisplayName, user.Email, user.Role, user.IsActive, user.UpdatedAt, id)
	if err != nil {
		if isUniqueViolation(err) {
			respondError(c, http.StatusConflict, "email already taken")
			return
		}
		respondError(c, http.StatusInternalServerError, "failed to update user")
		return
	}

	respondOK(c, &user, nil)
}

// DeleteUser deletes a user account (admin only).
// DELETE /api/admin/users/:id
func (h *AuthHandler) DeleteUser(c *gin.Context) {
	id := c.Param("id")

	result, err := h.db.Exec(`DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "database error")
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		respondError(c, http.StatusNotFound, "user not found")
		return
	}

	respondOK(c, gin.H{"deleted": true}, nil)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// isUniqueViolation returns true if err is a PostgreSQL unique constraint violation.
func isUniqueViolation(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "unique constraint") || strings.Contains(err.Error(), "duplicate key"))
}
