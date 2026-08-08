// Package migration holds the goose SQL migrations embedded into the binaries.
package migration

import "embed"

// FS contains every goose migration file in this directory.
//
//go:embed *.sql
var FS embed.FS
