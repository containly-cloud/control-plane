// Package config defines the local, user-owned data layout for Control Plane.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths describes files that must move together when a user backs up a local
// Containly installation. The default root deliberately follows the requested
// ~/.containly location instead of the operating-system-specific config path.
type Paths struct {
	Root       string
	ControlDir string
	DataDir    string
	BackupDir  string
	LogDir     string
	Database   string
}

// ResolvePaths returns the persisted-data layout. CONTAINLY_HOME is useful for
// portable installations and tests; by default it is ~/.containly.
func ResolvePaths() (Paths, error) {
	root := os.Getenv("CONTAINLY_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Paths{}, fmt.Errorf("resolve user home: %w", err)
		}
		root = filepath.Join(home, ".containly")
	}

	root, err := filepath.Abs(root)
	if err != nil {
		return Paths{}, fmt.Errorf("resolve Containly home: %w", err)
	}

	controlDir := filepath.Join(root, "control-plane")
	dataDir := filepath.Join(controlDir, "data")
	return Paths{
		Root:       root,
		ControlDir: controlDir,
		DataDir:    dataDir,
		BackupDir:  filepath.Join(controlDir, "backups"),
		LogDir:     filepath.Join(controlDir, "logs"),
		Database:   filepath.Join(dataDir, "control-plane.sqlite3"),
	}, nil
}

// EnsureDirectories creates only the directories owned by Control Plane and
// applies owner-only permissions where the platform allows it.
func (p Paths) EnsureDirectories() error {
	for _, directory := range []string{p.Root, p.ControlDir, p.DataDir, p.BackupDir, p.LogDir} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return fmt.Errorf("create %s: %w", directory, err)
		}
		if err := os.Chmod(directory, 0o700); err != nil {
			return fmt.Errorf("secure %s: %w", directory, err)
		}
	}
	return nil
}
