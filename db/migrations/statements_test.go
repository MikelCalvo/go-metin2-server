package migrations

import (
	"errors"
	"strings"
	"testing"
)

func TestSplitMigrationSQLStatementsPreservesQuotedSemicolons(t *testing.T) {
	body := "-- go-metin2 migration: 0002 accounts up\n" +
		"CREATE TABLE accounts (kind TEXT NOT NULL CHECK (kind IN ('talk;local', 'shop''keeper')));\n" +
		"CREATE INDEX accounts_kind_index ON accounts (kind);\n"

	statements, err := splitMigrationSQLStatements(body)
	if err != nil {
		t.Fatalf("split migration SQL statements: %v", err)
	}
	if len(statements) != 2 {
		t.Fatalf("expected 2 statements, got %#v", statements)
	}
	if !strings.Contains(statements[0], "'talk;local'") || !strings.Contains(statements[0], "'shop''keeper'") {
		t.Fatalf("expected first statement to preserve quoted semicolons and escaped quotes, got %q", statements[0])
	}
	if strings.Contains(statements[1], "talk;local") {
		t.Fatalf("quoted string leaked into second statement: %#v", statements)
	}
	if !strings.HasSuffix(statements[0], ");") || !strings.HasSuffix(statements[1], ";") {
		t.Fatalf("expected statement terminators to be preserved, got %#v", statements)
	}
}

func TestSplitMigrationSQLStatementsPreservesCommentSemicolons(t *testing.T) {
	body := "-- go-metin2 migration: 0002 accounts up; comment semicolon\n" +
		"CREATE TABLE accounts (login TEXT PRIMARY KEY); -- per-statement comment ;\n" +
		"/* block ; comment */\n" +
		"CREATE INDEX accounts_login_index ON accounts (login);\n"

	statements, err := splitMigrationSQLStatements(body)
	if err != nil {
		t.Fatalf("split migration SQL statements: %v", err)
	}
	if len(statements) != 2 {
		t.Fatalf("expected 2 statements, got %#v", statements)
	}
	if !strings.Contains(statements[0], "comment semicolon") || !strings.Contains(statements[0], "per-statement comment") {
		t.Fatalf("expected line comments to remain attached to first statement, got %#v", statements)
	}
	if !strings.Contains(statements[1], "block ; comment") {
		t.Fatalf("expected block comment before second statement to remain attached, got %#v", statements)
	}
}

func TestLoadCatalogRejectsMigrationSQLWithoutTerminatingSemicolon(t *testing.T) {
	migration := testMigration{
		version:  1,
		name:     "bootstrap_schema_migrations",
		upPath:   "0001_bootstrap_schema_migrations.up.sql",
		downPath: "0001_bootstrap_schema_migrations.down.sql",
		upSQL:    "-- go-metin2 migration: 0001 bootstrap_schema_migrations up\nCREATE TABLE schema_migrations (version INTEGER PRIMARY KEY)\n",
		downSQL:  "-- go-metin2 migration: 0001 bootstrap_schema_migrations down\nDROP TABLE schema_migrations;\n",
	}

	_, err := LoadCatalog(mapFSFor(migration))
	if !errors.Is(err, ErrInvalidCatalog) {
		t.Fatalf("expected ErrInvalidCatalog for unterminated SQL statement, got %v", err)
	}
	if !strings.Contains(err.Error(), "terminating semicolon") {
		t.Fatalf("expected terminating semicolon error, got %v", err)
	}
}

func TestLoadCatalogRejectsUnterminatedQuotedMigrationSQL(t *testing.T) {
	migration := testMigration{
		version:  1,
		name:     "bootstrap_schema_migrations",
		upPath:   "0001_bootstrap_schema_migrations.up.sql",
		downPath: "0001_bootstrap_schema_migrations.down.sql",
		upSQL:    "-- go-metin2 migration: 0001 bootstrap_schema_migrations up\nCREATE TABLE schema_migrations (name TEXT CHECK (name <> 'broken));\n",
		downSQL:  "-- go-metin2 migration: 0001 bootstrap_schema_migrations down\nDROP TABLE schema_migrations;\n",
	}

	_, err := LoadCatalog(mapFSFor(migration))
	if !errors.Is(err, ErrInvalidCatalog) {
		t.Fatalf("expected ErrInvalidCatalog for unterminated quoted SQL, got %v", err)
	}
	if !strings.Contains(err.Error(), "unterminated single-quoted string") {
		t.Fatalf("expected unterminated quote error, got %v", err)
	}
}
