// Package migrations embeds the SQL files into the binary so the migrate
// command needs nothing on disk at runtime.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
