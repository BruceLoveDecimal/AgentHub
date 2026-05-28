package branch

import (
	"strings"
	"testing"
	"time"
)

func TestLeaseLifecycle(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	lease, err := NewLease(LeaseSpec{
		ID:            "lease-1",
		OrgID:         "org-1",
		WorkspaceID:   "workspace-1",
		RepoID:        "repo-1",
		BranchName:    "agent/agh-1",
		HolderAgentID: "agent-1",
		FencingToken:  1,
		TTL:           time.Minute,
		Now:           now,
	})
	if err != nil {
		t.Fatalf("new lease: %v", err)
	}
	if !lease.IsActive(now.Add(30 * time.Second)) {
		t.Fatal("expected active lease before expiry")
	}
	if err := lease.Release(now.Add(40*time.Second), "agent-2"); err == nil || !strings.Contains(err.Error(), "holder") {
		t.Fatalf("expected holder validation error, got %v", err)
	}
	if err := lease.Release(now.Add(40*time.Second), "agent-1"); err != nil {
		t.Fatalf("release lease: %v", err)
	}
	if lease.Status != LeaseReleased {
		t.Fatalf("expected released status, got %s", lease.Status)
	}
}
