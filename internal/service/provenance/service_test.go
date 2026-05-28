package provenance

import (
	"testing"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
)

func TestBuildRequiresAgentActor(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	chain, err := audit.NewAgentActorChain("chain-1", "org-1", "workspace-1", "user-1", "agent-1", now)
	if err != nil {
		t.Fatalf("new chain: %v", err)
	}
	prov, err := Service{}.Build(BuildCommand{
		CommitID:         "commit-1",
		WorkspaceID:      "workspace-1",
		ActorChain:       chain,
		PromptHash:       "sha256:p",
		ContextHash:      "sha256:c",
		DiffHash:         "sha256:d",
		ToolLogRefs:      []string{"tool-1"},
		TestEvidenceRefs: []string{"test-1"},
		CreatedAt:        now,
	})
	if err != nil {
		t.Fatalf("build provenance: %v", err)
	}
	if prov.ActorChainID != "chain-1" {
		t.Fatalf("unexpected actor chain id: %s", prov.ActorChainID)
	}
}
