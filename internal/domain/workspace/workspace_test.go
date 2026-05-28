package workspace

import (
	"strings"
	"testing"
	"time"
)

func TestNewWorkspaceValidatesRequirementAndKey(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)

	_, err := New(Spec{
		ID:              "workspace-1",
		OrgID:           "org-1",
		Key:             "bad-key",
		Title:           "Fix auth",
		Requirement:     Requirement{Kind: RequirementIssue, Ref: "ISSUE-1"},
		CreatedByUserID: "user-1",
		CreatedAt:       now,
	})
	if err == nil || !strings.Contains(err.Error(), "workspace key") {
		t.Fatalf("expected key validation error, got %v", err)
	}

	_, err = New(Spec{
		ID:              "workspace-1",
		OrgID:           "org-1",
		Key:             "AGH-1",
		Title:           "Fix auth",
		Requirement:     Requirement{Kind: RequirementIssue},
		CreatedByUserID: "user-1",
		CreatedAt:       now,
	})
	if err == nil || !strings.Contains(err.Error(), "requirement ref") {
		t.Fatalf("expected requirement ref validation error, got %v", err)
	}
}

func TestWorkspaceBindsSinglePrimaryRepository(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	w := mustWorkspace(t, now)

	if err := w.BindRepository(RepositoryBinding{
		RepoID:       "repo-1",
		Role:         RepositoryPrimary,
		BaseBranch:   "main",
		TargetBranch: "main",
	}, now); err != nil {
		t.Fatalf("bind primary repo: %v", err)
	}
	err := w.BindRepository(RepositoryBinding{
		RepoID:       "repo-2",
		Role:         RepositoryPrimary,
		BaseBranch:   "main",
		TargetBranch: "main",
	}, now)
	if err == nil || !strings.Contains(err.Error(), "one primary") {
		t.Fatalf("expected single primary validation error, got %v", err)
	}
}

func TestWorkspaceAssignAgentSetsPrimaryAgent(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	w := mustWorkspace(t, now)

	err := w.AssignAgent(AgentAssignment{
		AgentID:          "agent-1",
		Role:             AgentImplementer,
		AssignedByUserID: "user-1",
	}, now)
	if err != nil {
		t.Fatalf("assign agent: %v", err)
	}
	if w.PrimaryAgentID != "agent-1" {
		t.Fatalf("expected primary agent to be agent-1, got %q", w.PrimaryAgentID)
	}
	if w.Agents[0].Status != AgentAssigned {
		t.Fatalf("expected assigned status, got %s", w.Agents[0].Status)
	}
}

func TestWorkspacePlansBranchOnlyForBoundRepo(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	w := mustWorkspace(t, now)

	err := w.PlanBranch(Branch{
		ID:               "branch-1",
		RepoID:           "repo-1",
		Name:             "agent/agh-1",
		BaseBranch:       "main",
		BaseSHA:          "abc123",
		CreatedByAgentID: "agent-1",
	}, now)
	if err == nil || !strings.Contains(err.Error(), "not bound") {
		t.Fatalf("expected unbound repo validation error, got %v", err)
	}

	if err := w.BindRepository(RepositoryBinding{
		RepoID:       "repo-1",
		Role:         RepositoryPrimary,
		BaseBranch:   "main",
		TargetBranch: "main",
	}, now); err != nil {
		t.Fatalf("bind repo: %v", err)
	}
	if err := w.PlanBranch(Branch{
		ID:               "branch-1",
		RepoID:           "repo-1",
		Name:             "agent/agh-1",
		BaseBranch:       "main",
		BaseSHA:          "abc123",
		CreatedByAgentID: "agent-1",
	}, now); err != nil {
		t.Fatalf("plan branch: %v", err)
	}
	if w.Branches[0].Status != BranchPlanned {
		t.Fatalf("expected planned status, got %s", w.Branches[0].Status)
	}
}

func TestWorkspaceStatusTransitionsRejectClosedWorkspaceMutation(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	w := mustWorkspace(t, now)

	if err := w.Transition(StatusRunning, now.Add(time.Minute)); err != nil {
		t.Fatalf("transition running: %v", err)
	}
	if err := w.Transition(StatusCompleted, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("transition completed: %v", err)
	}
	if w.ClosedAt == nil {
		t.Fatal("expected closed_at to be set")
	}
	err := w.Transition(StatusRunning, now.Add(3*time.Minute))
	if err == nil || !strings.Contains(err.Error(), "cannot transition") {
		t.Fatalf("expected terminal transition error, got %v", err)
	}

	err = w.BindRepository(RepositoryBinding{
		RepoID:       "repo-1",
		Role:         RepositoryPrimary,
		BaseBranch:   "main",
		TargetBranch: "main",
	}, now.Add(3*time.Minute))
	if err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("expected closed workspace mutation error, got %v", err)
	}
}

func mustWorkspace(t *testing.T, now time.Time) *Workspace {
	t.Helper()
	w, err := New(Spec{
		ID:              "workspace-1",
		OrgID:           "org-1",
		Key:             "AGH-1",
		Title:           "Fix auth",
		Requirement:     Requirement{Kind: RequirementIssue, Ref: "https://example.test/issues/1"},
		CreatedByUserID: "user-1",
		CreatedAt:       now,
	})
	if err != nil {
		t.Fatalf("new workspace: %v", err)
	}
	return w
}
