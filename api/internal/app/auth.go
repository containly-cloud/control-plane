package app

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"api/internal/identity"
	"api/internal/localization"
)

type authHandler struct {
	authenticator Authenticator
}

func (h authHandler) login(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()
	request.Body = http.MaxBytesReader(writer, request.Body, 16<<10)
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(writer, request, http.StatusBadRequest, localization.InvalidLoginInput)
		return
	}

	session, err := h.authenticator.Login(request.Context(), input.Username, input.Password, requestSessionMetadata(request))
	if err != nil {
		if errors.Is(err, identity.ErrInvalidCredentials) {
			writeError(writer, request, http.StatusUnauthorized, localization.InvalidCredentials)
			return
		}
		writeError(writer, request, http.StatusInternalServerError, localization.AuthenticationUnavailable)
		return
	}

	http.SetCookie(writer, sessionCookie(request, session.Token, session.ExpiresAt))
	writeJSON(writer, http.StatusOK, map[string]any{"user": identity.PrincipalFor(session.User)})
}

func (h authHandler) session(writer http.ResponseWriter, request *http.Request) {
	user, err := h.authenticator.Session(request.Context(), sessionToken(request))
	if err != nil {
		if errors.Is(err, identity.ErrUnauthenticated) {
			writeError(writer, request, http.StatusUnauthorized, localization.Unauthenticated)
			return
		}
		writeError(writer, request, http.StatusInternalServerError, localization.AuthenticationUnavailable)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"user": identity.PrincipalFor(user)})
}

func (h authHandler) logout(writer http.ResponseWriter, request *http.Request) {
	if err := h.authenticator.Logout(request.Context(), sessionToken(request)); err != nil {
		writeError(writer, request, http.StatusInternalServerError, localization.AuthenticationUnavailable)
		return
	}
	http.SetCookie(writer, sessionCookie(request, "", time.Unix(0, 0)))
	writer.WriteHeader(http.StatusNoContent)
}

func sessionToken(request *http.Request) string {
	cookie, err := request.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func requestSessionMetadata(request *http.Request) identity.SessionMetadata {
	address, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		address = request.RemoteAddr
	}
	if net.ParseIP(address) == nil {
		address = ""
	}
	userAgent := strings.TrimSpace(request.UserAgent())
	if len(userAgent) > 256 {
		userAgent = userAgent[:256]
	}
	return identity.SessionMetadata{IPAddress: address, UserAgent: userAgent}
}
