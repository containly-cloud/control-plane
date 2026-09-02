package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"api/internal/identity"
	"api/internal/storage"
	"github.com/go-chi/chi/v5"
)

type resetControlUsersStub struct {
	controlUsersStub
	err          error
	deleteErr    error
	passwordHash string
	resetCalls   int
	deleteCalls  int
}

func (s *resetControlUsersStub) ResetControlUserPassword(_ context.Context, _ int64, passwordHash string) error {
	s.resetCalls++
	s.passwordHash = passwordHash
	return s.err
}

func (s *resetControlUsersStub) DeleteControlUser(context.Context, int64) error {
	s.deleteCalls++
	return s.deleteErr
}

type userAuthenticatorStub struct {
	authenticatorStub
	user identity.User
}

func (s userAuthenticatorStub) Session(context.Context, string) (identity.User, error) {
	return s.user, nil
}

func TestResetPasswordRequiresUserManagementPermission(t *testing.T) {
	users := &resetControlUsersStub{}
	handler := usersHandler{
		authenticator: userAuthenticatorStub{user: identity.User{}},
		users:         users,
	}
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"id":2}`))
	response := httptest.NewRecorder()

	handler.resetPassword(response, request)

	if response.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if users.resetCalls != 0 {
		t.Errorf("ResetControlUserPassword calls = %d, want 0", users.resetCalls)
	}
}

func TestResetPasswordReturnsOneTimePasswordForAuthorizedUser(t *testing.T) {
	users := &resetControlUsersStub{}
	handler := usersHandler{
		authenticator: userAuthenticatorStub{user: identity.User{Permissions: []string{identity.PermissionUsersManage}}},
		users:         users,
	}
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"id":2}`))
	response := httptest.NewRecorder()

	handler.resetPassword(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body struct {
		TemporaryPassword string `json:"temporaryPassword"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.TemporaryPassword == "" || body.TemporaryPassword == users.passwordHash {
		t.Errorf("temporary password must be returned separately from its hash")
	}
	if users.resetCalls != 1 {
		t.Errorf("ResetControlUserPassword calls = %d, want 1", users.resetCalls)
	}
}

func TestResetPasswordRejectsRootTarget(t *testing.T) {
	users := &resetControlUsersStub{err: storage.ErrRootPasswordResetForbidden}
	handler := usersHandler{
		authenticator: userAuthenticatorStub{user: identity.User{Permissions: []string{identity.PermissionUsersManage}}},
		users:         users,
	}
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"id":1}`))
	response := httptest.NewRecorder()

	handler.resetPassword(response, request)

	if response.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestDeleteUserRequiresUserManagementPermission(t *testing.T) {
	users := &resetControlUsersStub{}
	handler := usersHandler{
		authenticator: userAuthenticatorStub{user: identity.User{}},
		users:         users,
	}
	request := requestWithUserID(http.MethodDelete, 2)
	response := httptest.NewRecorder()

	handler.remove(response, request)

	if response.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if users.deleteCalls != 0 {
		t.Errorf("DeleteControlUser calls = %d, want 0", users.deleteCalls)
	}
}

func TestDeleteUserRejectsRootTarget(t *testing.T) {
	users := &resetControlUsersStub{deleteErr: storage.ErrRootUserDeletionForbidden}
	handler := usersHandler{
		authenticator: userAuthenticatorStub{user: identity.User{Permissions: []string{identity.PermissionUsersManage}}},
		users:         users,
	}
	request := requestWithUserID(http.MethodDelete, 1)
	response := httptest.NewRecorder()

	handler.remove(response, request)

	if response.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestDeleteUserRemovesAuthorizedMember(t *testing.T) {
	users := &resetControlUsersStub{}
	handler := usersHandler{
		authenticator: userAuthenticatorStub{user: identity.User{Permissions: []string{identity.PermissionUsersManage}}},
		users:         users,
	}
	request := requestWithUserID(http.MethodDelete, 2)
	response := httptest.NewRecorder()

	handler.remove(response, request)

	if response.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if users.deleteCalls != 1 {
		t.Errorf("DeleteControlUser calls = %d, want 1", users.deleteCalls)
	}
}

func requestWithUserID(method string, id int64) *http.Request {
	request := httptest.NewRequest(method, "/", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", strconv.FormatInt(id, 10))
	return request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
}
