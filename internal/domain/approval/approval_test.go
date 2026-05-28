package approval

import (
	"strings"
	"testing"
	"time"
)

func TestApproveRequiresMatchingContextHash(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	gate := mustGate(t, now)

	_, err := gate.Approve("decision-1", "user-1", "sha256:other", "ok", now)
	if err == nil || !strings.Contains(err.Error(), "context hash") {
		t.Fatalf("expected context mismatch, got %v", err)
	}

	decision, err := gate.Approve("decision-1", "user-1", "sha256:context", "ok", now)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if gate.Status != StatusApproved || decision.Decision != DecisionApproved {
		t.Fatalf("unexpected approval state: gate=%s decision=%s", gate.Status, decision.Decision)
	}
}

func TestGateCannotApproveAfterExpiry(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Minute)
	gate := mustGate(t, now)
	gate.ExpiresAt = &expiresAt

	_, err := gate.Approve("decision-1", "user-1", "sha256:context", "ok", now.Add(time.Minute))
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired approval error, got %v", err)
	}
	if gate.Status != StatusExpired {
		t.Fatalf("expected expired status, got %s", gate.Status)
	}
}

func TestMarkUsedConsumesApprovedGate(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	gate := mustGate(t, now)
	if _, err := gate.Approve("decision-1", "user-1", "sha256:context", "ok", now); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if err := gate.MarkUsed("sha256:context", now.Add(time.Second)); err != nil {
		t.Fatalf("mark used: %v", err)
	}
	if gate.Status != StatusUsed {
		t.Fatalf("expected used gate, got %s", gate.Status)
	}
}

func mustGate(t *testing.T, now time.Time) *Gate {
	t.Helper()
	gate, err := NewGate(GateSpec{
		ID:                 "gate-1",
		OrgID:              "org-1",
		WorkspaceID:        "workspace-1",
		Type:               GateProtectedPath,
		Action:             "modify protected file",
		ResourceKind:       ResourcePath,
		ResourceID:         "migrations/001.sql",
		RequestedByAgentID: "agent-1",
		ContextHash:        "sha256:context",
		DiffHash:           "sha256:diff",
		CreatedAt:          now,
	})
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}
	return gate
}
