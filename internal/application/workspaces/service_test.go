package workspaces

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/workspace"
)

func TestCreateWorkspacePersistsBindingsAndAuditEvent(t *testing.T) {
	ctx := context.Background()
	svc, stores := newTestService()

	created, err := svc.CreateWorkspace(ctx, CreateWorkspaceCommand{
		OrgID:           "org-1",
		Key:             "AGH-1",
		Title:           "Fix auth",
		Description:     "Fix auth bug",
		Requirement:     workspace.Requirement{Kind: workspace.RequirementIssue, Ref: "ISSUE-1"},
		CreatedByUserID: "user-1",
		Repositories: []BindRepositoryInput{
			{RepoID: "repo-1", Role: workspace.RepositoryPrimary, BaseBranch: "main", TargetBranch: "main"},
		},
		Agents: []AssignAgentInput{
			{AgentID: "agent-1", Role: workspace.AgentImplementer},
		},
	})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if stores.workspaces.saved[created.ID] == nil {
		t.Fatal("expected workspace to be persisted")
	}
	if len(created.Repositories) != 1 {
		t.Fatalf("expected one repo binding, got %d", len(created.Repositories))
	}
	if created.PrimaryAgentID != "agent-1" {
		t.Fatalf("expected primary agent assignment, got %q", created.PrimaryAgentID)
	}
	if len(stores.audit.events) != 1 {
		t.Fatalf("expected one audit event, got %d", len(stores.audit.events))
	}
	event := stores.audit.events[0]
	if event.Type != audit.EventWorkspaceCreated {
		t.Fatalf("unexpected audit event type: %s", event.Type)
	}
	if event.WorkspaceID != created.ID {
		t.Fatalf("unexpected audit workspace id: %s", event.WorkspaceID)
	}
}

func TestPlanBranchUsesDefaultAgentBranchName(t *testing.T) {
	ctx := context.Background()
	svc, stores := newTestService()
	w := mustStoredWorkspace(t, stores, workspace.StatusCreated)
	if err := w.BindRepository(workspace.RepositoryBinding{
		RepoID:       "repo-1",
		Role:         workspace.RepositoryPrimary,
		BaseBranch:   "main",
		TargetBranch: "main",
	}, stores.clock.now); err != nil {
		t.Fatalf("bind repo: %v", err)
	}

	updated, err := svc.PlanBranch(ctx, PlanBranchCommand{
		WorkspaceID:      w.ID,
		RepoID:           "repo-1",
		BaseBranch:       "main",
		BaseSHA:          "abc123",
		CreatedByAgentID: "agent-1",
		RequestedByID:    "user-1",
	})
	if err != nil {
		t.Fatalf("plan branch: %v", err)
	}
	if len(updated.Branches) != 1 {
		t.Fatalf("expected one planned branch, got %d", len(updated.Branches))
	}
	if updated.Branches[0].Name != "agent/agh-1" {
		t.Fatalf("unexpected branch name: %q", updated.Branches[0].Name)
	}
	if stores.audit.events[len(stores.audit.events)-1].Type != audit.EventWorkspaceBranchPlanned {
		t.Fatalf("expected branch planned audit event")
	}
}

func TestStartWorkspaceRequiresRepoAndAgent(t *testing.T) {
	ctx := context.Background()
	svc, stores := newTestService()
	w := mustStoredWorkspace(t, stores, workspace.StatusCreated)

	_, err := svc.StartWorkspace(ctx, StartWorkspaceCommand{
		WorkspaceID:   w.ID,
		RequestedByID: "user-1",
	})
	if err == nil {
		t.Fatal("expected ready-to-run validation error")
	}

	if err := w.BindRepository(workspace.RepositoryBinding{
		RepoID:       "repo-1",
		Role:         workspace.RepositoryPrimary,
		BaseBranch:   "main",
		TargetBranch: "main",
	}, stores.clock.now); err != nil {
		t.Fatalf("bind repo: %v", err)
	}
	if err := w.AssignAgent(workspace.AgentAssignment{
		AgentID:          "agent-1",
		Role:             workspace.AgentImplementer,
		AssignedByUserID: "user-1",
	}, stores.clock.now); err != nil {
		t.Fatalf("assign agent: %v", err)
	}

	updated, err := svc.StartWorkspace(ctx, StartWorkspaceCommand{
		WorkspaceID:   w.ID,
		RequestedByID: "user-1",
	})
	if err != nil {
		t.Fatalf("start workspace: %v", err)
	}
	if updated.Status != workspace.StatusRunning {
		t.Fatalf("expected running workspace, got %s", updated.Status)
	}
	if stores.audit.events[len(stores.audit.events)-1].Type != audit.EventWorkspaceStatusChanged {
		t.Fatalf("expected status changed audit event")
	}
}

type testStores struct {
	workspaces *memoryWorkspaceStore
	audit      *memoryAuditRecorder
	clock      *fixedClock
}

func newTestService() (Service, testStores) {
	stores := testStores{
		workspaces: &memoryWorkspaceStore{saved: map[string]*workspace.Workspace{}},
		audit:      &memoryAuditRecorder{},
		clock:      &fixedClock{now: time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)},
	}
	return Service{
		Workspaces: stores.workspaces,
		Audit:      stores.audit,
		IDs:        &sequenceIDs{},
		Clock:      stores.clock,
	}, stores
}

func mustStoredWorkspace(t *testing.T, stores testStores, status workspace.Status) *workspace.Workspace {
	t.Helper()
	w, err := workspace.New(workspace.Spec{
		ID:              "workspace-1",
		OrgID:           "org-1",
		Key:             "AGH-1",
		Title:           "Fix auth",
		Requirement:     workspace.Requirement{Kind: workspace.RequirementManual},
		CreatedByUserID: "user-1",
		CreatedAt:       stores.clock.now,
	})
	if err != nil {
		t.Fatalf("new workspace: %v", err)
	}
	w.Status = status
	stores.workspaces.saved[w.ID] = w
	return w
}

type memoryWorkspaceStore struct {
	saved map[string]*workspace.Workspace
}

func (s *memoryWorkspaceStore) Save(_ context.Context, value *workspace.Workspace) error {
	s.saved[value.ID] = value
	return nil
}

func (s *memoryWorkspaceStore) Find(_ context.Context, id string) (*workspace.Workspace, error) {
	value := s.saved[id]
	if value == nil {
		return nil, errors.New("workspace not found")
	}
	return value, nil
}

type memoryAuditRecorder struct {
	events []*audit.Event
}

func (r *memoryAuditRecorder) Record(_ context.Context, event *audit.Event) error {
	r.events = append(r.events, event)
	return nil
}

type sequenceIDs struct {
	next int
}

func (g *sequenceIDs) NewID() string {
	g.next++
	return "id-" + string(rune('0'+g.next))
}

type fixedClock struct {
	now time.Time
}

func (c *fixedClock) Now() time.Time {
	return c.now
}
