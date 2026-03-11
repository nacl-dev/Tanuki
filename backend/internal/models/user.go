// Package models defines the data structures used across Tanuki.
package models

import (
	"time"

	"github.com/lib/pq"
)

// UserRole represents the role of a user in the system.
type UserRole string

const (
	RoleAdmin UserRole = "admin"
	RoleUser  UserRole = "user"
)

// User represents a Tanuki user account.
type User struct {
	ID                         string         `db:"id"                            json:"id"`
	Username                   string         `db:"username"                      json:"username"`
	Email                      string         `db:"email"                         json:"email"`
	PasswordHash               string         `db:"password_hash"                 json:"-"`
	DisplayName                string         `db:"display_name"                  json:"display_name"`
	Role                       UserRole       `db:"role"                          json:"role"`
	IsActive                   bool           `db:"is_active"                     json:"is_active"`
	LibraryPinnedCollectionIDs pq.StringArray `db:"library_pinned_collection_ids" json:"library_pinned_collection_ids"`
	CreatedAt                  time.Time      `db:"created_at"                    json:"created_at"`
	UpdatedAt                  time.Time      `db:"updated_at"                    json:"updated_at"`
}
