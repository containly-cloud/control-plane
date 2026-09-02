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
	history       SystemMetricHistory
}

func (h systemHandler) metricsResponse(w http.ResponseWriter, r *http.Request) {
	user, err := h.authenticator.Session(r.Context(), sessionToken(r))
	if err != nil {
		if errors.Is(err, identity.ErrUnauthenticated) {
			writeError(w, r, http.StatusUnauthorized, localization.Unauthenticated)
		} else {
			writeError(w, r, http.StatusInternalServerError, localization.AuthenticationUnavailable)
		}
		return
	}
	if !identity.HasPermission(user, identity.PermissionWorkspaceRead) {
		writeError(w, r, http.StatusForbidden, localization.AccessDenied)
		return
	}
	from, to, granularity, ok := metricRange(r)
	if !ok {
		writeError(w, r, http.StatusBadRequest, localization.InvalidMetricRange)
		return
	}
	oldestAvailableAt, found, err := h.history.OldestSystemMetric(r.Context())
	if err != nil {
		writeError(w, r, http.StatusServiceUnavailable, localization.SystemOverviewUnavailable)
		return
	}
	if found && from.Before(oldestAvailableAt) {
		from = oldestAvailableAt
	}
	metrics, err := h.history.ListSystemMetrics(r.Context(), from, to, granularity)
	if err != nil {
		writeError(w, r, http.StatusServiceUnavailable, localization.SystemOverviewUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Metrics           []system.MetricSample    `json:"metrics"`
		Granularity       system.MetricGranularity `json:"granularity"`
		OldestAvailableAt *time.Time               `json:"oldestAvailableAt"`
	}{
		Metrics:           metrics,
		Granularity:       granularity,
		OldestAvailableAt: optionalMetricTime(oldestAvailableAt, found),
	})
}

func optionalMetricTime(value time.Time, found bool) *time.Time {
	if !found {
		return nil
	}
	value = value.UTC()
	return &value
}

func metricRange(r *http.Request) (time.Time, time.Time, system.MetricGranularity, bool) {
	from, err := time.Parse(time.RFC3339, r.URL.Query().Get("from"))
	if err != nil {
		return time.Time{}, time.Time{}, "", false
	}
	to, err := time.Parse(time.RFC3339, r.URL.Query().Get("to"))
	if err != nil || !to.After(from) || to.Sub(from) > 3650*24*time.Hour {
		return time.Time{}, time.Time{}, "", false
	}
	granularity := system.MetricGranularity(r.URL.Query().Get("granularity"))
	if granularity == "" {
		granularity = system.MetricGranularityMinute
	}
	if !system.ValidMetricGranularity(granularity) {
		return time.Time{}, time.Time{}, "", false
	}
	return from.UTC(), to.UTC(), granularity, true
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

type SystemMetricHistory interface {
	ListSystemMetrics(context.Context, time.Time, time.Time, system.MetricGranularity) ([]system.MetricSample, error)
	OldestSystemMetric(context.Context) (time.Time, bool, error)
}

// SessionInventory is the server-side device listing used for an authenticated
// account. The client never chooses which account can be read.
type SessionInventory interface {
	ListActiveSessions(ctx context.Context, userID int64, now time.Time) ([]identity.ActiveSession, error)
}
