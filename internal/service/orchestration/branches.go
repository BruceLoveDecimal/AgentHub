package orchestration

import (
	"fmt"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/workspace"
)

type BranchState struct {
	BaseSHA string
	HeadSHA string
}

type RebaseQueueItem struct {
	WorkspaceID string
	RepoID      string
	BranchName  string
	OldBaseSHA  string
	NewBaseSHA  string
	Reason      string
	QueuedAt    time.Time
}

type Service struct{}

func (Service) IsStale(planned workspace.Branch, currentBaseSHA string) bool {
	return planned.BaseSHA != "" && currentBaseSHA != "" && planned.BaseSHA != currentBaseSHA
}

func (s Service) QueueRebase(workspaceID string, planned workspace.Branch, currentBaseSHA string, now time.Time) (RebaseQueueItem, error) {
	if !s.IsStale(planned, currentBaseSHA) {
		return RebaseQueueItem{}, fmt.Errorf("branch %s is not stale", planned.Name)
	}
	return RebaseQueueItem{
		WorkspaceID: workspaceID,
		RepoID:      planned.RepoID,
		BranchName:  planned.Name,
		OldBaseSHA:  planned.BaseSHA,
		NewBaseSHA:  currentBaseSHA,
		Reason:      "base branch advanced",
		QueuedAt:    now,
	}, nil
}
