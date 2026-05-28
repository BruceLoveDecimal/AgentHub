package changes

import (
	"context"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/change"
)

type BundleStore interface {
	Save(ctx context.Context, bundle *change.Bundle) error
	Find(ctx context.Context, id string) (*change.Bundle, error)
}

type CommitStore interface {
	SaveCommit(ctx context.Context, commit *change.Commit) error
	SaveProvenance(ctx context.Context, provenance *change.Provenance) error
}

type RevertPlanStore interface {
	Save(ctx context.Context, plan *change.RevertPlan) error
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
