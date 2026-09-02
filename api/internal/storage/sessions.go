package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"api/internal/identity"
)

func (s *Store) FindCredential(ctx context.Context, username string) (identity.StoredCredential, error) {
	var credential identity.StoredCredential
	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, role, password_hash, active, password_temporary
		FROM users
		WHERE username = ?`, username,
	).Scan(&credential.ID, &credential.Username, &credential.Role, &credential.PasswordHash, &credential.Active, &credential.PasswordTemporary)
	if errors.Is(err, sql.ErrNoRows) {
		return identity.StoredCredential{}, identity.ErrInvalidCredentials
	}
	if err != nil {
		return identity.StoredCredential{}, fmt.Errorf("find credential: %w", err)
	}
	if !credential.Active { return identity.StoredCredential{}, identity.ErrInvalidCredentials }
	permissions, err := s.userPermissions(ctx, credential.ID)
	if err != nil {
		return identity.StoredCredential{}, err
	}
	credential.Permissions = permissions
	return credential, nil
}

func (s *Store) CreateSession(ctx context.Context, id, tokenHash string, userID int64, expiresAt time.Time, metadata identity.SessionMetadata) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("remove expired sessions: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, expires_at, ip_address, user_agent, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, id, userID, tokenHash, expiresAt.UTC().Format(time.RFC3339Nano), metadata.IPAddress, metadata.UserAgent, time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("store session: %w", err)
	}
	return nil
}

func (s *Store) FindSessionUser(ctx context.Context, tokenHash string, now time.Time) (identity.User, error) {
	var user identity.User
	err := s.db.QueryRowContext(ctx, `
		SELECT users.id, users.username, users.role, users.active, users.password_temporary
		FROM sessions
		JOIN users ON users.id = sessions.user_id
		WHERE sessions.token_hash = ? AND sessions.expires_at > ?`,
		tokenHash, now.UTC().Format(time.RFC3339Nano),
	).Scan(&user.ID, &user.Username, &user.Role, &user.Active, &user.PasswordTemporary)
	if errors.Is(err, sql.ErrNoRows) {
		return identity.User{}, identity.ErrUnauthenticated
	}
	if err != nil {
		return identity.User{}, fmt.Errorf("find session user: %w", err)
	}
	permissions, err := s.userPermissions(ctx, user.ID)
	if err != nil {
		return identity.User{}, err
	}
	user.Permissions = permissions
	// Overview polling authenticates frequently. Recording activity at most once
	// every five minutes keeps the device list useful without turning reads into
	// a continuous SQLite write workload.
	if _, err := s.db.ExecContext(ctx, `
		UPDATE sessions
		SET last_seen_at = ?
		WHERE token_hash = ? AND last_seen_at <= ?`,
		now.UTC().Format(time.RFC3339Nano), tokenHash, now.Add(-5*time.Minute).UTC().Format(time.RFC3339Nano),
	); err != nil {
		return identity.User{}, fmt.Errorf("touch session: %w", err)
	}
	return user, nil
}

func (s *Store) userPermissions(ctx context.Context, userID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT permission FROM user_permissions WHERE user_id = ? ORDER BY permission`, userID)
	if err != nil {
		return nil, fmt.Errorf("read user permissions: %w", err)
	}
	defer rows.Close()
	permissions := make([]string, 0)
	for rows.Next() {
		var permission string
		if err := rows.Scan(&permission); err != nil {
			return nil, fmt.Errorf("scan user permission: %w", err)
		}
		permissions = append(permissions, permission)
	}
	return permissions, rows.Err()
}

// ListActiveSessions returns device metadata only for the requested account.
func (s *Store) ListActiveSessions(ctx context.Context, userID int64, now time.Time) ([]identity.ActiveSession, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ip_address, user_agent, created_at, last_seen_at, expires_at
		FROM sessions
		WHERE user_id = ? AND expires_at > ?
		ORDER BY last_seen_at DESC`, userID, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, fmt.Errorf("list active sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]identity.ActiveSession, 0)
	for rows.Next() {
		var session identity.ActiveSession
		var createdAt, lastSeenAt, expiresAt string
		if err := rows.Scan(&session.IPAddress, &session.UserAgent, &createdAt, &lastSeenAt, &expiresAt); err != nil {
			return nil, fmt.Errorf("scan active session: %w", err)
		}
		var err error
		if session.CreatedAt, err = parseSessionTime(createdAt); err != nil {
			return nil, fmt.Errorf("parse session creation time: %w", err)
		}
		if session.LastSeenAt, err = parseSessionTime(lastSeenAt); err != nil {
			return nil, fmt.Errorf("parse session last seen time: %w", err)
		}
		if session.ExpiresAt, err = parseSessionTime(expiresAt); err != nil {
			return nil, fmt.Errorf("parse session expiry time: %w", err)
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active sessions: %w", err)
	}
	return sessions, nil
}

func parseSessionTime(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid session time %q", value)
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}
