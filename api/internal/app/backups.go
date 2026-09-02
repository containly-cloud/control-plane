package app

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"api/internal/identity"
	"api/internal/localization"
	"api/internal/storage"
)

type BackupManager interface {
	Backup(ctx context.Context) (string, error)
	ListBackups() ([]storage.BackupFile, error)
	DeleteBackup(name string) error
}

type backupHandler struct {
	authenticator Authenticator
	backups       BackupManager
}

func (h backupHandler) list(writer http.ResponseWriter, request *http.Request) {
	if !h.authorize(writer, request, identity.PermissionBackupsRead) { return }
	backups, err := h.backups.ListBackups()
	if err != nil { writeError(writer, request, http.StatusInternalServerError, localization.BackupsUnavailable); return }
	writeJSON(writer, http.StatusOK, map[string]any{"backups": backups})
}

func (h backupHandler) create(writer http.ResponseWriter, request *http.Request) {
	if !h.authorize(writer, request, identity.PermissionBackupsManage) { return }
	if _, err := h.backups.Backup(request.Context()); err != nil { writeError(writer, request, http.StatusInternalServerError, localization.BackupCreationFailed); return }
	writer.WriteHeader(http.StatusCreated)
}

func (h backupHandler) remove(writer http.ResponseWriter, request *http.Request) {
	if !h.authorize(writer, request, identity.PermissionBackupsManage) { return }
	if err := h.backups.DeleteBackup(strings.TrimSpace(request.URL.Query().Get("name"))); err != nil { writeError(writer, request, http.StatusUnprocessableEntity, localization.BackupDeletionFailed); return }
	writer.WriteHeader(http.StatusNoContent)
}

func (h backupHandler) authorize(writer http.ResponseWriter, request *http.Request, permission string) bool {
	user, err := h.authenticator.Session(request.Context(), sessionToken(request))
	if err != nil {
		if errors.Is(err, identity.ErrUnauthenticated) { writeError(writer, request, http.StatusUnauthorized, localization.Unauthenticated) } else { writeError(writer, request, http.StatusInternalServerError, localization.AuthenticationUnavailable) }
		return false
	}
	if !identity.HasPermission(user, permission) { writeError(writer, request, http.StatusForbidden, localization.AccessDenied); return false }
	return true
}
