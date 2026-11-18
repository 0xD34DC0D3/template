package migrations

import "embed"

// FS holds the goose migrations; the binary carries them so deploys need no
// loose SQL files.
//
//go:embed *.sql
var FS embed.FS
