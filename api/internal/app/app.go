// Package app assembles HTTP concerns through explicit dependencies.
package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"api/internal/identity"
	"api/internal/localization"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// RootCreator is the setup use case consumed by HTTP, independent of storage.
type RootCreator interface {
	Create(ctx context.Context, username, password string) error
}

// SetupStatus is the read model needed to decide which first-access screen to
// show. It deliberately exposes no user data.
type SetupStatus interface {
	HasRoot(ctx context.Context) (bool, error)
}

type Authenticator interface {
	Login(ctx context.Context, username, password string, metadata identity.SessionMetadata) (identity.Session, error)
	Session(ctx context.Context, token string) (identity.User, error)
	Logout(ctx context.Context, token string) error
}

// Dependencies makes infrastructure ownership clear at the composition root.
type Dependencies struct {
	Logger               *slog.Logger
	RootCreator          RootCreator
	SetupStatus          SetupStatus
	Authenticator        Authenticator
	SystemOverview       SystemOverview
	SystemMetricHistory  SystemMetricHistory
	SessionInventory     SessionInventory
	BackupManager        BackupManager
	ControlUsers         ControlUserManager
	MonitoringSettings   MonitoringSettingsStore
	MonitoringController MonitoringController
	WebHandler           http.Handler
}

func New(dependencies Dependencies) (http.Handler, error) {
	if dependencies.Logger == nil || dependencies.RootCreator == nil || dependencies.SetupStatus == nil || dependencies.Authenticator == nil || dependencies.SystemOverview == nil || dependencies.SystemMetricHistory == nil || dependencies.SessionInventory == nil || dependencies.BackupManager == nil || dependencies.ControlUsers == nil || dependencies.MonitoringSettings == nil || dependencies.MonitoringController == nil || dependencies.WebHandler == nil {
		return nil, errors.New("application dependencies must not be nil")
	}

	handler := setupHandler{rootCreator: dependencies.RootCreator, setupStatus: dependencies.SetupStatus}
	auth := authHandler{authenticator: dependencies.Authenticator}
	system := systemHandler{authenticator: dependencies.Authenticator, overview: dependencies.SystemOverview, history: dependencies.SystemMetricHistory}
	account := accountHandler{authenticator: dependencies.Authenticator, sessions: dependencies.SessionInventory}
	backups := backupHandler{authenticator: dependencies.Authenticator, backups: dependencies.BackupManager}
	users := usersHandler{authenticator: dependencies.Authenticator, users: dependencies.ControlUsers}
	settings := settingsHandler{authenticator: dependencies.Authenticator, settings: dependencies.MonitoringSettings, monitor: dependencies.MonitoringController}
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(requestLogger(dependencies.Logger))
	router.Use(middleware.Recoverer)
	router.Route("/api", func(apiRouter chi.Router) {
		apiRouter.Get("/v1/setup/status", handler.status)
		apiRouter.Post("/v1/setup/root", handler.createRoot)
		apiRouter.Post("/v1/auth/login", auth.login)
		apiRouter.Get("/v1/auth/session", auth.session)
		apiRouter.Post("/v1/auth/logout", auth.logout)
		apiRouter.Get("/v1/system/overview", system.overviewResponse)
		apiRouter.Get("/v1/system/metrics", system.metricsResponse)
		apiRouter.Get("/v1/account/sessions", account.sessionsResponse)
		apiRouter.Get("/v1/backups", backups.list)
		apiRouter.Post("/v1/backups", backups.create)
		apiRouter.Delete("/v1/backups", backups.remove)
		apiRouter.Get("/v1/control-plane/users", users.list)
		apiRouter.Post("/v1/control-plane/users", users.create)
		apiRouter.Delete("/v1/control-plane/users/{id}", users.remove)
		apiRouter.Put("/v1/control-plane/users/permissions", users.permissions)
		apiRouter.Post("/v1/control-plane/users/password/reset", users.resetPassword)
		apiRouter.Get("/v1/control-plane/settings/system-monitoring", settings.monitoring)
		apiRouter.Put("/v1/control-plane/settings/system-monitoring", settings.updateMonitoring)
		apiRouter.Delete("/v1/control-plane/settings/system-monitoring/metrics", settings.clearMetrics)
		apiRouter.NotFound(apiNotFound)
	})
	router.Handle("/*", dependencies.WebHandler)
	return router, nil
}

const sessionCookieName = "containly_session"

func sessionCookie(request *http.Request, token string, expiresAt time.Time) *http.Cookie {
	secure := request.TLS != nil || strings.EqualFold(request.Header.Get("X-Forwarded-Proto"), "https")
	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	}
	if token == "" {
		cookie.MaxAge = -1
	}
	return cookie
}

func apiNotFound(writer http.ResponseWriter, request *http.Request) {
	writeError(writer, request, http.StatusNotFound, localization.APIRouteNotFound)
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			startedAt := time.Now()
			response := &responseRecorder{ResponseWriter: writer, status: http.StatusOK}
			next.ServeHTTP(response, request)
			logger.Info("http request",
				"request_id", middleware.GetReqID(request.Context()),
				"method", request.Method,
				"path", request.URL.Path,
				"status", response.status,
				"bytes", response.bytes,
				"duration_ms", time.Since(startedAt).Milliseconds(),
			)
		})
	}
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(body []byte) (int, error) {
	bytes, err := r.ResponseWriter.Write(body)
	r.bytes += bytes
	return bytes, err
}
