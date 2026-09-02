package app

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api/internal/identity"
	"api/internal/storage"
	"api/internal/system"
)

type rootCreatorStub struct{}

func (rootCreatorStub) Create(context.Context, string, string) error { return nil }

type setupStatusStub struct{ configured bool }

func (s setupStatusStub) HasRoot(context.Context) (bool, error) { return s.configured, nil }

type authenticatorStub struct{}

func (authenticatorStub) Login(context.Context, string, string, identity.SessionMetadata) (identity.Session, error) {
	return identity.Session{}, nil
}

func (authenticatorStub) Session(context.Context, string) (identity.User, error) {
	return identity.User{}, nil
}

func (authenticatorStub) Logout(context.Context, string) error { return nil }

type systemOverviewStub struct{}

func (systemOverviewStub) Overview(context.Context) (system.Overview, error) {
	return system.Overview{}, nil
}

type systemMetricHistoryStub struct{}

func (systemMetricHistoryStub) ListSystemMetrics(context.Context, time.Time, time.Time, system.MetricGranularity) ([]system.MetricSample, error) {
	return nil, nil
}

func (systemMetricHistoryStub) OldestSystemMetric(context.Context) (time.Time, bool, error) {
	return time.Time{}, false, nil
}

type monitoringSettingsStub struct{}

func (monitoringSettingsStub) GetSystemMonitoringSettings(context.Context) (system.MonitoringSettings, error) {
	return system.DefaultMonitoringSettings(), nil
}
func (monitoringSettingsStub) SetSystemMonitoringSettings(context.Context, system.MonitoringSettings) error {
	return nil
}
func (monitoringSettingsStub) SystemMetricsStorageBytes(context.Context) (uint64, error) {
	return 0, nil
}
func (monitoringSettingsStub) ClearSystemMetrics(context.Context) error { return nil }

type monitoringControllerStub struct{}

func (monitoringControllerStub) Configure(system.MonitoringSettings) {}

type sessionInventoryStub struct{}

func (sessionInventoryStub) ListActiveSessions(context.Context, int64, time.Time) ([]identity.ActiveSession, error) {
	return nil, nil
}

type backupManagerStub struct{}

func (backupManagerStub) Backup(context.Context) (string, error)     { return "", nil }
func (backupManagerStub) ListBackups() ([]storage.BackupFile, error) { return nil, nil }
func (backupManagerStub) DeleteBackup(string) error                  { return nil }

type controlUsersStub struct{}

func (controlUsersStub) ListControlUsers(context.Context) ([]storage.ControlUser, error) {
	return nil, nil
}
func (controlUsersStub) CreateControlUser(context.Context, string, string, []string) (storage.ControlUser, error) {
	return storage.ControlUser{}, nil
}
func (controlUsersStub) SetControlUserActive(context.Context, int64, bool) error          { return nil }
func (controlUsersStub) SetControlUserPermissions(context.Context, int64, []string) error { return nil }
func (controlUsersStub) ResetControlUserPassword(context.Context, int64, string) error    { return nil }
func (controlUsersStub) DeleteControlUser(context.Context, int64) error                   { return nil }

func TestAPIRoutesStayUnderAPIPrefix(t *testing.T) {
	handler, err := New(Dependencies{
		Logger:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		RootCreator:          rootCreatorStub{},
		SetupStatus:          setupStatusStub{configured: false},
		Authenticator:        authenticatorStub{},
		SystemOverview:       systemOverviewStub{},
		SystemMetricHistory:  systemMetricHistoryStub{},
		SessionInventory:     sessionInventoryStub{},
		BackupManager:        backupManagerStub{},
		ControlUsers:         controlUsersStub{},
		MonitoringSettings:   monitoringSettingsStub{},
		MonitoringController: monitoringControllerStub{},
		WebHandler:           http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusTeapot) }),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	statusRequest := httptest.NewRequest(http.MethodGet, "/api/v1/setup/status", nil)
	statusResponse := httptest.NewRecorder()
	handler.ServeHTTP(statusResponse, statusRequest)
	if statusResponse.Code != http.StatusOK {
		t.Errorf("GET setup status = %d, want %d", statusResponse.Code, http.StatusOK)
	}

	unknownAPIRequest := httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	unknownAPIRequest.Header.Set("Accept-Language", "en-US")
	unknownAPIResponse := httptest.NewRecorder()
	handler.ServeHTTP(unknownAPIResponse, unknownAPIRequest)
	if unknownAPIResponse.Code != http.StatusNotFound {
		t.Errorf("GET unknown API route = %d, want %d", unknownAPIResponse.Code, http.StatusNotFound)
	}
	if language := unknownAPIResponse.Header().Get("Content-Language"); language != "en-US" {
		t.Errorf("Content-Language = %q, want en-US", language)
	}
	var apiError struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(unknownAPIResponse.Body).Decode(&apiError); err != nil {
		t.Fatalf("decode API error: %v", err)
	}
	if apiError.Code != "api_route_not_found" || apiError.Error != "API route not found." {
		t.Errorf("API error = %#v", apiError)
	}

	workspaceRequest := httptest.NewRequest(http.MethodGet, "/workspace", nil)
	workspaceResponse := httptest.NewRecorder()
	handler.ServeHTTP(workspaceResponse, workspaceRequest)
	if workspaceResponse.Code != http.StatusTeapot {
		t.Errorf("GET workspace = %d, want %d", workspaceResponse.Code, http.StatusTeapot)
	}
}
