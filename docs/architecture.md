# AgentHub Architecture

## Goal

AgentHub is designed for one job: let agents safely change code while humans can approve, resume, inspect, and audit every step.

It should not start as a generic GitHub clone. Repository hosting can be integrated later, but the first-class control plane is built around agent identity, task workspaces, scoped capabilities, approvals, execution logs, branch orchestration, and review evidence.

## Core Primitives

### Agent

An agent is a first-class actor, not just a bot token.

Required fields:

- Agent profile
- Owner
- Purpose
- Capability scope
- Recovery owner
- Human binding
- Short-lived task token policy

Every operation should record an actor chain:

```text
human -> agent -> tool invocation
```

### Workspace

A workspace is the unit of agent work.

It binds:

- Issue or requirement
- Repo set
- Branch set
- Sandbox environment
- Agent memory
- Logs
- Produced commits and pull requests
- Approval state

Agents should not freely mutate repositories. They operate inside workspaces.

### Capability

Permissions should be modeled as capabilities, not only `read`, `write`, and `admin`.

Examples:

- `branch.create`
- `branch.push.agent_pattern`
- `pull_request.open`
- `issue.comment.edit`
- `review.request`
- `workflow.trigger`
- `secret.read`
- `network.access`
- `protected_path.modify`

Dangerous capabilities should default to denied and require explicit approval gates.

### Approval Gate

Approval gates make human control explicit.

Gate examples:

- Before push
- Before opening PR
- Before touching protected files
- Before privileged commands
- Before merge
- Before network access
- Before secret access
- Before cross-repo changes

Every approval record should include:

- Approver
- Approved action
- Diff or context snapshot
- Expiry
- Resulting operation id

### Change Bundle

A change bundle groups related code changes.

It can contain:

- One or more repositories
- One or more task branches
- Generated commits
- Pull requests
- Test evidence
- Review evidence
- Rollback or revert plan

This is required for cross-repo changes.

### Code Intelligence Primitive

Code intelligence primitives are trusted, auditable operations that let agents inspect code before changing it.

Examples:

- `code.grep`
- `code.read_file`
- `code.symbols`
- `code.references`
- `code.ownership`
- `code.ast_query`
- `code.dependencies`
- `code.history`
- `code.diff_map`
- `code.test_discover`

These primitives should be exposed through MCP, CLI, HTTP/gRPC, and internal Go use cases, but all entrypoints must share the same authorization, workspace scope, provenance, and audit path.

See [Code Intelligence Primitives](code-intelligence-primitives.md) for the detailed design and MVP.

### Audit Event

Audit events are append-only records for security, recovery, and review.

Important event types:

- Agent token issued
- Capability checked
- Workspace created
- Tool invoked
- Command executed
- File snapshot created
- Branch lease acquired
- Commit produced
- Approval requested
- Approval granted or denied
- Pull request opened
- Review thread assigned
- Check result produced

## Control Plane

The control plane owns identity, capabilities, workspace state, approvals, leases, events, and provenance.

Responsibilities:

- Validate actor chain
- Authorize capability usage
- Create and update workspaces
- Issue short-lived task tokens
- Manage branch leases
- Persist audit events
- Route agent hooks
- Track approval state

## Execution Plane

The execution plane runs agent work inside controlled environments.

Responsibilities:

- Create per-task devbox or container
- Apply network policy
- Apply secret policy
- Capture filesystem snapshots
- Execute reproducible commands
- Capture stdout, stderr, exit code, artifacts, and logs
- Resume long-running jobs

The execution plane should never decide product policy by itself. It receives policy from the control plane and emits auditable results.

## Semantic Context Layer

The semantic layer helps agents understand code.

It may maintain:

- Repo symbol index
- Dependency graph
- Ownership map
- Embedding search
- Issue, PR, and code cross references
- Repo, task, and org scoped memory
- Historical summaries such as "why was this line changed"

Important rule: summaries are not facts. Every semantic result must link to Git, database records, logs, or artifacts.

The semantic layer should be consumed through code intelligence primitives rather than ad hoc shell commands when an agent is working inside AgentHub. This makes retrieved context durable, permission-aware, and available for commit provenance.

## Event Bus

AgentHub should have native agent hooks, not only webhooks for external integrations.

Examples:

- Issue assigned to agent
- Review comment assigned to agent
- CI failed with logs
- Branch conflict appeared
- Dependency changed
- Human requested retry
- PR approved and ready to merge
- Production incident linked

Events should route into workspace task queues with retry, debounce, and dead-letter handling.

## Go Implementation

AgentHub should be implemented in Go because the product needs strong backend service boundaries, reliable concurrency, simple deployment artifacts, and clear integration with Git, containers, queues, and policy engines.

Recommended service split for the first version:

- API service: HTTP/gRPC API for workspaces, agents, approvals, and reviews
- MCP service: agent-facing code intelligence and workspace tools
- Worker service: event handling, background jobs, rebase queue, checks
- Runner service: sandbox command execution and artifact capture
- Indexer service: semantic code index and ownership graph

These can start as packages in one Go module and later split into deployable binaries under `cmd/`.

## System Of Record

Use PostgreSQL for the primary control-plane database.

The schema should explicitly model agent identity, capabilities, workspaces, branch leases, approval gates, sandbox execution records, review evidence, event routing, durable memory, semantic context metadata, checks, multi-agent coordination, and append-only audit events.

See [Database Schema](database-schema.md) for the proposed table design.
