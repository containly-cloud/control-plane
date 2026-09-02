package storage

import (
	"context"
	"fmt"

	"api/internal/identity"
)

// ErrRootAlreadyExists makes the setup endpoint idempotently safe: a second
// browser tab cannot replace the root credential after first setup.
var ErrRootAlreadyExists = identity.ErrRootAlreadyExists

// HasRoot reports whether first-time setup has already completed.
func (s *Store) HasRoot(ctx context.Context) (bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE role = 'root')`).Scan(&exists); err != nil {
		return false, fmt.Errorf("check root user: %w", err)
	}
	return exists, nil
}

// CreateRoot persists the one and only local superuser credential.
func (s *Store) CreateRoot(ctx context.Context, username, passwordHash string) error {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO users (username, role, password_hash)
		SELECT ?, 'root', ?
		WHERE NOT EXISTS (SELECT 1 FROM users WHERE role = 'root')`, username, passwordHash)
	if err != nil {
		return fmt.Errorf("create root user: %w", err)
	}
	created, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("confirm root user creation: %w", err)
	}
	if created != 1 {
		return ErrRootAlreadyExists
	}
	return nil
}
