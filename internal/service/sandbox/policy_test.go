package sandbox

import (
	"testing"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/sandbox"
)

func TestValidateCommandAppliesPolicyAndInstanceState(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	policy, err := sandbox.NewPolicy(sandbox.Policy{
		ID:              "policy-1",
		OrgID:           "org-1",
		Name:            "default",
		WritablePaths:   []string{"."},
		CreatedByUserID: "user-1",
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	if err != nil {
		t.Fatalf("new policy: %v", err)
	}
	instance, err := sandbox.NewInstance(sandbox.Instance{
		ID:          "sandbox-1",
		WorkspaceID: "workspace-1",
		PolicyID:    "policy-1",
		RunnerID:    "runner-1",
		CreatedAt:   now,
	})
	if err != nil {
		t.Fatalf("new instance: %v", err)
	}
	if err := instance.Start(now); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := (Service{}).ValidateCommand(*policy, *instance, sandbox.CommandRequest{Command: "go", Args: []string{"test"}, WritePath: "."}); err != nil {
		t.Fatalf("validate command: %v", err)
	}
}
