// Package storage owns SQLite persistence and its versioned schema lifecycle.
package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"api/internal/config"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Store is the application's SQLite connection and migration manager.
type Store struct {
	db              *sql.DB
	paths           config.Paths
	logger          *slog.Logger
	databaseExisted bool
}

// Open prepares the secure local data directory and opens a single SQLite
// connection. A single connection avoids unnecessary writer contention for a
// local control-plane process while WAL keeps reads responsive.
func Open(ctx context.Context, paths config.Paths, logger *slog.Logger) (*Store, error) {
	if err := paths.EnsureDirectories(); err != nil {
		return nil, err
	}

	_, err := os.Stat(paths.Database)
	databaseExisted := err == nil
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("inspect database: %w", err)
	}

	databaseURL := "file:" + filepath.ToSlash(paths.Database) + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	pingContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingContext); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	if err := os.Chmod(paths.Database, 0o600); err != nil {
		db.Close()
		return nil, fmt.Errorf("secure database file: %w", err)
	}

	return &Store{db: db, paths: paths, logger: logger, databaseExisted: databaseExisted}, nil
}

// Close releases the local database file.
func (s *Store) Close() error {
	return s.db.Close()
}

// Migrate verifies already-applied migrations and applies only versions that
// are not recorded. Before a schema update to an existing database it writes a
// consistent SQLite backup using VACUUM INTO.
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			checksum TEXT NOT NULL,
			applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		return fmt.Errorf("create migration metadata: %w", err)
	}

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	applied, err := s.appliedMigrations(ctx)
	if err != nil {
		return err
	}

	pending := make([]migration, 0)
	for _, migration := range migrations {
		if recordedChecksum, ok := applied[migration.version]; ok {
			if recordedChecksum != migration.checksum {
				return fmt.Errorf("migration %03d has changed after being applied", migration.version)
			}
			continue
		}
		pending = append(pending, migration)
	}

	if len(pending) == 0 {
		return nil
	}
	if s.databaseExisted {
		backupPath, err := s.Backup(ctx)
		if err != nil {
			return fmt.Errorf("backup before migration: %w", err)
		}
		s.logger.Info("database backup created before migration", "path", backupPath)
	}

	for _, migration := range pending {
		transaction, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %03d: %w", migration.version, err)
		}
		if _, err := transaction.ExecContext(ctx, migration.sql); err != nil {
			transaction.Rollback()
			return fmt.Errorf("apply migration %03d: %w", migration.version, err)
		}
		if _, err := transaction.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, name, checksum) VALUES (?, ?, ?)`,
			migration.version, migration.name, migration.checksum,
		); err != nil {
			transaction.Rollback()
			return fmt.Errorf("record migration %03d: %w", migration.version, err)
		}
		if err := transaction.Commit(); err != nil {
			return fmt.Errorf("commit migration %03d: %w", migration.version, err)
		}
		s.logger.Info("database migration applied", "version", migration.version, "name", migration.name)
	}

	return nil
}

// Backup writes a transactionally consistent, standalone SQLite snapshot. It
// must be used instead of copying a live .sqlite3 file because WAL state may
// otherwise be left behind.
func (s *Store) Backup(ctx context.Context) (string, error) {
	filename := "control-plane-" + time.Now().UTC().Format("20060102T150405.000000000Z") + ".sqlite3"
	path := filepath.Join(s.paths.BackupDir, filename)
	quotedPath := "'" + strings.ReplaceAll(path, "'", "''") + "'"
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO "+quotedPath); err != nil {
		return "", err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", fmt.Errorf("secure backup: %w", err)
	}
	return path, nil
}

func (s *Store) appliedMigrations(ctx context.Context) (map[int]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT version, checksum FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read migration metadata: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]string)
	for rows.Next() {
		var version int
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, fmt.Errorf("scan migration metadata: %w", err)
		}
		applied[version] = checksum
	}
	return applied, rows.Err()
}

type migration struct {
	version  int
	name     string
	checksum string
	sql      string
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		parts := strings.SplitN(entry.Name(), "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("migration %q must start with a numeric version followed by _", entry.Name())
		}
		version, err := strconv.Atoi(parts[0])
		if err != nil || version < 1 {
			return nil, fmt.Errorf("invalid migration version in %q", entry.Name())
		}
		contents, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}
		migrations = append(migrations, migration{
			version:  version,
			name:     entry.Name(),
			checksum: checksum(contents),
			sql:      string(contents),
		})
	}

	sort.Slice(migrations, func(i, j int) bool { return migrations[i].version < migrations[j].version })
	for index := 1; index < len(migrations); index++ {
		if migrations[index-1].version == migrations[index].version {
			return nil, fmt.Errorf("duplicate migration version %03d", migrations[index].version)
		}
	}
	return migrations, nil
}
