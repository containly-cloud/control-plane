// Package identity implements local credentials used by the Control Plane.
package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

var ErrWeakPassword = errors.New("password must contain at least 12 characters")
var ErrRootAlreadyExists = errors.New("root user is already configured")
var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrUnauthenticated = errors.New("unauthenticated")
var ErrPasswordChangeRequired = errors.New("password change required")

type User struct {
	ID                int64    `json:"id"`
	Username          string   `json:"username"`
	Role              string   `json:"role"`
	Permissions       []string `json:"permissions"`
	Active            bool     `json:"active"`
	PasswordTemporary bool     `json:"passwordTemporary"`
}

type StoredCredential struct {
	User
	PasswordHash string
}

type Session struct {
	Token     string
	User      User
	ExpiresAt time.Time
}

// SessionMetadata describes the client that established an authenticated
// session. It intentionally contains no credential or token material.
type SessionMetadata struct {
	IPAddress string
	UserAgent string
}

// ActiveSession is safe to expose to the owner of the session list.
type ActiveSession struct {
	IPAddress  string    `json:"ipAddress"`
	UserAgent  string    `json:"userAgent"`
	CreatedAt  time.Time `json:"createdAt"`
	LastSeenAt time.Time `json:"lastSeenAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

type Principal struct {
	Username               string   `json:"username"`
	Permissions            []string `json:"permissions"`
	PasswordChangeRequired bool     `json:"passwordChangeRequired"`
}

const (
	PermissionWorkspaceRead     = "workspace:read"
	PermissionUsersManage       = "users:manage"
	PermissionPermissionsRead   = "permissions:read"
	PermissionPermissionsManage = "permissions:manage"
	PermissionSettingsManage    = "settings:manage"
	PermissionBackupsRead       = "backups:read"
	PermissionBackupsManage     = "backups:manage"
	PermissionCommandsExecute   = "commands:execute"
)

// AllPermissions is the server-owned allowlist used when assigning access.
var AllPermissions = []string{
	PermissionWorkspaceRead,
	PermissionUsersManage,
	PermissionPermissionsRead,
	PermissionPermissionsManage,
	PermissionSettingsManage,
	PermissionBackupsRead,
	PermissionBackupsManage,
	PermissionCommandsExecute,
}

// PermissionScope is the server-owned grouping of permissions. The client uses
// this metadata for presentation only; the server remains the source of truth
// for which permissions grant read or write access.
type PermissionScope struct {
	Scope string   `json:"scope"`
	Read  []string `json:"read"`
	Write []string `json:"write"`
}

var permissionScopes = []PermissionScope{
	{Scope: "workspace", Read: []string{PermissionWorkspaceRead}},
	{Scope: "users", Write: []string{PermissionUsersManage}},
	{Scope: "permissions", Read: []string{PermissionPermissionsRead}, Write: []string{PermissionPermissionsManage}},
	{Scope: "settings", Write: []string{PermissionSettingsManage}},
	{Scope: "backups", Read: []string{PermissionBackupsRead}, Write: []string{PermissionBackupsManage}},
	{Scope: "commands", Write: []string{PermissionCommandsExecute}},
}

// PermissionScopes returns a defensive copy of the permission catalog exposed
// to permission-management clients.
func PermissionScopes() []PermissionScope {
	result := make([]PermissionScope, len(permissionScopes))
	for index, scope := range permissionScopes {
		result[index] = PermissionScope{
			Scope: scope.Scope,
			Read:  append([]string{}, scope.Read...),
			Write: append([]string{}, scope.Write...),
		}
	}
	return result
}

// ValidPermissions confirms that a request contains only known permissions
// and does not repeat a permission.
func ValidPermissions(permissions []string) bool {
	known := make(map[string]struct{}, len(AllPermissions))
	for _, permission := range AllPermissions {
		known[permission] = struct{}{}
	}
	seen := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		if _, ok := known[permission]; !ok {
			return false
		}
		if _, duplicate := seen[permission]; duplicate {
			return false
		}
		seen[permission] = struct{}{}
	}
	return true
}

// RootRepository is intentionally small so application handlers depend on an
// interface rather than SQLite directly.
type RootRepository interface {
	CreateRoot(ctx context.Context, username, passwordHash string) error
}

type SessionRepository interface {
	FindCredential(ctx context.Context, username string) (StoredCredential, error)
	CreateSession(ctx context.Context, id, tokenHash string, userID int64, expiresAt time.Time, metadata SessionMetadata) error
	FindSessionUser(ctx context.Context, tokenHash string, now time.Time) (User, error)
	DeleteSession(ctx context.Context, tokenHash string) error
}

// RootCreator hashes credentials before delegating persistence. Passwords are
// deliberately one-way: this release has no recovery or reset mechanism.
type RootCreator struct {
	repository RootRepository
}

func NewRootCreator(repository RootRepository) *RootCreator {
	return &RootCreator{repository: repository}
}

func (c *RootCreator) Create(ctx context.Context, username, password string) error {
	username = strings.TrimSpace(username)
	if len(username) < 3 || len(username) > 64 {
		return errors.New("username must contain between 3 and 64 characters")
	}
	for _, character := range username {
		if !(character >= 'a' && character <= 'z') && !(character >= '0' && character <= '9') && character != '-' && character != '_' && character != '.' {
			return errors.New("username may only use lowercase letters, numbers, hyphens, underscores, and dots")
		}
	}
	if len(password) < 12 {
		return ErrWeakPassword
	}
	if len(password) > 128 {
		return errors.New("password must contain at most 128 characters")
	}
	if strings.TrimSpace(password) == "" {
		return ErrWeakPassword
	}

	hash, err := hashPassword(password)
	if err != nil {
		return fmt.Errorf("hash root password: %w", err)
	}
	return c.repository.CreateRoot(ctx, username, hash)
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	// OWASP-aligned interactive parameters. The encoded format carries every
	// parameter needed to verify old credentials after a future upgrade; it is
	// not an encrypted password and cannot be used to recover the original.
	const memory uint32 = 64 * 1024
	const iterations uint32 = 3
	const parallelism uint8 = 2
	const keyLength uint32 = 32
	hash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)
	encoded := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", memory, iterations, parallelism,
		encoded.EncodeToString(salt), encoded.EncodeToString(hash)), nil
}

// NewTemporaryPassword creates a one-time credential shown only to the admin.
func NewTemporaryPassword() (string, error) {
	value := make([]byte, 18)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func HashPassword(password string) (string, error) { return hashPassword(password) }

// Authenticator verifies one-way password hashes and manages opaque sessions.
type Authenticator struct {
	repository SessionRepository
	now        func() time.Time
}

func NewAuthenticator(repository SessionRepository) *Authenticator {
	return &Authenticator{repository: repository, now: time.Now}
}

func (a *Authenticator) Login(ctx context.Context, username, password string, metadata SessionMetadata) (Session, error) {
	credential, err := a.repository.FindCredential(ctx, strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return Session{}, ErrInvalidCredentials
		}
		return Session{}, fmt.Errorf("find credential: %w", err)
	}
	if !verifyPassword(password, credential.PasswordHash) {
		return Session{}, ErrInvalidCredentials
	}

	token, err := randomToken()
	if err != nil {
		return Session{}, fmt.Errorf("generate session token: %w", err)
	}
	now := a.now().UTC()
	session := Session{
		Token:     token,
		User:      credential.User,
		ExpiresAt: now.Add(24 * time.Hour),
	}
	if err := a.repository.CreateSession(ctx, tokenChecksum(token), tokenChecksum(token), session.User.ID, session.ExpiresAt, metadata); err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}
	return session, nil
}

func (a *Authenticator) Session(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrUnauthenticated
	}
	user, err := a.repository.FindSessionUser(ctx, tokenChecksum(token), a.now().UTC())
	if err != nil {
		if errors.Is(err, ErrUnauthenticated) {
			return User{}, ErrUnauthenticated
		}
		return User{}, fmt.Errorf("find session: %w", err)
	}
	return user, nil
}

func (a *Authenticator) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if err := a.repository.DeleteSession(ctx, tokenChecksum(token)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// PrincipalFor is the only role-to-permission policy used by the HTTP layer.
// Clients receive these effective permissions, never a role decision to make.
func PrincipalFor(user User) Principal {
	permissions := []string{PermissionWorkspaceRead}
	if user.Role == "root" {
		permissions = append([]string(nil), AllPermissions...)
	} else {
		permissions = append(permissions, user.Permissions...)
	}
	return Principal{Username: user.Username, Permissions: permissions, PasswordChangeRequired: user.PasswordTemporary}
}

// HasPermission keeps authorization policy on the server. Frontends only use
// the effective permissions returned by authenticated endpoints for display.
func HasPermission(user User, permission string) bool {
	for _, candidate := range PrincipalFor(user).Permissions {
		if candidate == permission {
			return true
		}
	}
	return false
}

func verifyPassword(password, encodedHash string) bool {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) == 0 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func tokenChecksum(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
