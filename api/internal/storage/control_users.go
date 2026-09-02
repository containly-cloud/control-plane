package storage

import (
	"api/internal/identity"
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrRootPermissionsImmutable prevents the root account's effective access
// from being changed through the user-management API.
var ErrRootPermissionsImmutable = errors.New("root permissions cannot be changed")
var ErrRootPasswordResetForbidden = errors.New("root password cannot be reset")
var ErrRootUserDeletionForbidden = errors.New("root user cannot be deleted")

type ControlUser struct {
	identity.User
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Store) ListControlUsers(ctx context.Context) ([]ControlUser, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, username, role, active, password_temporary, created_at FROM users ORDER BY username`)
	if err != nil {
		return nil, fmt.Errorf("list control users: %w", err)
	}
	result := []ControlUser{}
	for rows.Next() {
		var u ControlUser
		var created string
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.Active, &u.PasswordTemporary, &created); err != nil {
			rows.Close()
			return nil, err
		}
		u.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		result = append(result, u)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for index := range result {
		if result[index].Role == "root" {
			result[index].Permissions = append([]string(nil), identity.AllPermissions...)
			continue
		}
		p, err := s.userPermissions(ctx, result[index].ID)
		if err != nil {
			return nil, err
		}
		result[index].Permissions = p
	}
	return result, nil
}

func (s *Store) CreateControlUser(ctx context.Context, username, passwordHash string, permissions []string) (ControlUser, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ControlUser{}, err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `INSERT INTO users (username,role,password_hash,password_temporary) VALUES (?,'member',?,1)`, username, passwordHash)
	if err != nil {
		return ControlUser{}, fmt.Errorf("create control user: %w", err)
	}
	id, _ := r.LastInsertId()
	for _, p := range permissions {
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_permissions(user_id,permission) VALUES (?,?)`, id, p); err != nil {
			return ControlUser{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ControlUser{}, err
	}
	users, err := s.ListControlUsers(ctx)
	if err != nil {
		return ControlUser{}, err
	}
	for _, u := range users {
		if u.ID == id {
			return u, nil
		}
	}
	return ControlUser{}, fmt.Errorf("created user missing")
}

func (s *Store) SetControlUserActive(ctx context.Context, id int64, active bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET active=? WHERE id=? AND role!='root'`, active, id)
	return err
}
func (s *Store) SetControlUserPermissions(ctx context.Context, id int64, permissions []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var role string
	if err := tx.QueryRowContext(ctx, `SELECT role FROM users WHERE id=?`, id).Scan(&role); err != nil {
		return err
	}
	if role == "root" {
		return ErrRootPermissionsImmutable
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM user_permissions WHERE user_id=?`, id); err != nil {
		return err
	}
	for _, permission := range permissions {
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_permissions(user_id,permission) VALUES (?,?)`, id, permission); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ResetControlUserPassword replaces a member's credential with a temporary
// password hash. Root credentials are deliberately excluded from this path.
func (s *Store) ResetControlUserPassword(ctx context.Context, id int64, passwordHash string) error {
	var role string
	if err := s.db.QueryRowContext(ctx, `SELECT role FROM users WHERE id=?`, id).Scan(&role); err != nil {
		return err
	}
	if role == "root" {
		return ErrRootPasswordResetForbidden
	}
	_, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash=?,password_temporary=1 WHERE id=?`, passwordHash, id)
	return err
}

// DeleteControlUser removes a non-root account and its dependent permissions
// and sessions. The database cascade performs the dependent cleanup.
func (s *Store) DeleteControlUser(ctx context.Context, id int64) error {
	var role string
	if err := s.db.QueryRowContext(ctx, `SELECT role FROM users WHERE id=?`, id).Scan(&role); err != nil {
		return err
	}
	if role == "root" {
		return ErrRootUserDeletionForbidden
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id=?`, id)
	return err
}
