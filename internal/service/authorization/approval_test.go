package authorization

import (
	"testing"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/approval"
)

func TestEnforceApprovalAllowsApprovedContext(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	gate := mustApprovalGate(t, now)
	if _, err := gate.Approve("decision-1", "user-1", "sha256:context", "ok", now); err != nil {
		t.Fatalf("approve: %v", err)
	}

	decision := Service{}.EnforceApproval(ApprovalRequest{Gate: *gate, ContextHash: "sha256:context", At: now})
	if !decision.Allowed || decision.Reason != "approved" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestEnforceApprovalDeniesPendingAndContextMismatch(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	gate := mustApprovalGate(t, now)

	decision := Service{}.EnforceApproval(ApprovalRequest{Gate: *gate, ContextHash: "sha256:context", At: now})
	if decision.Allowed || decision.Reason != "approval_pending" {
		t.Fatalf("unexpected pending decision: %+v", decision)
	}
	if _, err := gate.Approve("decision-1", "user-1", "sha256:context", "ok", now); err != nil {
		t.Fatalf("approve: %v", err)
	}
	decision = Service{}.EnforceApproval(ApprovalRequest{Gate: *gate, ContextHash: "sha256:other", At: now})
	if decision.Allowed || decision.Reason != "context_mismatch" {
		t.Fatalf("unexpected mismatch decision: %+v", decision)
	}
}

func mustApprovalGate(t *testing.T, now time.Time) *approval.Gate {
	t.Helper()
	gate, err := approval.NewGate(approval.GateSpec{
		ID:                 "gate-1",
		OrgID:              "org-1",
		WorkspaceID:        "workspace-1",
		Type:               approval.GateNetwork,
		Action:             "access network",
		ResourceKind:       approval.ResourceCommand,
		ResourceID:         "curl",
		RequestedByAgentID: "agent-1",
		ContextHash:        "sha256:context",
		CreatedAt:          now,
	})
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}
	return gate
}
