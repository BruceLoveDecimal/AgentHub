# Go Code Organization

## Principles

AgentHub should use a layered Go architecture with strict dependency direction.

The goal is to keep policy and workflow testable without depending directly on infrastructure systems such as Git, PostgreSQL, queues, containers, or GitHub APIs.

Dependency direction:

```text
interfaces -> application -> service -> domain
infra -------> application ports
```

The domain layer must not import infrastructure packages.

## Proposed Directory Layout

```text
.
├── cmd/
│   ├── agenthub-api/
│   │   └── main.go
│   ├── agenthub-worker/
│   │   └── main.go
│   ├── agenthub-runner/
│   │   └── main.go
│   ├── agenthub-mcp/
│   │   └── main.go
│   ├── agenthub-cli/
│   │   └── main.go
│   └── agenthub-indexer/
│       └── main.go
├── internal/
│   ├── application/
│   │   ├── agents/
│   │   ├── approvals/
│   │   ├── branches/
│   │   ├── changes/
│   │   ├── codeintel/
│   │   ├── checks/
│   │   ├── events/
│   │   ├── reviews/
│   │   ├── sandboxes/
│   │   └── workspaces/
│   ├── domain/
│   │   ├── agent/
│   │   ├── approval/
│   │   ├── audit/
│   │   ├── branch/
│   │   ├── capability/
│   │   ├── change/
│   │   ├── codeintel/
│   │   ├── event/
│   │   ├── review/
│   │   ├── sandbox/
│   │   └── workspace/
│   ├── service/
│   │   ├── authorization/
│   │   ├── codeintel/
│   │   ├── orchestration/
│   │   ├── provenance/
│   │   ├── risk/
│   │   └── workflow/
│   ├── infra/
│   │   ├── db/
│   │   ├── ast/
│   │   ├── git/
│   │   ├── github/
│   │   ├── queue/
│   │   ├── sandbox/
│   │   ├── search/
│   │   ├── secrets/
│   │   └── storage/
│   ├── interfaces/
│   │   ├── http/
│   │   ├── grpc/
│   │   ├── cli/
│   │   ├── mcp/
│   │   ├── webhook/
│   │   └── worker/
│   └── utils/
│       ├── clock/
│       ├── ids/
│       ├── hashing/
│       └── validation/
├── migrations/
├── configs/
├── deployments/
├── docs/
└── tests/
```

## Layer Responsibilities

### `cmd/`

Contains executable entrypoints only.

Rules:

- Parse config
- Initialize logger
- Wire dependencies
- Start servers or workers
- No business logic

### `internal/domain/`

Contains core business concepts and invariants.

Examples:

- Agent identity
- Workspace state machine
- Capability expressions
- Approval gate status
- Branch lease invariants
- Change bundle model
- Audit event model

Rules:

- No database imports
- No HTTP imports
- No GitHub SDK imports
- No container runtime imports
- Prefer pure Go types and methods

### `internal/application/`

Contains use cases and ports.

Examples:

- `CreateWorkspace`
- `AssignAgent`
- `RequestApproval`
- `AcquireBranchLease`
- `RecordToolInvocation`
- `GrepCode`
- `ReadFile`
- `FindSymbols`
- `FindReferences`
- `OpenChangeBundlePRs`
- `AssignReviewThreadToAgent`

Application packages define interfaces for required infrastructure:

```go
type WorkspaceStore interface {
    Save(ctx context.Context, workspace *workspace.Workspace) error
    Find(ctx context.Context, id workspace.ID) (*workspace.Workspace, error)
}
```

Infrastructure implements these interfaces.

### `internal/service/`

Contains reusable business services that coordinate domain behavior.

Examples:

- Authorization service
- Code intelligence query service
- Provenance service
- Risk classification service
- Workspace orchestration service
- Review evidence service

Rules:

- Can depend on domain
- Can be called by application use cases
- Should not contain transport-specific code
- Should not directly own persistence unless behind application-defined ports

### `internal/infra/`

Contains concrete adapters.

Examples:

- PostgreSQL repositories
- Git command adapter
- AST parser adapter
- GitHub API adapter
- Queue adapter
- Container/devbox runner
- Object storage adapter
- Secret provider
- Embedding/vector search adapter

Rules:

- Implements application ports
- Contains external SDK usage
- Converts external errors into application/domain errors
- Does not decide core policy

### `internal/interfaces/`

Contains inbound adapters.

Examples:

- HTTP handlers
- gRPC handlers
- CLI commands
- MCP tools
- GitHub webhook handlers
- Worker event handlers

Rules:

- Parse and validate requests
- Call application use cases
- Map responses and errors
- No business policy beyond request-level validation

### `internal/utils/`

Contains small, low-level helpers.

Allowed examples:

- Clock abstraction
- ID generation
- Hashing helpers
- Validation helpers

Rules:

- Keep this package small
- No business concepts
- No imports from `application`, `service`, `domain`, `infra`, or `interfaces`
- Prefer specific packages like `utils/clock` over a broad `utils` package

## Package Naming

Prefer business names over technical names.

Good:

- `workspace`
- `approval`
- `capability`
- `provenance`
- `branch`

Avoid:

- `manager`
- `common`
- `helper`
- `misc`

## Initial Go Module

Use a single Go module at first:

```text
module github.com/agenthub/agenthub
```

The module path can be changed once the final repository organization is decided.

## Testing Strategy

Recommended test levels:

- Domain tests for invariants and state transitions
- Application tests with fake ports
- Infra integration tests for database, Git, queue, and sandbox adapters
- End-to-end workflow tests for workspace lifecycle

Critical workflows to test early:

- Agent token issuance and expiry
- Capability authorization
- Workspace creation
- Branch lease acquisition
- Approval gate enforcement
- Command execution audit
- Commit provenance recording
