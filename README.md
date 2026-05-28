# AgentHub

AgentHub is an agent-native code change platform.

The core product is not repository hosting. The core product is a safe, resumable, and auditable control plane that lets software agents modify code inside explicit task workspaces.

## Product Thesis

Traditional Git platforms are centered on repositories, issues, and pull requests. AgentHub adds first-class primitives for agent work:

- Agent identity with scoped capabilities
- Task workspace as the unit of agent execution
- Branch leases and change provenance
- Human approval gates
- Execution sandbox and command audit
- Durable conversation, memory, and artifacts
- Agent-aware review and checks
- Multi-agent collaboration and cross-repo change bundles

The source of truth remains Git, database records, logs, and artifacts. Semantic summaries and memory help agents navigate the system, but they must always link back to verifiable sources.

## Implementation Direction

AgentHub should be implemented in Go.

The backend should use clear package boundaries:

- `cmd/`: service entrypoints
- `internal/domain/`: core entities, value objects, policies, and invariants
- `internal/application/`: use cases and ports
- `internal/service/`: business services that coordinate domain behavior
- `internal/infra/`: concrete adapters for Git, database, queues, sandboxes, storage, and external APIs
- `internal/interfaces/`: HTTP, gRPC, CLI, webhook, and worker handlers
- `internal/utils/`: small shared utilities with strict dependency rules

See:

- [Architecture](docs/architecture.md)
- [Code Organization](docs/code-organization.md)
- [Database Schema](docs/database-schema.md)
- [Milestones](docs/milestones.md)
