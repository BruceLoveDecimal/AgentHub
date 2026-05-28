package authorization

import (
	"testing"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/capability"
)

func TestAuthorizeDenyOverridesAllow(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	chain, err := audit.NewAgentActorChain("chain-1", "org-1", "workspace-1", "user-1", "agent-1", now)
	if err != nil {
		t.Fatalf("new chain: %v", err)
	}
	grants := []capability.Grant{
		mustGrant(t, capability.GrantSpec{
			ID:              "allow-1",
			OrgID:           "org-1",
			Principal:       capability.Principal{Kind: capability.PrincipalAgent, ID: "agent-1"},
			Capability:      capability.CodeReadFile,
			Effect:          capability.EffectAllow,
			ResourceKind:    capability.ResourcePath,
			ResourceID:      "path-1",
			RepoID:          "repo-1",
			PathGlob:        "**",
			CreatedByUserID: "user-1",
			CreatedAt:       now,
		}),
		mustGrant(t, capability.GrantSpec{
			ID:              "deny-1",
			OrgID:           "org-1",
			Principal:       capability.Principal{Kind: capability.PrincipalAgent, ID: "agent-1"},
			Capability:      capability.CodeReadFile,
			Effect:          capability.EffectDeny,
			ResourceKind:    capability.ResourcePath,
			ResourceID:      "path-1",
			RepoID:          "repo-1",
			PathGlob:        "secrets/**",
			CreatedByUserID: "user-1",
			CreatedAt:       now,
		}),
	}

	decision := Service{}.Authorize(Request{
		ActorChain: chain,
		Principal:  capability.Principal{Kind: capability.PrincipalAgent, ID: "agent-1"},
		Capability: capability.CodeReadFile,
		Resource: capability.Resource{
			OrgID:  "org-1",
			Kind:   capability.ResourcePath,
			ID:     "path-1",
			RepoID: "repo-1",
			Path:   "secrets/prod.env",
		},
		At: now,
	}, grants)

	if decision.Allowed {
		t.Fatal("expected explicit deny to block the request")
	}
	if decision.Reason != "explicit_deny" || decision.MatchedGrantID != "deny-1" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestAuthorizeRequiresApproval(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	chain, err := audit.NewAgentActorChain("chain-1", "org-1", "workspace-1", "user-1", "agent-1", now)
	if err != nil {
		t.Fatalf("new chain: %v", err)
	}
	grant := mustGrant(t, capability.GrantSpec{
		ID:              "grant-1",
		OrgID:           "org-1",
		Principal:       capability.Principal{Kind: capability.PrincipalAgent, ID: "agent-1"},
		Capability:      capability.ProtectedPathModify,
		Effect:          capability.EffectAllow,
		ResourceKind:    capability.ResourcePath,
		ResourceID:      "path-1",
		RepoID:          "repo-1",
		PathGlob:        "migrations/**",
		Conditions:      capability.Conditions{RequiresApproval: true},
		CreatedByUserID: "user-1",
		CreatedAt:       now,
	})

	decision := Service{}.Authorize(Request{
		ActorChain: chain,
		Principal:  capability.Principal{Kind: capability.PrincipalAgent, ID: "agent-1"},
		Capability: capability.ProtectedPathModify,
		Resource: capability.Resource{
			OrgID:  "org-1",
			Kind:   capability.ResourcePath,
			ID:     "path-1",
			RepoID: "repo-1",
			Path:   "migrations/001.sql",
		},
		At: now,
	}, []capability.Grant{grant})

	if decision.Allowed {
		t.Fatal("approval-gated request should not be immediately allowed")
	}
	if !decision.RequiresApproval || decision.Reason != "approval_required" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func mustGrant(t *testing.T, spec capability.GrantSpec) capability.Grant {
	t.Helper()
	grant, err := capability.NewGrant(spec)
	if err != nil {
		t.Fatalf("new grant: %v", err)
	}
	return *grant
}
