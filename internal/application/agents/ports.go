package agents

import (
	"context"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/agent"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/capability"
)

type AgentStore interface {
	Save(ctx context.Context, agent *agent.Agent) error
	Find(ctx context.Context, id string) (*agent.Agent, error)
}

type CapabilityGrantStore interface {
	Save(ctx context.Context, grant *capability.Grant) error
	ListForPrincipal(ctx context.Context, principal capability.Principal) ([]capability.Grant, error)
}

type TaskTokenStore interface {
	Save(ctx context.Context, token *agent.TaskToken) error
}

type AuditRecorder interface {
	Record(ctx context.Context, event *audit.Event) error
}

type IDGenerator interface {
	NewID() string
}

type TokenGenerator interface {
	NewToken(ctx context.Context) (plaintext string, hash string, err error)
}

type Clock interface {
	Now() time.Time
}
