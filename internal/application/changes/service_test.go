package changes

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/change"
)

func TestCreateBundleAndAddItem(t *testing.T) {
	ctx := context.Background()
	svc, stores := newTestService()
	chain := agentChain(t, stores.clock.now)

	bundle, err := svc.CreateBundle(ctx, CreateBundleCommand{
		OrgID:            "org-1",
		WorkspaceID:      "workspace-1",
		Title:            "Fix auth",
		CreatedByAgentID: "agent-1",
		ActorChain:       chain,
	})
	if err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	updated, err := svc.AddBundleItem(ctx, AddBundleItemCommand{
		BundleID:          bundle.ID,
		RepoID:            "repo-1",
		WorkspaceBranchID: "branch-1",
		ActorChain:        chain,
	})
	if err != nil {
		t.Fatalf("add bundle item: %v", err)
	}
	if len(updated.Items) != 1 {
		t.Fatalf("expected one bundle item, got %d", len(updated.Items))
	}
	if stores.audit.events[0].Type != audit.EventChangeBundleCreated {
		t.Fatalf("unexpected audit event: %s", stores.audit.events[0].Type)
	}
}

func TestRecordCommitWithProvenance(t *testing.T) {
	ctx := context.Background()
	svc, stores := newTestService()
	chain := agentChain(t, stores.clock.now)

	result, err := svc.RecordCommitWithProvenance(ctx, RecordCommitCommand{
		OrgID:            "org-1",
		RepoID:           "repo-1",
		WorkspaceID:      "workspace-1",
		BundleID:         "bundle-1",
		SHA:              "abc123",
		Message:          "fix auth",
		ParentSHAs:       []string{"parent"},
		AuthorAgentID:    "agent-1",
		ActorChain:       chain,
		PromptHash:       "sha256:p",
		ContextHash:      "sha256:c",
		DiffHash:         "sha256:d",
		ToolLogRefs:      []string{"tool-1"},
		TestEvidenceRefs: []string{"test-1"},
	})
	if err != nil {
		t.Fatalf("record commit: %v", err)
	}
	if result.Provenance.CommitID != result.Commit.ID {
		t.Fatalf("provenance commit mismatch")
	}
	if len(stores.commits.provenance) != 1 {
		t.Fatalf("expected provenance persisted")
	}
}

func TestGenerateRevertPlan(t *testing.T) {
	ctx := context.Background()
	svc, stores := newTestService()
	chain := agentChain(t, stores.clock.now)

	plan, err := svc.GenerateRevertPlan(ctx, GenerateRevertPlanCommand{
		OrgID:              "org-1",
		WorkspaceID:        "workspace-1",
		BundleID:           "bundle-1",
		CommitID:           "commit-1",
		CommitSHA:          "abc123",
		GeneratedByAgentID: "agent-1",
		ActorChain:         chain,
	})
	if err != nil {
		t.Fatalf("generate revert plan: %v", err)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("expected revert commit and bundle steps, got %+v", plan.Steps)
	}
	if stores.audit.events[len(stores.audit.events)-1].Type != audit.EventRevertPlanGenerated {
		t.Fatalf("expected revert plan audit event")
	}
}

type changeStores struct {
	bundles *memoryBundleStore
	commits *memoryCommitStore
	reverts *memoryRevertStore
	audit   *memoryAuditRecorder
	clock   *fixedClock
}

func newTestService() (Service, changeStores) {
	stores := changeStores{
		bundles: &memoryBundleStore{bundles: map[string]*change.Bundle{}},
		commits: &memoryCommitStore{},
		reverts: &memoryRevertStore{},
		audit:   &memoryAuditRecorder{},
		clock:   &fixedClock{now: time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)},
	}
	return Service{
		Bundles:     stores.bundles,
		Commits:     stores.commits,
		RevertPlans: stores.reverts,
		Audit:       stores.audit,
		IDs:         &sequenceIDs{},
		Clock:       stores.clock,
	}, stores
}

func agentChain(t *testing.T, now time.Time) audit.ActorChain {
	t.Helper()
	chain, err := audit.NewAgentActorChain("chain-1", "org-1", "workspace-1", "user-1", "agent-1", now)
	if err != nil {
		t.Fatalf("new chain: %v", err)
	}
	return chain
}

type memoryBundleStore struct{ bundles map[string]*change.Bundle }

func (s *memoryBundleStore) Save(_ context.Context, bundle *change.Bundle) error {
	s.bundles[bundle.ID] = bundle
	return nil
}

func (s *memoryBundleStore) Find(_ context.Context, id string) (*change.Bundle, error) {
	bundle := s.bundles[id]
	if bundle == nil {
		return nil, errors.New("bundle not found")
	}
	return bundle, nil
}

type memoryCommitStore struct {
	commits    []*change.Commit
	provenance []*change.Provenance
}

func (s *memoryCommitStore) SaveCommit(_ context.Context, commit *change.Commit) error {
	s.commits = append(s.commits, commit)
	return nil
}

func (s *memoryCommitStore) SaveProvenance(_ context.Context, provenance *change.Provenance) error {
	s.provenance = append(s.provenance, provenance)
	return nil
}

type memoryRevertStore struct{ plans []*change.RevertPlan }

func (s *memoryRevertStore) Save(_ context.Context, plan *change.RevertPlan) error {
	s.plans = append(s.plans, plan)
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
