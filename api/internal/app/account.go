package app

import (
	"errors"
	"net/http"
	"time"

	"api/internal/identity"
	"api/internal/localization"
)

type accountHandler struct {
	authenticator Authenticator
	sessions      SessionInventory
}

func (h accountHandler) sessionsResponse(writer http.ResponseWriter, request *http.Request) {
	user, err := h.authenticator.Session(request.Context(), sessionToken(request))
	if err != nil {
		if errors.Is(err, identity.ErrUnauthenticated) {
			writeError(writer, request, http.StatusUnauthorized, localization.Unauthenticated)
			return
		}
		writeError(writer, request, http.StatusInternalServerError, localization.AuthenticationUnavailable)
		return
	}
	sessions, err := h.sessions.ListActiveSessions(request.Context(), user.ID, time.Now().UTC())
	if err != nil {
		writeError(writer, request, http.StatusInternalServerError, localization.AccountSessionsUnavailable)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"sessions": sessions})
}
