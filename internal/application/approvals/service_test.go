package approvals

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/approval"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
)

func TestRequestApproveAndConsumeGate(t *testing.T) {
	ctx := context.Background()
	svc, stores := newTestService()

	gate, err := svc.RequestGate(ctx, RequestGateCommand{
		OrgID:              "org-1",
		WorkspaceID:        "workspace-1",
		Type:               approval.GateProtectedPath,
		Action:             "modify protected file",
		ResourceKind:       approval.ResourcePath,
		ResourceID:         "migrations/001.sql",
		RequestedByAgentID: "agent-1",
		ContextHash:        "sha256:context",
		DiffHash:           "sha256:diff",
		ExpiresIn:          time.Hour,
	})
	if err != nil {
		t.Fatalf("request gate: %v", err)
	}
	if stores.audit.events[0].Type != audit.EventApprovalRequested {
		t.Fatalf("unexpected audit event: %s", stores.audit.events[0].Type)
	}

	updated, decision, err := svc.DecideGate(ctx, DecideGateCommand{
		GateID:          gate.ID,
		Decision:        approval.DecisionApproved,
		DecidedByUserID: "user-1",
		ContextHash:     "sha256:context",
		Comment:         "approved",
	})
	if err != nil {
		t.Fatalf("decide gate: %v", err)
	}
	if updated.Status != approval.StatusApproved || decision.Decision != approval.DecisionApproved {
		t.Fatalf("unexpected decision: gate=%s decision=%s", updated.Status, decision.Decision)
	}

	enforced, err := svc.EnforceGate(ctx, EnforceGateCommand{
		GateID:      gate.ID,
		ContextHash: "sha256:context",
		Consume:     true,
	})
	if err != nil {
		t.Fatalf("enforce gate: %v", err)
	}
	if !enforced.Allowed {
		t.Fatalf("expected allowed enforcement: %+v", enforced)
	}
	if stores.gates.gates[gate.ID].Status != approval.StatusUsed {
		t.Fatalf("expected used gate, got %s", stores.gates.gates[gate.ID].Status)
	}
}

func TestDeniedGateBlocksEnforcement(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService()
	gate, err := svc.RequestGate(ctx, RequestGateCommand{
		OrgID:              "org-1",
		WorkspaceID:        "workspace-1",
		Type:               approval.GateNetwork,
		Action:             "access network",
		ResourceKind:       approval.ResourceCommand,
		ResourceID:         "curl",
		RequestedByAgentID: "agent-1",
		ContextHash:        "sha256:context",
	})
	if err != nil {
		t.Fatalf("request gate: %v", err)
	}
	if _, _, err := svc.DecideGate(ctx, DecideGateCommand{
		GateID:          gate.ID,
		Decision:        approval.DecisionDenied,
		DecidedByUserID: "user-1",
		Comment:         "no",
	}); err != nil {
		t.Fatalf("deny gate: %v", err)
	}
	enforced, err := svc.EnforceGate(ctx, EnforceGateCommand{GateID: gate.ID, ContextHash: "sha256:context"})
	if err != nil {
		t.Fatalf("enforce gate: %v", err)
	}
	if enforced.Allowed || enforced.Reason != string(approval.StatusDenied) {
		t.Fatalf("unexpected enforcement: %+v", enforced)
	}
}

type testStores struct {
	gates *memoryGateStore
	audit *memoryAuditRecorder
	clock *fixedClock
}

func newTestService() (Service, testStores) {
	stores := testStores{
		gates: &memoryGateStore{gates: map[string]*approval.Gate{}},
		audit: &memoryAuditRecorder{},
		clock: &fixedClock{now: time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)},
	}
	return Service{
		Gates: stores.gates,
		Audit: stores.audit,
		IDs:   &sequenceIDs{},
		Clock: stores.clock,
	}, stores
}

type memoryGateStore struct{ gates map[string]*approval.Gate }

func (s *memoryGateStore) Save(_ context.Context, gate *approval.Gate) error {
	s.gates[gate.ID] = gate
	return nil
}

func (s *memoryGateStore) Find(_ context.Context, id string) (*approval.Gate, error) {
	gate := s.gates[id]
	if gate == nil {
		return nil, errors.New("gate not found")
	}
	return gate, nil
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
