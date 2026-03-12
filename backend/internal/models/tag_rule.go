package models

import "time"

type TagAlias struct {
	ID        string    `db:"id"         json:"id"`
	AliasName string    `db:"alias_name" json:"alias_name"`
	TagID     string    `db:"tag_id"     json:"tag_id"`
	Tag       Tag       `db:"-"          json:"tag"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type TagImplication struct {
	ID           string    `db:"id"             json:"id"`
	TagID        string    `db:"tag_id"         json:"tag_id"`
	ImpliedTagID string    `db:"implied_tag_id" json:"implied_tag_id"`
	Tag          Tag       `db:"-"              json:"tag"`
	ImpliedTag   Tag       `db:"-"              json:"implied_tag"`
	CreatedAt    time.Time `db:"created_at"     json:"created_at"`
}
