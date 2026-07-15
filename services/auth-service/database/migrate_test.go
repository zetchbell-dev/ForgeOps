package database

import (
	"regexp"
	"testing"
)

// These tests exercise loadMigrations() directly against the real
// embedded migrations/*.sql files (migrationFiles, migrate.go). They
// deliberately never touch Up/Down/appliedVersions — those require a
// *pgxpool.Pool (a live Postgres connection) and are integration-level
// concerns out of scope here. loadMigrations itself has no database
// dependency at all: it only reads the embed.FS, which makes it fully
// unit-testable without a real Postgres instance.
//
// NOTE on "duplicate migration detection": loadMigrations (migrate.go)
// does not implement any duplicate-version detection — two files sharing
// the same numeric prefix would both be loaded and merely sorted next to
// each other by sort.Slice, with no error raised. Since this test suite
// must not modify production code, and the real migrations/ directory
// (checked below) contains exactly one migration, there is no way to
// exercise a "two files, same version" scenario against loadMigrations
// as written without either modifying migrate.go to add detection or
// injecting a second embed.FS the function doesn't accept. TestLoadMigrations_VersionsAreUnique
// below documents and locks in the actual current behavior (versions
// happen to be unique today) rather than asserting a duplicate-rejection
// behavior that doesn't exist in the code under test.

func TestLoadMigrations_NoError(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations() returned error: %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("loadMigrations() returned zero migrations, want at least one embedded migration")
	}
}

func TestLoadMigrations_EmbeddedMigrationsExist(t *testing.T) {
	// migrations/0001_init.up.sql and its .down.sql counterpart are
	// checked into the repo (database/migrations/), so loadMigrations
	// must surface at least that one pair.
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations() returned error: %v", err)
	}

	found := false
	for _, m := range migrations {
		if m.version == 1 && m.name == "0001_init.up.sql" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected migration version=1 name=0001_init.up.sql among loaded migrations, got %+v", migrations)
	}
}

func TestLoadMigrations_Ordering(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations() returned error: %v", err)
	}

	for i := 1; i < len(migrations); i++ {
		if migrations[i-1].version > migrations[i].version {
			t.Fatalf("migrations not sorted ascending by version: index %d (version %d) comes after index %d (version %d)",
				i, migrations[i].version, i-1, migrations[i-1].version)
		}
	}
}

func TestLoadMigrations_VersionsAreUnique(t *testing.T) {
	// Documents current behavior: loadMigrations performs no dedup pass
	// of its own (see the package-level NOTE above), so this only holds
	// because the real migrations/ directory happens not to contain a
	// version collision today. A future PR adding a colliding pair
	// would silently pass through loadMigrations and only be caught by
	// this test failing.
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations() returned error: %v", err)
	}

	seen := map[int64]string{}
	for _, m := range migrations {
		if prior, ok := seen[m.version]; ok {
			t.Fatalf("duplicate migration version %d: %q and %q", m.version, prior, m.name)
		}
		seen[m.version] = m.name
	}
}

func TestLoadMigrations_ParsingAndFieldPopulation(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations() returned error: %v", err)
	}

	for _, m := range migrations {
		if m.version <= 0 {
			t.Errorf("migration %q has non-positive version %d", m.name, m.version)
		}
		if m.name == "" {
			t.Error("migration has empty name")
		}
		if m.upSQL == "" {
			t.Errorf("migration %q has empty upSQL", m.name)
		}
		if m.downSQL == "" {
			t.Errorf("migration %q has empty downSQL", m.name)
		}
	}
}

func TestLoadMigrations_VersionExtraction(t *testing.T) {
	// upFilePattern (migrate.go) is what extracts the numeric version
	// prefix from a filename; exercise it directly against representative
	// filenames rather than only indirectly through loadMigrations, so a
	// change to the pattern's capture semantics is caught even if the
	// embedded directory only ever has one file.
	tests := []struct {
		filename    string
		wantMatch   bool
		wantVersion string
	}{
		{"0001_init.up.sql", true, "0001"},
		{"0002_add_index.up.sql", true, "0002"},
		{"0042_widgets.up.sql", true, "0042"},
		{"0001_init.down.sql", false, ""}, // .down.sql is never an "up" match
		{"init.up.sql", false, ""},        // missing numeric prefix
		{"0001-init.up.sql", false, ""},   // hyphen instead of underscore
		{"0001_init.sql", false, ""},      // missing .up. segment
		{"readme.md", false, ""},
	}

	for _, tt := range tests {
		matches := upFilePattern.FindStringSubmatch(tt.filename)
		gotMatch := matches != nil
		if gotMatch != tt.wantMatch {
			t.Errorf("upFilePattern.FindStringSubmatch(%q): match = %v, want %v", tt.filename, gotMatch, tt.wantMatch)
			continue
		}
		if tt.wantMatch && matches[1] != tt.wantVersion {
			t.Errorf("upFilePattern.FindStringSubmatch(%q): version = %q, want %q", tt.filename, matches[1], tt.wantVersion)
		}
	}
}

func TestLoadMigrations_FilenamesAreValid(t *testing.T) {
	// Every loaded migration's name must actually match upFilePattern —
	// loadMigrations only appends entries that already passed that check,
	// so this re-verifies the invariant holds for whatever is currently
	// embedded rather than assuming it.
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations() returned error: %v", err)
	}

	validName := regexp.MustCompile(`^\d+_.+\.up\.sql$`)
	for _, m := range migrations {
		if !validName.MatchString(m.name) {
			t.Errorf("migration name %q does not match expected up-migration filename shape", m.name)
		}
	}
}
