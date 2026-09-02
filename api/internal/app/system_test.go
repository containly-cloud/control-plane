package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api/internal/identity"
	"api/internal/system"
)

type metricHistoryRecorder struct {
	metrics []system.MetricSample
	calls   int
	from    time.Time
	to      time.Time
	oldest  time.Time
}

func (h *metricHistoryRecorder) OldestSystemMetric(context.Context) (time.Time, bool, error) {
	return h.oldest, !h.oldest.IsZero(), nil
}

func (h *metricHistoryRecorder) ListSystemMetrics(_ context.Context, from, to time.Time, _ system.MetricGranularity) ([]system.MetricSample, error) {
	h.calls++
	h.from = from
	h.to = to
	return h.metrics, nil
}

func TestMetricHistoryIsAvailableToWorkspaceUsers(t *testing.T) {
	history := &metricHistoryRecorder{}
	handler := systemHandler{
		authenticator: userAuthenticatorStub{user: identity.User{Role: "member"}},
		history:       history,
	}
	request := httptest.NewRequest(http.MethodGet, "/?from=2026-01-01T00:00:00Z&to=2026-01-02T00:00:00Z", nil)
	response := httptest.NewRecorder()

	handler.metricsResponse(response, request)

	if response.Code != http.StatusOK || history.calls != 1 {
		t.Errorf("history for workspace user: status=%d calls=%d", response.Code, history.calls)
	}
}

func TestMetricHistoryReturnsOnlyRequestedRange(t *testing.T) {
	history := &metricHistoryRecorder{metrics: []system.MetricSample{{CapturedAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}}}
	handler := systemHandler{
		authenticator: userAuthenticatorStub{user: identity.User{Role: "member", Permissions: []string{identity.PermissionWorkspaceRead}}},
		history:       history,
	}
	request := httptest.NewRequest(http.MethodGet, "/?from=2026-01-01T00:00:00Z&to=2026-01-02T00:00:00Z", nil)
	response := httptest.NewRecorder()

	handler.metricsResponse(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if history.from != time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) || history.to != time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC) {
		t.Errorf("range = %v to %v", history.from, history.to)
	}
}

func TestMetricHistoryLimitsRangeToOldestAvailableMetric(t *testing.T) {
	oldest := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	history := &metricHistoryRecorder{oldest: oldest}
	handler := systemHandler{
		authenticator: userAuthenticatorStub{user: identity.User{Role: "member", Permissions: []string{identity.PermissionWorkspaceRead}}},
		history:       history,
	}
	request := httptest.NewRequest(http.MethodGet, "/?from=2026-01-01T00:00:00Z&to=2026-01-02T00:00:00Z", nil)
	response := httptest.NewRecorder()

	handler.metricsResponse(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if history.from != oldest {
		t.Errorf("from = %v, want %v", history.from, oldest)
	}
}
