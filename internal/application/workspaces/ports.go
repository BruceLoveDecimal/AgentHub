package workspaces

import (
	"context"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/workspace"
)

type WorkspaceStore interface {
	Save(ctx context.Context, workspace *workspace.Workspace) error
	Find(ctx context.Context, id string) (*workspace.Workspace, error)
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
