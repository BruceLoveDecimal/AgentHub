package orchestration

import (
	"testing"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/workspace"
)

func TestQueueRebaseForStaleBranch(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	branch := workspace.Branch{
		RepoID:     "repo-1",
		Name:       "agent/agh-1",
		BaseBranch: "main",
		BaseSHA:    "old",
	}

	item, err := Service{}.QueueRebase("workspace-1", branch, "new", now)
	if err != nil {
		t.Fatalf("queue rebase: %v", err)
	}
	if item.OldBaseSHA != "old" || item.NewBaseSHA != "new" {
		t.Fatalf("unexpected rebase item: %+v", item)
	}
}
