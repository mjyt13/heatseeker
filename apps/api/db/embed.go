// Package db exposes the embedded SQL migrations so the binary can apply them
// without shipping the files separately.
package db

import "embed"

// Migrations contains goose SQL migration files.
//
//go:embed migrations/*.sql
var Migrations embed.FS
