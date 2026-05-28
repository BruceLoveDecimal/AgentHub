package sandbox

import (
	"strings"
	"testing"
	"time"
)

func TestPolicyDeniesPrivilegedNetworkAndSecretByDefault(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	policy, err := NewPolicy(Policy{
		ID:              "policy-1",
		OrgID:           "org-1",
		Name:            "default",
		WritablePaths:   []string{"workspace"},
		CreatedByUserID: "user-1",
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	if err != nil {
		t.Fatalf("new policy: %v", err)
	}
	err = policy.Allows(CommandRequest{Command: "go", NetworkRequired: true})
	if err == nil || !strings.Contains(err.Error(), "network") {
		t.Fatalf("expected network denial, got %v", err)
	}
	err = policy.Allows(CommandRequest{Command: "cat", SecretsRequired: true})
	if err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("expected secret denial, got %v", err)
	}
	err = policy.Allows(CommandRequest{Command: "sudo", Privileged: true})
	if err == nil || !strings.Contains(err.Error(), "privileged") {
		t.Fatalf("expected privileged denial, got %v", err)
	}
}

func TestInstanceMustBeRunningForCommands(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	instance, err := NewInstance(Instance{
		ID:          "sandbox-1",
		WorkspaceID: "workspace-1",
		PolicyID:    "policy-1",
		RunnerID:    "runner-1",
		CreatedAt:   now,
	})
	if err != nil {
		t.Fatalf("new instance: %v", err)
	}
	if err := instance.CanRunCommand(); err == nil {
		t.Fatal("expected non-running instance error")
	}
	if err := instance.Start(now); err != nil {
		t.Fatalf("start instance: %v", err)
	}
	if err := instance.CanRunCommand(); err != nil {
		t.Fatalf("running instance should accept command: %v", err)
	}
}
