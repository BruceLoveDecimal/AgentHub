package search

import (
	"context"
	"testing"
)

func TestOwnershipIndexResolvesRules(t *testing.T) {
	index := OwnershipIndex{Rules: []OwnershipRule{
		{
			RepoID:           "repo-1",
			PathGlob:         "internal/**",
			Owners:           []string{"team/backend"},
			Protected:        true,
			RequiresApproval: true,
		},
	}}

	result, err := index.ResolveOwnership(context.Background(), "repo-1", "internal/auth.go")
	if err != nil {
		t.Fatalf("resolve ownership: %v", err)
	}
	if len(result.Owners) != 1 || result.Owners[0] != "team/backend" {
		t.Fatalf("unexpected owners: %+v", result.Owners)
	}
	if !result.Protected || !result.RequiresApproval {
		t.Fatalf("expected protected ownership result: %+v", result)
	}
}
