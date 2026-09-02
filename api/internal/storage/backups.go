package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BackupFile describes a user-owned, standalone SQLite backup.
type BackupFile struct {
	Name      string    `json:"name"`
	SizeBytes int64     `json:"sizeBytes"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Store) ListBackups() ([]BackupFile, error) {
	entries, err := os.ReadDir(s.paths.BackupDir)
	if err != nil {
		return nil, fmt.Errorf("read backups: %w", err)
	}
	backups := make([]BackupFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sqlite3") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("read backup metadata: %w", err)
		}
		backups = append(backups, BackupFile{Name: entry.Name(), SizeBytes: info.Size(), CreatedAt: info.ModTime().UTC()})
	}
	sort.Slice(backups, func(i, j int) bool { return backups[i].CreatedAt.After(backups[j].CreatedAt) })
	return backups, nil
}

func (s *Store) DeleteBackup(name string) error {
	if filepath.Base(name) != name || !strings.HasPrefix(name, "control-plane-") || !strings.HasSuffix(name, ".sqlite3") {
		return errors.New("invalid backup name")
	}
	if err := os.Remove(filepath.Join(s.paths.BackupDir, name)); err != nil {
		return fmt.Errorf("delete backup: %w", err)
	}
	return nil
}
