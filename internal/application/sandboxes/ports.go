package sandboxes

import (
	"context"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/sandbox"
)

type PolicyStore interface {
	Save(ctx context.Context, policy *sandbox.Policy) error
	Find(ctx context.Context, id string) (*sandbox.Policy, error)
}

type InstanceStore interface {
	Save(ctx context.Context, instance *sandbox.Instance) error
	Find(ctx context.Context, id string) (*sandbox.Instance, error)
}

type ToolInvocationStore interface {
	Save(ctx context.Context, invocation *sandbox.ToolInvocation) error
}

type CommandRunStore interface {
	Save(ctx context.Context, run *sandbox.CommandRun) error
}

type ArtifactStore interface {
	Save(ctx context.Context, artifact *sandbox.Artifact) error
}

type CommandExecutor interface {
	Execute(ctx context.Context, req sandbox.CommandRequest) (CommandResult, error)
}

type CommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

type AuditRecorder interface {
	Record(ctx context.Context, event *audit.Event) error
}

type IDGenerator interface {
	NewID() string
}

type Clock interface {
	Now() time.Time
}
