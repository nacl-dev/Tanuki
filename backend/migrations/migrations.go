// Package migrations embeds the SQL migration files.
package migrations

import "embed"

// FS contains all SQL migration files.
//go:embed *.sql
var FS embed.FS
