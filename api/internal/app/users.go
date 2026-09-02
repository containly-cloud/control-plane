package app

import (
	"api/internal/identity"
	"api/internal/localization"
	"api/internal/storage"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

type ControlUserManager interface {
	ListControlUsers(context.Context) ([]storage.ControlUser, error)
	CreateControlUser(context.Context, string, string, []string) (storage.ControlUser, error)
	SetControlUserActive(context.Context, int64, bool) error
	SetControlUserPermissions(context.Context, int64, []string) error
	ResetControlUserPassword(context.Context, int64, string) error
	DeleteControlUser(context.Context, int64) error
}
type usersHandler struct {
	authenticator Authenticator
	users         ControlUserManager
}

func (h usersHandler) list(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r, identity.PermissionUsersManage, identity.PermissionPermissionsRead, identity.PermissionPermissionsManage) {
		return
	}
	u, e := h.users.ListControlUsers(r.Context())
	if e != nil {
		writeError(w, r, 500, localization.UsersUnavailable)
		return
	}
	writeJSON(w, 200, map[string]any{"users": u, "permissionScopes": identity.PermissionScopes()})
}
func (h usersHandler) create(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r, identity.PermissionUsersManage) {
		return
	}
	defer r.Body.Close()
	var in struct {
		Username    string   `json:"username"`
		Permissions []string `json:"permissions"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeError(w, r, 400, localization.InvalidUserInput)
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	if len(in.Username) < 3 || !identity.ValidPermissions(in.Permissions) {
		writeError(w, r, 422, localization.InvalidUserInput)
		return
	}
	temp, e := identity.NewTemporaryPassword()
	if e != nil {
		writeError(w, r, 500, localization.UsersUnavailable)
		return
	}
	hash, e := identity.HashPassword(temp)
	if e != nil {
		writeError(w, r, 500, localization.UsersUnavailable)
		return
	}
	u, e := h.users.CreateControlUser(r.Context(), in.Username, hash, in.Permissions)
	if e != nil {
		writeError(w, r, 422, localization.UserCreationFailed)
		return
	}
	writeJSON(w, 201, map[string]any{"user": u, "temporaryPassword": temp})
}
func (h usersHandler) permissions(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r, identity.PermissionPermissionsManage) {
		return
	}
	var in struct {
		ID          int64    `json:"id"`
		Permissions []string `json:"permissions"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || !identity.ValidPermissions(in.Permissions) {
		writeError(w, r, 400, localization.InvalidUserInput)
		return
	}
	if e := h.users.SetControlUserPermissions(r.Context(), in.ID, in.Permissions); e != nil {
		if errors.Is(e, storage.ErrRootPermissionsImmutable) {
			writeError(w, r, 403, localization.AccessDenied)
			return
		}
		writeError(w, r, 422, localization.UsersUnavailable)
		return
	}
	w.WriteHeader(204)
}
func (h usersHandler) resetPassword(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r, identity.PermissionUsersManage) {
		return
	}
	var in struct {
		ID int64 `json:"id"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.ID <= 0 {
		writeError(w, r, http.StatusBadRequest, localization.InvalidUserInput)
		return
	}
	temporaryPassword, err := identity.NewTemporaryPassword()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, localization.UsersUnavailable)
		return
	}
	passwordHash, err := identity.HashPassword(temporaryPassword)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, localization.UsersUnavailable)
		return
	}
	if err := h.users.ResetControlUserPassword(r.Context(), in.ID, passwordHash); err != nil {
		if errors.Is(err, storage.ErrRootPasswordResetForbidden) {
			writeError(w, r, http.StatusForbidden, localization.AccessDenied)
			return
		}
		writeError(w, r, http.StatusUnprocessableEntity, localization.UsersUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"temporaryPassword": temporaryPassword})
}
func (h usersHandler) remove(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r, identity.PermissionUsersManage) {
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, r, http.StatusBadRequest, localization.InvalidUserInput)
		return
	}
	if err := h.users.DeleteControlUser(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrRootUserDeletionForbidden) {
			writeError(w, r, http.StatusForbidden, localization.AccessDenied)
			return
		}
		writeError(w, r, http.StatusUnprocessableEntity, localization.UsersUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h usersHandler) authorize(w http.ResponseWriter, r *http.Request, permissions ...string) bool {
	u, e := h.authenticator.Session(r.Context(), sessionToken(r))
	if e != nil {
		if errors.Is(e, identity.ErrUnauthenticated) {
			writeError(w, r, 401, localization.Unauthenticated)
		} else {
			writeError(w, r, 500, localization.AuthenticationUnavailable)
		}
		return false
	}
	for _, permission := range permissions {
		if identity.HasPermission(u, permission) {
			return true
		}
	}
	writeError(w, r, 403, localization.AccessDenied)
	return false
}
