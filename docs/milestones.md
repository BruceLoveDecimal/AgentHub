# Milestones

## Milestone 0: Product Spec And Architecture Seed

Goal: define AgentHub as an agent-native code change control plane, not a generic Git hosting clone.

Deliverables:

- Product principles
- Architecture document
- Go code organization document
- Core primitive definitions
- Initial threat model

Success criteria:

- Contributors understand that AgentHub is centered on safe, resumable, auditable agent code changes.
- The initial Go package boundaries are clear enough to start implementation.

## Milestone 1: Agent Identity And Capability Model

Goal: make agents first-class actors.

Scope:

- Agent profile
- Owner and recovery owner
- Purpose
- Capability scope
- Short-lived task token
- Repo, org, team, and path scoped permissions
- Actor chain: `human -> agent -> tool invocation`

Go focus:

- `internal/domain/agent`
- `internal/domain/capability`
- `internal/domain/audit`
- `internal/application/agents`
- `internal/service/authorization`

Success criteria:

- Every operation can answer who requested it, which agent executed it, and which capability allowed it.

## Milestone 2: Workspace / Task Primitive

Goal: make workspace the unit of agent execution.

Scope:

- Requirement or issue binding
- Repo set
- Branch set
- Sandbox config
- Agent memory references
- Logs and artifacts
- Produced commits and PRs
- Approval state

Go focus:

- `internal/domain/workspace`
- `internal/application/workspaces`
- `internal/service/workflow`
- `internal/infra/db`
- `internal/interfaces/http`

Success criteria:

- Agents operate inside explicit workspaces instead of directly mutating repositories.

## Milestone 3: Code Intelligence Primitives MVP

Goal: give agents trusted, permission-aware, commit-pinned code context before they generate patches.

Scope:

- Expose code intelligence primitives through MCP, CLI, HTTP/gRPC, and internal Go use cases
- MVP primitives: `code.grep`, `code.read_file`, `code.symbols`, `code.references`, `code.ownership`
- Workspace-scoped task token required for agent calls
- Repo, revision, path, language, and protected-path authorization
- Commit-pinned or workspace-snapshot-pinned results
- Structured result contract with file paths, line ranges, content hashes, and source refs
- `tool_invocations`, `audit_events`, and `context_references` records for every relevant call
- Go-first symbol and reference extraction

Go focus:

- `internal/domain/codeintel`
- `internal/application/codeintel`
- `internal/service/codeintel`
- `internal/infra/ast`
- `internal/infra/git`
- `internal/infra/search`
- `internal/interfaces/mcp`
- `internal/interfaces/cli`
- `internal/interfaces/http`
- `internal/interfaces/grpc`
- `cmd/agenthub-mcp`
- `cmd/agenthub-cli`

Success criteria:

- Agents can retrieve code context through MCP and CLI without bypassing workspace scope.
- Every code context item used by an agent can be traced through `context_references`.
- Commit provenance can point back to the exact code facts the agent used.

## Milestone 4: Branch And Change Orchestration

Goal: establish an agent-specific change protocol.

Scope:

- Auto-create task branch
- Branch lease
- Commit provenance
- Change bundle
- Rollback or revert plan
- Stale branch detection
- Rebase queue

Go focus:

- `internal/domain/branch`
- `internal/domain/change`
- `internal/application/branches`
- `internal/application/changes`
- `internal/service/provenance`
- `internal/infra/git`

Success criteria:

- Every generated commit links back to prompt hash, context hash, tool logs, tests, and actor chain.

## Milestone 5: Execution Sandbox

Goal: standardize agent runtime.

Scope:

- Per-task container or devbox
- Network policy
- Secret policy
- Filesystem snapshot
- Reproducible command execution
- Artifact and log capture
- Tool call audit
- Long-running job resume

Go focus:

- `internal/domain/sandbox`
- `internal/application/sandboxes`
- `internal/infra/sandbox`
- `internal/infra/storage`
- `cmd/agenthub-runner`

Success criteria:

- Every command has policy, actor chain, stdout, stderr, exit code, artifacts, and audit events.

## Milestone 6: Human Approval Gates

Goal: make human control explicit and auditable.

Scope:

- Before push
- Before opening PR
- Before protected file modification
- Before privileged command
- Before merge
- Before secret access
- Before network access
- Before cross-repo change

Go focus:

- `internal/domain/approval`
- `internal/application/approvals`
- `internal/service/authorization`
- `internal/interfaces/http`

Success criteria:

- High-risk actions cannot proceed without a durable approval record.

## Milestone 7: Review For Agents

