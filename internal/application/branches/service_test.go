package branches

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/branch"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/workspace"
	"github.com/BruceLoveDecimal/AgentHub/internal/service/orchestration"
)

func TestPlanTaskBranchAcquiresLeaseAndPlansWorkspaceBranch(t *testing.T) {
	ctx := context.Background()
	svc, stores := newTestService()
	w := stores.workspace

	result, err := svc.PlanTaskBranch(ctx, PlanTaskBranchCommand{
		WorkspaceID:       w.ID,
		RepoID:            "repo-1",
		BaseBranch:        "main",
		BaseSHA:           "abc123",
		CreatedByAgentID:  "agent-1",
		RequestedByUserID: "user-1",
	})
	if err != nil {
		t.Fatalf("plan task branch: %v", err)
	}
	if result.Branch.Name != "agent/agh-1" {
		t.Fatalf("unexpected branch name: %q", result.Branch.Name)
	}
	if len(stores.leases.leases) != 1 {
		t.Fatalf("expected one lease, got %d", len(stores.leases.leases))
	}
	if stores.audit.events[0].Type != audit.EventBranchLeaseAcquired {
		t.Fatalf("unexpected audit event: %s", stores.audit.events[0].Type)
	}
}

func TestPlanTaskBranchRejectsActiveLease(t *testing.T) {
	ctx := context.Background()
	svc, stores := newTestService()
	now := stores.clock.now
	lease, err := branch.NewLease(branch.LeaseSpec{
		ID:            "lease-1",
		OrgID:         "org-1",
		WorkspaceID:   "workspace-1",
		RepoID:        "repo-1",
		BranchName:    "agent/agh-1",
		HolderAgentID: "agent-1",
		FencingToken:  1,
		TTL:           time.Hour,
		Now:           now,
	})
	if err != nil {
		t.Fatalf("new lease: %v", err)
	}
	stores.leases.leases = append(stores.leases.leases, lease)

	_, err = svc.PlanTaskBranch(ctx, PlanTaskBranchCommand{
		WorkspaceID:      "workspace-1",
		RepoID:           "repo-1",
		BaseBranch:       "main",
		BaseSHA:          "abc123",
		CreatedByAgentID: "agent-1",
	})
	if err == nil {
		t.Fatal("expected active lease error")
	}
}

func TestDetectStaleBranchQueuesRebase(t *testing.T) {
	ctx := context.Background()
	svc, stores := newTestService()
	_, err := svc.PlanTaskBranch(ctx, PlanTaskBranchCommand{
		WorkspaceID:      "workspace-1",
		RepoID:           "repo-1",
		BaseBranch:       "main",
		BaseSHA:          "old",
		CreatedByAgentID: "agent-1",
	})
	if err != nil {
		t.Fatalf("plan task branch: %v", err)
	}

	stale, err := svc.DetectStaleBranch(ctx, DetectStaleBranchCommand{
		WorkspaceID:    "workspace-1",
		RepoID:         "repo-1",
		BranchName:     "agent/agh-1",
		CurrentBaseSHA: "new",
	})
	if err != nil {
		t.Fatalf("detect stale: %v", err)
	}
	if !stale || len(stores.rebase.items) != 1 {
		t.Fatalf("expected stale branch rebase item, stale=%v items=%d", stale, len(stores.rebase.items))
	}
}

type branchStores struct {
	workspace  *workspace.Workspace
	workspaces *memoryWorkspaceStore
	leases     *memoryLeaseStore
	rebase     *memoryRebaseQueue
	audit      *memoryAuditRecorder
	clock      *fixedClock
}

func newTestService() (Service, branchStores) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
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
		panic(err)
	}
	if err := w.BindRepository(workspace.RepositoryBinding{
		RepoID:       "repo-1",
		Role:         workspace.RepositoryPrimary,
		BaseBranch:   "main",
		TargetBranch: "main",
	}, now); err != nil {
		panic(err)
	}
	stores := branchStores{
		workspace:  w,
		workspaces: &memoryWorkspaceStore{workspace: w},
		leases:     &memoryLeaseStore{},
		rebase:     &memoryRebaseQueue{},
		audit:      &memoryAuditRecorder{},
		clock:      &fixedClock{now: now},
	}
	return Service{
		Workspaces: stores.workspaces,
		Leases:     stores.leases,
		Rebase:     stores.rebase,
		Audit:      stores.audit,
		IDs:        &sequenceIDs{},
		Clock:      stores.clock,
	}, stores
}

type memoryWorkspaceStore struct{ workspace *workspace.Workspace }

func (s *memoryWorkspaceStore) Save(_ context.Context, w *workspace.Workspace) error {
	s.workspace = w
	return nil
}

func (s *memoryWorkspaceStore) Find(context.Context, string) (*workspace.Workspace, error) {
	if s.workspace == nil {
		return nil, errors.New("workspace not found")
	}
	return s.workspace, nil
}

type memoryLeaseStore struct {
	leases []*branch.Lease
	next   int64
}

func (s *memoryLeaseStore) Save(_ context.Context, lease *branch.Lease) error {
	for i := range s.leases {
		if s.leases[i].ID == lease.ID {
			s.leases[i] = lease
			return nil
		}
	}
	s.leases = append(s.leases, lease)
	return nil
}

func (s *memoryLeaseStore) FindActive(_ context.Context, repoID, branchName string) (*branch.Lease, error) {
	for _, lease := range s.leases {
		if lease.RepoID == repoID && lease.BranchName == branchName && lease.Status == branch.LeaseActive {
			return lease, nil
		}
	}
	return nil, nil
}

func (s *memoryLeaseStore) NextFencingToken(context.Context, string, string) (int64, error) {
	s.next++
	return s.next, nil
}

type memoryRebaseQueue struct {
	items []orchestration.RebaseQueueItem
}

func (q *memoryRebaseQueue) Enqueue(_ context.Context, item orchestration.RebaseQueueItem) error {
	q.items = append(q.items, item)
	return nil
}

type memoryAuditRecorder struct{ events []*audit.Event }

func (r *memoryAuditRecorder) Record(_ context.Context, event *audit.Event) error {
	r.events = append(r.events, event)
	return nil
}

type sequenceIDs struct{ next int }

func (g *sequenceIDs) NewID() string {
	g.next++
	return "id-" + string(rune('0'+g.next))
}

type fixedClock struct{ now time.Time }

func (c *fixedClock) Now() time.Time { return c.now }
