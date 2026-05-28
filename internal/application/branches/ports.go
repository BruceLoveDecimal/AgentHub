package branches

import (
	"context"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/branch"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/workspace"
	"github.com/BruceLoveDecimal/AgentHub/internal/service/orchestration"
)

type WorkspaceStore interface {
	Save(ctx context.Context, workspace *workspace.Workspace) error
	Find(ctx context.Context, id string) (*workspace.Workspace, error)
}

type LeaseStore interface {
	Save(ctx context.Context, lease *branch.Lease) error
	FindActive(ctx context.Context, repoID, branchName string) (*branch.Lease, error)
	NextFencingToken(ctx context.Context, repoID, branchName string) (int64, error)
}

type RebaseQueue interface {
	Enqueue(ctx context.Context, item orchestration.RebaseQueueItem) error
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
