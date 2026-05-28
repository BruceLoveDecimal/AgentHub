package agent

import (
	"strings"
	"testing"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
)

func TestNewAgentRequiresPurposeAndRecoveryOwner(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)

	_, err := New(Profile{
		ID:              "agent-1",
		OrgID:           "org-1",
		Slug:            "review-agent",
		DisplayName:     "Review Agent",
		OwnerUserID:     "user-1",
		CreatedByUserID: "user-1",
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	if err == nil || !strings.Contains(err.Error(), "purpose") {
		t.Fatalf("expected purpose validation error, got %v", err)
	}

	_, err = New(Profile{
		ID:              "agent-1",
		OrgID:           "org-1",
		Slug:            "review-agent",
		DisplayName:     "Review Agent",
		Purpose:         "Review pull requests",
		OwnerUserID:     "user-1",
		CreatedByUserID: "user-1",
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	if err == nil || !strings.Contains(err.Error(), "recovery owner") {
		t.Fatalf("expected recovery owner validation error, got %v", err)
	}
}

func TestIssueTaskTokenEnforcesShortLivedPolicy(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	created, err := New(Profile{
		ID:                  "agent-1",
		OrgID:               "org-1",
		Slug:                "review-agent",
		DisplayName:         "Review Agent",
		Purpose:             "Review pull requests",
		OwnerUserID:         "user-1",
		RecoveryOwnerUserID: "user-2",
		MaxTokenTTL:         5 * time.Minute,
		CreatedByUserID:     "user-1",
		CreatedAt:           now,
		UpdatedAt:           now,
	})
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}
	chain, err := audit.NewAgentActorChain("chain-1", "org-1", "workspace-1", "user-1", "agent-1", now)
	if err != nil {
		t.Fatalf("new actor chain: %v", err)
	}

	_, err = IssueTaskToken(created, TaskTokenSpec{
		ID:             "token-1",
		OrgID:          "org-1",
		WorkspaceID:    "workspace-1",
		IssuedByUserID: "user-1",
		TokenHash:      "sha256:token",
		ActorChain:     chain,
		TTL:            10 * time.Minute,
		Now:            now,
	})
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected ttl validation error, got %v", err)
	}

	token, err := IssueTaskToken(created, TaskTokenSpec{
		ID:             "token-1",
		OrgID:          "org-1",
		WorkspaceID:    "workspace-1",
		IssuedByUserID: "user-1",
		TokenHash:      "sha256:token",
		ActorChain:     chain,
		TTL:            time.Minute,
		Now:            now,
	})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	if token.ExpiresAt != now.Add(time.Minute) {
		t.Fatalf("unexpected expires_at: %v", token.ExpiresAt)
	}
	if token.IsExpired(now.Add(59 * time.Second)) {
		t.Fatal("token should still be active before expiry")
	}
	if !token.IsExpired(now.Add(time.Minute)) {
		t.Fatal("token should be expired at expiry")
	}
}
