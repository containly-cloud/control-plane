package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api/internal/identity"
	"api/internal/system"
)

type settingsStoreStub struct {
	settings system.MonitoringSettings
	setCalls int
	bytes    uint64
	clears   int
}

func (s *settingsStoreStub) GetSystemMonitoringSettings(context.Context) (system.MonitoringSettings, error) {
	return s.settings, nil
}
func (s *settingsStoreStub) SetSystemMonitoringSettings(_ context.Context, settings system.MonitoringSettings) error {
	s.settings = settings
	s.setCalls++
	return nil
}
func (s *settingsStoreStub) SystemMetricsStorageBytes(context.Context) (uint64, error) {
	return s.bytes, nil
}
func (s *settingsStoreStub) ClearSystemMetrics(context.Context) error {
	s.clears++
	s.bytes = 0
	return nil
}

type monitoringControllerRecorder struct {
	settings system.MonitoringSettings
	calls    int
}

func (m *monitoringControllerRecorder) Configure(settings system.MonitoringSettings) {
	m.settings = settings
	m.calls++
}

func TestMonitoringSettingsRequireManagementPermission(t *testing.T) {
	store := &settingsStoreStub{}
	monitor := &monitoringControllerRecorder{}
	handler := settingsHandler{
		authenticator: userAuthenticatorStub{user: identity.User{}},
		settings:      store,
		monitor:       monitor,
	}
	request := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"enabled":false,"intervalSeconds":60,"retentionDays":30}`))
	response := httptest.NewRecorder()

	handler.updateMonitoring(response, request)

	if response.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if store.setCalls != 0 || monitor.calls != 0 {
		t.Errorf("settings must not change without permission")
	}
}

func TestMonitoringSettingsApplyWithoutServerRestart(t *testing.T) {
	store := &settingsStoreStub{}
	monitor := &monitoringControllerRecorder{}
	handler := settingsHandler{
		authenticator: userAuthenticatorStub{user: identity.User{Permissions: []string{identity.PermissionSettingsManage}}},
		settings:      store,
		monitor:       monitor,
	}
	request := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"enabled":false,"intervalSeconds":60,"retentionDays":30}`))
	response := httptest.NewRecorder()

	handler.updateMonitoring(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	want := system.MonitoringSettings{Enabled: false, IntervalSeconds: 60, RetentionDays: 30}
	if store.settings != want || monitor.settings != want || monitor.calls != 1 {
		t.Errorf("settings were not applied immediately: store=%+v monitor=%+v calls=%d", store.settings, monitor.settings, monitor.calls)
	}
}

func TestDisablingMonitoringCanClearSavedMetrics(t *testing.T) {
	store := &settingsStoreStub{bytes: 4096}
	monitor := &monitoringControllerRecorder{}
	handler := settingsHandler{
		authenticator: userAuthenticatorStub{user: identity.User{Permissions: []string{identity.PermissionSettingsManage}}},
		settings:      store,
		monitor:       monitor,
	}
	request := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"enabled":false,"intervalSeconds":60,"retentionDays":30,"clearSavedMetrics":true}`))
	response := httptest.NewRecorder()

	handler.updateMonitoring(response, request)

	if response.Code != http.StatusOK || store.clears != 1 || store.bytes != 0 {
		t.Errorf("metrics were not cleared: status=%d clears=%d bytes=%d", response.Code, store.clears, store.bytes)
	}
}

func TestClearMetricsRequiresSettingsManagementPermission(t *testing.T) {
	store := &settingsStoreStub{bytes: 4096}
	handler := settingsHandler{
		authenticator: userAuthenticatorStub{user: identity.User{}},
		settings:      store,
		monitor:       &monitoringControllerRecorder{},
	}
	request := httptest.NewRequest(http.MethodDelete, "/", nil)
	response := httptest.NewRecorder()

	handler.clearMetrics(response, request)

	if response.Code != http.StatusForbidden || store.clears != 0 {
		t.Errorf("clear without permission: status=%d clears=%d", response.Code, store.clears)
	}
}
