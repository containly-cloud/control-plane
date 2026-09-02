package app

import (
	"context"
	"errors"
	"net/http"
	"time"

	"api/internal/identity"
	"api/internal/localization"
	"api/internal/system"
)

type systemHandler struct {
	authenticator Authenticator
	overview      SystemOverview
}

func (h systemHandler) overviewResponse(writer http.ResponseWriter, request *http.Request) {
	user, err := h.authenticator.Session(request.Context(), sessionToken(request))
	if err != nil {
		if errors.Is(err, identity.ErrUnauthenticated) {
			writeError(writer, request, http.StatusUnauthorized, localization.Unauthenticated)
			return
		}
		writeError(writer, request, http.StatusInternalServerError, localization.AuthenticationUnavailable)
		return
	}
	if !identity.HasPermission(user, identity.PermissionWorkspaceRead) {
		writeError(writer, request, http.StatusForbidden, localization.AccessDenied)
		return
	}

	overview, err := h.overview.Overview(request.Context())
	if err != nil {
		writeError(writer, request, http.StatusServiceUnavailable, localization.SystemOverviewUnavailable)
		return
	}
	writeJSON(writer, http.StatusOK, struct {
		System system.Overview `json:"system"`
	}{System: overview})
}

// SystemOverview is the live read model supplied by the low-overhead monitor.
type SystemOverview interface {
	Overview(ctx context.Context) (system.Overview, error)
}

// SessionInventory is the server-side device listing used for an authenticated
// account. The client never chooses which account can be read.
type SessionInventory interface {
	ListActiveSessions(ctx context.Context, userID int64, now time.Time) ([]identity.ActiveSession, error)
}