Goal: support human review of agents and agent review of agents.

Scope:

- Agent self-checklist
- Required evidence
- Tests run
- Files touched
- Risk summary
- Inline review threads assigned to agent
- Native "fix this thread"
- Approve intent but require patch regeneration
- Diff explanation tied to code ranges

Go focus:

- `internal/domain/review`
- `internal/application/reviews`
- `internal/service/risk`
- `internal/infra/github`

Success criteria:

- Reviewers can assess both patch content and agent evidence.

## Milestone 8: Durable Conversation And Memory

Goal: make task execution resumable and auditable.

Scope:

- User intent
- Planning notes
- Tool calls
- Code search results
- Failed attempts
- Test output
- Final reasoning
- Summaries for future tasks

Go focus:

- `internal/domain/audit`
- `internal/application/events`
- `internal/infra/storage`
- `internal/infra/search`

Success criteria:

- A stopped or crashed agent can resume from workspace state and logs.

## Milestone 9: Event Bus / Agent Hooks

Goal: make agent work event-driven.

Scope:

- Issue assigned to agent
- Review comment assigned to agent
- CI failed with logs
- Branch conflict appeared
- Dependency changed
- Human requested retry
- PR approved
- Incident linked

Go focus:

- `internal/domain/event`
- `internal/application/events`
- `internal/infra/queue`
- `internal/interfaces/webhook`
- `internal/interfaces/worker`
- `cmd/agenthub-worker`

Success criteria:

- Agent task queues receive structured events with retry, debounce, and dead-letter handling.

## Milestone 10: Semantic Code Context Expansion

Goal: expand code understanding beyond the MVP primitives without replacing source-of-truth systems.

Scope:

- Advanced `code.ast_query`
- Advanced `code.dependencies`
- Advanced `code.history`
- Advanced `code.diff_map`
- Advanced `code.test_discover`
- Advanced `code.semantic_search`
- Repo symbol index
- Dependency graph
- Ownership map
- Embedding search
- Issue, PR, and code cross references
- History summary for changed lines
- Repo, task, and org scoped memory
- `context_references` for code facts used by agents

Go focus:

- `internal/domain/codeintel`
- `internal/application/codeintel`
- `internal/service/codeintel`
- `internal/infra/ast`
- `internal/infra/search`
- `cmd/agenthub-indexer`

Success criteria:

- Semantic context always links back to Git, database records, logs, or artifacts.
- Advanced semantic results can be consumed through the same code intelligence primitive interfaces.
- Generated summaries never become independent facts.

## Milestone 11: Agent-Aware Checks

Goal: make checks produce structured, agent-readable remediation.

Scope:

- Risk classifier
- Touched ownership
- Missing tests detector
- Generated-code detector
- API compatibility check
- Migration safety check
- Security scan remediation
- Flaky test attribution

Go focus:

- `internal/domain/checks`
- `internal/application/checks`
- `internal/service/risk`
- `internal/interfaces/worker`

Success criteria:

- Failed checks can become structured tasks assigned back to agents.

## Milestone 12: Native Multi-Agent Collaboration

Goal: coordinate multiple agents in one workspace or change bundle.

Scope:

- Task decomposition
- File and area leases
- Conflict detection
- Agent-to-agent handoff
- Reviewer and implementer roles
- Coordinator agent
- Shared workspace state
- Merge ordering

Go focus:

- `internal/domain/workspace`
- `internal/domain/branch`
- `internal/application/workspaces`
- `internal/service/orchestration`

Success criteria:

- Multiple agents can collaborate without relying on informal branch or file conventions.

## Milestone 13: Cross-Repo Change Bundle

Goal: support coordinated changes across repositories.

Scope:

- Multi-repo workspace
- Linked branches
- Pull request group
- Ordered merge plan
- Rollback or revert plan
- Cross-repo approval gate
- Dependency-aware validation

Go focus:

- `internal/domain/change`
- `internal/application/changes`
- `internal/service/orchestration`
- `internal/infra/git`
- `internal/infra/github`

Success criteria:

- A single task can safely produce, review, approve, merge, and revert coordinated changes across repositories.

## Recommended MVP

Build the first usable version in this order:

1. Agent Identity And Capability Model
2. Workspace / Task Primitive
3. Code Intelligence Primitives MVP
4. Branch Lease And Commit Provenance
5. Execution Sandbox Command Runner
6. Human Approval Gates

This MVP already makes AgentHub meaningfully different from normal repository hosting: agents work inside controlled workspaces, retrieve trusted code context through audited primitives, use scoped capabilities, produce auditable changes, and cross explicit approval gates.
