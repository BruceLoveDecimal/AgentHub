package approvals

import (
	"context"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/approval"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
)

type GateStore interface {
	Save(ctx context.Context, gate *approval.Gate) error
	Find(ctx context.Context, id string) (*approval.Gate, error)
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
