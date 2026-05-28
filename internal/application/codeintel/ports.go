package codeintel

import (
	"context"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/capability"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/codeintel"
)

type RepositoryReader interface {
	ListFiles(ctx context.Context, repoID, revision string) ([]string, error)
	ReadFile(ctx context.Context, repoID, revision, path string) (string, error)
}

type SymbolIndex interface {
	FindSymbols(ctx context.Context, repoID, revision, query string) ([]codeintel.Result, error)
	FindReferences(ctx context.Context, repoID, revision, symbol string) ([]codeintel.Result, error)
}

type OwnershipIndex interface {
	ResolveOwnership(ctx context.Context, repoID, path string) (codeintel.Result, error)
}

type WorkspaceScope interface {
	IsRepoBound(ctx context.Context, workspaceID, repoID string) (bool, error)
}

type CapabilityGrantStore interface {
	ListForPrincipal(ctx context.Context, principal capability.Principal) ([]capability.Grant, error)
}

type ContextReferenceStore interface {
	Save(ctx context.Context, ref *codeintel.ContextReference) error
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
