# Containly Cloud Control Plane

The Control Plane is the operational layer of Containly Cloud. It provides the platform surface through which users and external integrations manage Containly environments, machines, modules, and the resources attached to them.

Its role is to turn requests made at the platform boundary into coordinated actions in Containly-managed environments. Authentication is part of this experience, but this repository is not a general-purpose identity and access management system.

## Scope

The Control Plane is the home for platform-level capabilities, including:

- The user-facing workspace for operating Containly.
- External entry points for platform operations.
- Access to the Containly platform.
- Management of machines, modules, and their lifecycle.
- Resource visibility and management for Containly users and environments.

```text
Users and external integrations
              │
              ▼
       Control Plane
       ├── Platform workspace
       └── Control entry point
              │
              ▼
Containly modules, machines, and resources
```

## Monorepo layout

```text
.
├── api/                 # Platform entry point and control operations
│   ├── main.go          # Application bootstrap
│   └── internal/web/    # Web application delivery in each environment
├── ui/                  # Containly operational workspace
│   ├── src/             # Screens, routes, and interface assets
│   └── public/          # Public interface assets
├── dist/                # Generated distribution artifacts
├── package.json         # Repository-wide workflow commands
├── go.work              # Workspace definition for the entry application
└── .vscode/             # Shared editor preferences
```

### `api/`

Contains the application at the edge of the Control Plane. It owns the platform-facing operations and coordinates work involving Containly users, machines, resources, and modules.

`internal/web/` connects this application to the operational workspace: during development it forwards web traffic to the running interface; in a distributable build it serves the prepared interface alongside the Control Plane. It is internal because no other Go module should consume this delivery detail.

### `ui/`

Contains the operational workspace used to interact with the platform. Product views and navigation live here, with routes under `src/routes/`. This keeps the user experience separate from the operations coordinated by `api/`.

### Repository root

The root contains the shared workspace configuration and commands that coordinate the two applications. It is intentionally thin: the interface and the control entry point retain their own responsibilities while being built and delivered together.

## Responsibility boundaries

| Need | Primary area |
| --- | --- |
| Workspace screens, navigation, and user interactions | `ui/` |
| Platform entry points and coordination of operations | `api/` |
| Interface-to-control communication | Contract exposed by `api/` and used by `ui/` |
| Generated distribution output | `dist/` and `api/internal/web/dist/` |

## Delivery flow

1. The operational workspace is prepared from `ui/`.
2. Its output is included by the application in `api/`.
3. The resulting distribution delivers the workspace and the Control Plane together.

Generated artifacts are build output, not the source of truth for product changes.

## First access and local data

At the first access, the Control Plane displays the onboarding screen and asks
for an administrator username and password. The account has the internal
`root` role, but its username is chosen by the person setting up the instance.
The password is stored only as an Argon2id hash; it is never persisted or
logged in plain text.

Password recovery and reset are intentionally unavailable in this release.
There is no recovery API, administrative backdoor, or reversible credential
store: if the administrator password is lost, access to this Control Plane is
lost. A future authentication design may add MFA and an approved recovery
process, but neither should be introduced implicitly.

All API endpoints use the `/api` prefix. The application therefore serves the
workspace and API on the same origin:

| Route | Responsibility |
| --- | --- |
| `GET /api/v1/setup/status` | Indicates whether the first administrator exists. |
| `POST /api/v1/setup/root` | Creates the first administrator exactly once. |
| `GET /api/v1/system/overview` | Returns the authenticated account's live host overview. |
| `GET /api/v1/account/sessions` | Returns active sessions for the authenticated account. |
| Any non-`/api` route | Delivers the operational workspace. |

### Public API and language

Public API responses that contain user-facing text are localized from the
request's `Accept-Language` header. `pt-BR` and `en-US` are supported, with
`pt-BR` as the fallback. The response sets `Content-Language` and includes a
stable, language-neutral error `code`; logs remain in English and never use
localized product copy.

The web application bundles those same two locale catalogs through `i18next`
and `react-i18next`, using strict TypeScript resource typing and a local
fallback. It does not request translations from the API. Locale selection is
persisted in the browser, sent with API requests, and can be changed in the
onboarding screen. React Query caches the setup status indefinitely for the
page lifecycle and disables retries and refetch-on-focus, avoiding accidental
traffic amplification against the public API.

### Authentication and permissions

After initial setup, the workspace requires a username and password. A
successful login creates a 24-hour opaque session stored in a `HttpOnly`,
`SameSite=Strict` cookie. The database holds only the SHA-256 checksum of that
random session token, so neither the password nor a usable session token is
available to browser JavaScript or backup readers.

`GET /api/v1/auth/session` and `POST /api/v1/auth/login` return the current
principal's effective permissions, not its internal role. The web application
uses these permissions only to show or hide navigation and controls. Every
authorization decision belongs to the API; the current root account receives
`users:manage` and `settings:manage` in addition to `workspace:read`.

### Host monitoring

The first authenticated workspace view is the host overview. The Control Plane
reads Linux kernel interfaces directly for CPU, memory, storage, network and
distribution information; it does not install a separate monitoring agent.
CPU and memory samples are collected every five seconds, held in memory for
the live API response and written to SQLite. Raw samples are retained for 30
days so the database and its backups remain bounded. The browser refreshes the
authenticated overview every five seconds only while the page is visible.

The service resolves the public IP from the server, with a six-hour cache. The
browser never calls that external service. Active session details are limited
to the current authenticated account and contain client IP, user agent,
creation time and last activity; they never include passwords or session
tokens.

### Local data layout

Control Plane owns this directory tree, with permissions restricted to the
current user:

```text
~/.containly/
└── control-plane/
    ├── data/
    │   └── control-plane.sqlite3     # primary local state
    ├── backups/                      # consistent snapshots made before migrations
    └── logs/                         # reserved for local operational logs
```

`CONTAINLY_HOME` can override `~/.containly` for a portable or test
installation. It must point to the directory that will contain
`control-plane/`.

To protect an installation, back up the whole `~/.containly` directory while
the process is stopped. Do not copy only a live `.sqlite3` file: SQLite may
keep current writes in its WAL sidecar. Before every pending schema migration,
the application generates a consistent standalone SQLite snapshot via
`VACUUM INTO` in `backups/`.

### Schema updates

Migrations live in `api/internal/storage/migrations` and are embedded into the
application binary. They are numbered, applied transactionally, and registered
with a SHA-256 checksum in `schema_migrations`. On a new release, only missing
versions run. An already-applied migration must never be edited: add a new
numbered SQL file instead. A checksum mismatch intentionally stops startup,
which prevents silently running a database with an unknown schema history.

The API bootstrap is composed through explicit dependencies: `main` creates
the logger, local storage and identity service; HTTP handlers depend on small
interfaces rather than SQLite. JSON request logs are written to standard error
and include request ID, method, path, status, response size and duration.
