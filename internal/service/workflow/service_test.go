package workflow

import (
	"strings"
	"testing"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/workspace"
)

func TestValidateReadyToRunRequiresRepoAndAgent(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	w := mustWorkspace(t, now)

	err := Service{}.ValidateReadyToRun(*w)
	if err == nil || !strings.Contains(err.Error(), "no bound repositories") {
		t.Fatalf("expected repo validation error, got %v", err)
	}

	if err := w.BindRepository(workspace.RepositoryBinding{
		RepoID:       "repo-1",
		Role:         workspace.RepositoryPrimary,
		BaseBranch:   "main",
		TargetBranch: "main",
	}, now); err != nil {
		t.Fatalf("bind repo: %v", err)
	}
	err = Service{}.ValidateReadyToRun(*w)
	if err == nil || !strings.Contains(err.Error(), "no assigned agents") {
		t.Fatalf("expected agent validation error, got %v", err)
	}
}

func TestDefaultTaskBranchName(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	w := mustWorkspace(t, now)

	got := Service{}.DefaultTaskBranchName(*w)
	if got != "agent/agh-1" {
		t.Fatalf("unexpected branch name: %q", got)
	}
}

func mustWorkspace(t *testing.T, now time.Time) *workspace.Workspace {
	t.Helper()
	w, err := workspace.New(workspace.Spec{
		ID:              "workspace-1",
		OrgID:           "org-1",
		Key:             "AGH-1",
		Title:           "Fix auth",
		Requirement:     workspace.Requirement{Kind: workspace.RequirementManual},
		CreatedByUserID: "user-1",
		CreatedAt:       now,
	})
	if err != nil {
		t.Fatalf("new workspace: %v", err)
	}
	return w
}
