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
│   └── pkg/web/         # Web application delivery in each environment
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

`pkg/web/` connects this application to the operational workspace: during development it forwards web traffic to the running interface; in a distributable build it serves the prepared interface alongside the Control Plane.

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
| Generated distribution output | `dist/` and `api/pkg/web/dist/` |

## Delivery flow

1. The operational workspace is prepared from `ui/`.
2. Its output is included by the application in `api/`.
3. The resulting distribution delivers the workspace and the Control Plane together.

Generated artifacts are build output, not the source of truth for product changes.
