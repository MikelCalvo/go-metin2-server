//go:build sqlite_harness

package main

// Blank import for deliberately tagged lab/harness builds of metin2-migrate.
// Stock `go build ./cmd/metin2-migrate` stays free of a registered SQLite driver.
import _ "modernc.org/sqlite"
