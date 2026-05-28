package change

import (
	"strings"
	"testing"
	"time"
)

func TestBundleAddsItemsOnlyInDraft(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	bundle, err := NewBundle(BundleSpec{
		ID:               "bundle-1",
		OrgID:            "org-1",
		WorkspaceID:      "workspace-1",
		Title:            "Fix auth",
		CreatedByAgentID: "agent-1",
		CreatedAt:        now,
	})
	if err != nil {
		t.Fatalf("new bundle: %v", err)
	}
	if err := bundle.AddItem(BundleItem{ID: "item-1", RepoID: "repo-1", WorkspaceBranchID: "branch-1"}, now); err != nil {
		t.Fatalf("add item: %v", err)
	}
	if bundle.Items[0].MergeOrder != 1 {
		t.Fatalf("expected default merge order 1, got %d", bundle.Items[0].MergeOrder)
	}
	bundle.Status = BundleReadyForReview
	err = bundle.AddItem(BundleItem{ID: "item-2", RepoID: "repo-2", WorkspaceBranchID: "branch-2"}, now)
	if err == nil || !strings.Contains(err.Error(), "cannot add") {
		t.Fatalf("expected non-draft add error, got %v", err)
	}
}

func TestProvenanceRequiresEvidence(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	_, err := NewProvenance(Provenance{
		CommitID:     "commit-1",
		WorkspaceID:  "workspace-1",
		ActorChainID: "chain-1",
		PromptHash:   "sha256:p",
		ContextHash:  "sha256:c",
		DiffHash:     "sha256:d",
		CreatedAt:    now,
	})
	if err == nil || !strings.Contains(err.Error(), "tool log refs") {
		t.Fatalf("expected tool evidence validation error, got %v", err)
	}
}
