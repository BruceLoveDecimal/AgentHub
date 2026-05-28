package capability

import (
	"testing"
	"time"
)

func TestGrantMatchesRepoPathAndBranchScope(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	grant, err := NewGrant(GrantSpec{
		ID:              "grant-1",
		OrgID:           "org-1",
		Principal:       Principal{Kind: PrincipalAgent, ID: "agent-1"},
		Capability:      BranchPushAgentPattern,
		Effect:          EffectAllow,
		ResourceKind:    ResourceBranch,
		ResourceID:      "branch-1",
		RepoID:          "repo-1",
		PathGlob:        "internal/**",
		BranchPattern:   "agent/*",
		CreatedByUserID: "user-1",
		CreatedAt:       now,
	})
	if err != nil {
		t.Fatalf("new grant: %v", err)
	}

	matches := grant.Matches(
		Principal{Kind: PrincipalAgent, ID: "agent-1"},
		BranchPushAgentPattern,
		Resource{
			OrgID:  "org-1",
			Kind:   ResourceBranch,
			ID:     "branch-1",
			RepoID: "repo-1",
			Path:   "internal/service/auth.go",
			Branch: "agent/workspace-1",
		},
		now,
	)
	if !matches {
		t.Fatal("expected scoped grant to match")
	}

	matches = grant.Matches(
		Principal{Kind: PrincipalAgent, ID: "agent-1"},
		BranchPushAgentPattern,
		Resource{
			OrgID:  "org-1",
			Kind:   ResourceBranch,
			ID:     "branch-1",
			RepoID: "repo-1",
			Path:   "cmd/server/main.go",
			Branch: "agent/workspace-1",
		},
		now,
	)
	if matches {
		t.Fatal("expected path outside grant scope not to match")
	}
}

func TestMatchGlobSupportsDoubleStar(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		{pattern: "internal/**", value: "internal/service/auth.go", want: true},
		{pattern: "agent/*", value: "agent/workspace-1", want: true},
		{pattern: "agent/*", value: "agent/workspace-1/fix", want: false},
		{pattern: ".github/**", value: "internal/workflow.yml", want: false},
	}

	for _, tt := range tests {
		if got := MatchGlob(tt.pattern, tt.value); got != tt.want {
			t.Fatalf("MatchGlob(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
		}
	}
}
