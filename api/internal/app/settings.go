package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"api/internal/identity"
	"api/internal/localization"
	"api/internal/system"
)

type MonitoringSettingsStore interface {
	GetSystemMonitoringSettings(context.Context) (system.MonitoringSettings, error)
	SetSystemMonitoringSettings(context.Context, system.MonitoringSettings) error
	SystemMetricsStorageBytes(context.Context) (uint64, error)
	ClearSystemMetrics(context.Context) error
}

type MonitoringController interface {
	Configure(system.MonitoringSettings)
}

type settingsHandler struct {
	authenticator Authenticator
	settings      MonitoringSettingsStore
	monitor       MonitoringController
}

func (h settingsHandler) monitoring(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	settings, err := h.settings.GetSystemMonitoringSettings(r.Context())
	if err != nil {
		writeError(w, r, http.StatusServiceUnavailable, localization.SettingsUnavailable)
		return
	}
	h.writeMonitoringResponse(w, r, settings)
}

func (h settingsHandler) updateMonitoring(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	defer r.Body.Close()
	var input struct {
		Enabled           bool `json:"enabled"`
		RetentionDays     int  `json:"retentionDays"`
		ClearSavedMetrics bool `json:"clearSavedMetrics"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		writeError(w, r, http.StatusBadRequest, localization.InvalidMonitoringSettings)
		return
	}
	settings := system.MonitoringSettings{
		Enabled:       input.Enabled,
		RetentionDays: input.RetentionDays,
	}
	if !system.ValidMonitoringSettings(settings) || (input.ClearSavedMetrics && input.Enabled) {
		writeError(w, r, http.StatusBadRequest, localization.InvalidMonitoringSettings)
		return
	}
	if err := h.settings.SetSystemMonitoringSettings(r.Context(), settings); err != nil {
		writeError(w, r, http.StatusServiceUnavailable, localization.SettingsUnavailable)
		return
	}
	h.monitor.Configure(settings)
	if input.ClearSavedMetrics {
		if err := h.settings.ClearSystemMetrics(r.Context()); err != nil {
			writeError(w, r, http.StatusServiceUnavailable, localization.SettingsUnavailable)
			return
		}
	}
	h.writeMonitoringResponse(w, r, settings)
}

func (h settingsHandler) clearMetrics(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	if err := h.settings.ClearSystemMetrics(r.Context()); err != nil {
		writeError(w, r, http.StatusServiceUnavailable, localization.SettingsUnavailable)
		return
	}
	settings, err := h.settings.GetSystemMonitoringSettings(r.Context())
	if err != nil {
		writeError(w, r, http.StatusServiceUnavailable, localization.SettingsUnavailable)
		return
	}
	h.writeMonitoringResponse(w, r, settings)
}

func (h settingsHandler) writeMonitoringResponse(w http.ResponseWriter, r *http.Request, settings system.MonitoringSettings) {
	bytes, err := h.settings.SystemMetricsStorageBytes(r.Context())
	if err != nil {
		writeError(w, r, http.StatusServiceUnavailable, localization.SettingsUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		system.MonitoringSettings
		SavedMetricsBytes uint64 `json:"savedMetricsBytes"`
	}{MonitoringSettings: settings, SavedMetricsBytes: bytes})
}

func (h settingsHandler) authorize(w http.ResponseWriter, r *http.Request) bool {
	user, err := h.authenticator.Session(r.Context(), sessionToken(r))
	if err != nil {
		if errors.Is(err, identity.ErrUnauthenticated) {
			writeError(w, r, http.StatusUnauthorized, localization.Unauthenticated)
		} else {
			writeError(w, r, http.StatusInternalServerError, localization.AuthenticationUnavailable)
		}
		return false
	}
	if !identity.HasPermission(user, identity.PermissionSettingsManage) {
		writeError(w, r, http.StatusForbidden, localization.AccessDenied)
		return false
	}
	return true
}
