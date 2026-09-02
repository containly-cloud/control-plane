package identity

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type rootRepositoryStub struct {
	username string
	hash     string
	hashes   []string
	err      error
}

func (s *rootRepositoryStub) CreateRoot(_ context.Context, username, passwordHash string) error {
	s.username = username
	s.hash = passwordHash
	s.hashes = append(s.hashes, passwordHash)
	return s.err
}

func TestRootCreatorUsesANewSaltForEachPassword(t *testing.T) {
	repository := &rootRepositoryStub{}
	creator := NewRootCreator(repository)
	password := "a-long-and-unique-password"

	if err := creator.Create(context.Background(), "marina", password); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	if err := creator.Create(context.Background(), "marina", password); err != nil {
		t.Fatalf("second Create() error = %v", err)
	}
	if len(repository.hashes) != 2 || repository.hashes[0] == repository.hashes[1] {
		t.Fatal("Create() must produce distinct Argon2id hashes for the same password")
	}
}

func TestRootCreatorHashesPasswordBeforePersistence(t *testing.T) {
	repository := &rootRepositoryStub{}
	creator := NewRootCreator(repository)
	password := "a-long-and-unique-password"

	if err := creator.Create(context.Background(), "marina", password); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if repository.username != "marina" {
		t.Errorf("username = %q, want marina", repository.username)
	}
	if repository.hash == password || !strings.HasPrefix(repository.hash, "$argon2id$") {
		t.Errorf("password hash was not encoded with Argon2id: %q", repository.hash)
	}
}

func TestRootCreatorRejectsInvalidUsernameAndPassword(t *testing.T) {
	repository := &rootRepositoryStub{}
	creator := NewRootCreator(repository)

	if err := creator.Create(context.Background(), "Admin User", "a-long-and-unique-password"); err == nil {
		t.Fatal("Create() accepted an invalid username")
	}
	if err := creator.Create(context.Background(), "marina", "too-short"); !errors.Is(err, ErrWeakPassword) {
		t.Errorf("Create() error = %v, want ErrWeakPassword", err)
	}
}

func TestPermissionCatalogOnlyContainsKnownPermissions(t *testing.T) {
	for _, scope := range PermissionScopes() {
		if scope.Scope == "" {
			t.Fatal("permission scope must have a name")
		}
		if !ValidPermissions(append(scope.Read, scope.Write...)) {
			t.Errorf("scope %q contains an invalid permission", scope.Scope)
		}
	}
}

func TestValidPermissionsRejectsUnknownAndDuplicatePermissions(t *testing.T) {
	if ValidPermissions([]string{"unknown:permission"}) {
		t.Fatal("ValidPermissions accepted an unknown permission")
	}
	if ValidPermissions([]string{PermissionBackupsRead, PermissionBackupsRead}) {
		t.Fatal("ValidPermissions accepted a duplicate permission")
	}
}

func TestUserJSONUsesCamelCase(t *testing.T) {
	encoded, err := json.Marshal(User{
		ID:                1,
		Username:          "erik",
		Role:              "member",
		Permissions:       []string{PermissionWorkspaceRead},
		Active:            true,
		PasswordTemporary: true,
	})
	if err != nil {
		t.Fatalf("marshal user: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatalf("unmarshal user JSON: %v", err)
	}
	for _, field := range []string{"id", "username", "role", "permissions", "active", "passwordTemporary"} {
		if _, ok := result[field]; !ok {
			t.Errorf("JSON is missing %q: %s", field, encoded)
		}
	}
	if _, ok := result["PasswordTemporary"]; ok {
		t.Errorf("JSON must not expose PascalCase fields: %s", encoded)
	}
}
