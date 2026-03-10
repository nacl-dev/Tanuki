package models

import "encoding/json"

// Performer represents a person (artist, voice actor, etc.) who appears in media.
type Performer struct {
	ID        string           `db:"id"         json:"id"`
	Name      string           `db:"name"       json:"name"`
	ImagePath string           `db:"image_path" json:"image_path"`
	Metadata  *json.RawMessage `db:"metadata"   json:"metadata,omitempty"`
}
