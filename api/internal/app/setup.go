package app

import (
	"encoding/json"
	"errors"
	"net/http"

	"api/internal/identity"
	"api/internal/localization"
)

type setupHandler struct {
	rootCreator RootCreator
	setupStatus SetupStatus
}

func (h setupHandler) status(writer http.ResponseWriter, request *http.Request) {
	configured, err := h.setupStatus.HasRoot(request.Context())
	if err != nil {
		writeError(writer, request, http.StatusInternalServerError, localization.InitialSetupUnavailable)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]bool{"configured": configured})
}

func (h setupHandler) createRoot(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()
	request.Body = http.MaxBytesReader(writer, request.Body, 16<<10)
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(writer, request, http.StatusBadRequest, localization.InvalidSetupInput)
		return
	}

	if err := h.rootCreator.Create(request.Context(), input.Username, input.Password); err != nil {
		switch {
		case errors.Is(err, identity.ErrRootAlreadyExists):
			writeError(writer, request, http.StatusConflict, localization.InitialSetupComplete)
		case errors.Is(err, identity.ErrWeakPassword):
			writeError(writer, request, http.StatusUnprocessableEntity, localization.WeakPassword, map[string]string{"min": "12"})
		default:
			writeError(writer, request, http.StatusUnprocessableEntity, localization.AccountCreationFailed)
		}
		return
	}

	writeMessage(writer, request, http.StatusCreated, localization.AccountCreated)
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeMessage(writer http.ResponseWriter, request *http.Request, status int, key localization.Key) {
	locale := localization.FromAcceptLanguage(request.Header.Get("Accept-Language"))
	writer.Header().Set("Content-Language", string(locale))
	writeJSON(writer, status, map[string]string{"message": localization.Message(locale, key, nil)})
}

func writeError(writer http.ResponseWriter, request *http.Request, status int, key localization.Key, parameters ...map[string]string) {
	locale := localization.FromAcceptLanguage(request.Header.Get("Accept-Language"))
	values := map[string]string(nil)
	if len(parameters) > 0 {
		values = parameters[0]
	}
	writer.Header().Set("Content-Language", string(locale))
	writeJSON(writer, status, map[string]string{
		"code":  string(key),
		"error": localization.Message(locale, key, values),
	})
}
