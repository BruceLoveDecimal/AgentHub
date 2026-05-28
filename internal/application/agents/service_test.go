package agents

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/agent"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/capability"
)

func TestCreateAgentPersistsAgentAndAuditEvent(t *testing.T) {
	ctx := context.Background()
	svc, stores := newTestService()

	created, err := svc.CreateAgent(ctx, CreateAgentCommand{
		OrgID:               "org-1",
		Slug:                "review-agent",
		DisplayName:         "Review Agent",
		Purpose:             "Review pull requests",
		OwnerUserID:         "user-1",
		RecoveryOwnerUserID: "user-2",
		CreatedByUserID:     "user-1",
	})
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if stores.agents.saved[created.ID] == nil {
		t.Fatal("expected agent to be persisted")
	}
	if len(stores.audit.events) != 1 {
		t.Fatalf("expected one audit event, got %d", len(stores.audit.events))
	}
	if stores.audit.events[0].Type != audit.EventAgentCreated {
		t.Fatalf("unexpected audit event type: %s", stores.audit.events[0].Type)
	}
}

func TestIssueTaskTokenUsesScopedGrantsAndDoesNotPersistPlaintext(t *testing.T) {
	ctx := context.Background()
	svc, stores := newTestService()
	now := stores.clock.now
	created, err := agent.New(agent.Profile{
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
	stores.agents.saved[created.ID] = created
	grant := mustCapabilityGrant(t, capability.GrantSpec{
		ID:              "grant-1",
		OrgID:           "org-1",
		Principal:       capability.Principal{Kind: capability.PrincipalAgent, ID: "agent-1"},
		Capability:      capability.CodeGrep,
		Effect:          capability.EffectAllow,
		ResourceKind:    capability.ResourceRepo,
		ResourceID:      "repo-1",
		RepoID:          "repo-1",
		CreatedByUserID: "user-1",
		CreatedAt:       now,
	})
	stores.grants.grants = append(stores.grants.grants, grant)

	result, err := svc.IssueTaskToken(ctx, IssueTaskTokenCommand{
		OrgID:          "org-1",
		WorkspaceID:    "workspace-1",
		AgentID:        "agent-1",
		IssuedByUserID: "user-1",
		TTL:            time.Minute,
	})
	if err != nil {
		t.Fatalf("issue task token: %v", err)
	}
	if result.Plaintext != "plain-token" {
		t.Fatalf("unexpected plaintext token: %q", result.Plaintext)
	}
	if len(stores.tokens.tokens) != 1 {
		t.Fatalf("expected one token to be persisted, got %d", len(stores.tokens.tokens))
	}
	persisted := stores.tokens.tokens[0]
	if persisted.TokenHash != "sha256:plain-token" {
		t.Fatalf("unexpected token hash: %q", persisted.TokenHash)
	}
	if len(persisted.Scope) != 1 || persisted.Scope[0].ID != "grant-1" {
		t.Fatalf("expected token scope to include grant, got %+v", persisted.Scope)
	}
	if result.ActorChain.AgentID() != "agent-1" {
		t.Fatalf("unexpected actor chain agent: %q", result.ActorChain.AgentID())
	}
}

func TestAuthorizeCapabilityWritesAuditEvent(t *testing.T) {
	ctx := context.Background()
	svc, stores := newTestService()
	now := stores.clock.now
	grant := mustCapabilityGrant(t, capability.GrantSpec{
		ID:              "grant-1",
		OrgID:           "org-1",
		Principal:       capability.Principal{Kind: capability.PrincipalAgent, ID: "agent-1"},
		Capability:      capability.CodeGrep,
		Effect:          capability.EffectAllow,
		ResourceKind:    capability.ResourceRepo,
		ResourceID:      "repo-1",
		RepoID:          "repo-1",
		CreatedByUserID: "user-1",
		CreatedAt:       now,
	})
	stores.grants.grants = append(stores.grants.grants, grant)
	chain, err := audit.NewAgentActorChain("chain-1", "org-1", "workspace-1", "user-1", "agent-1", now)
	if err != nil {
		t.Fatalf("new actor chain: %v", err)
	}

	decision, err := svc.AuthorizeCapability(ctx, AuthorizeCapabilityCommand{
		ActorChain: chain,
		Principal:  capability.Principal{Kind: capability.PrincipalAgent, ID: "agent-1"},
		Capability: capability.CodeGrep,
		Resource: capability.Resource{
			OrgID:       "org-1",
			Kind:        capability.ResourceRepo,
			ID:          "repo-1",
			RepoID:      "repo-1",
			WorkspaceID: "workspace-1",
		},
	})
	if err != nil {
		t.Fatalf("authorize capability: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("expected decision to be allowed, got %+v", decision)
	}
	if len(stores.audit.events) != 1 {
		t.Fatalf("expected one audit event, got %d", len(stores.audit.events))
	}
	event := stores.audit.events[0]
	if event.Type != audit.EventCapabilityChecked {
		t.Fatalf("unexpected audit event type: %s", event.Type)
	}
	if event.Decision != audit.DecisionAllowed {
		t.Fatalf("unexpected audit decision: %s", event.Decision)
	}
	if event.Payload["matched_grant_id"] != "grant-1" {
		t.Fatalf("expected matched grant in audit payload, got %+v", event.Payload)
	}
}

type testStores struct {
	agents *memoryAgentStore
	grants *memoryGrantStore
	tokens *memoryTaskTokenStore
	audit  *memoryAuditRecorder
	clock  *fixedClock
}

func newTestService() (Service, testStores) {
	stores := testStores{
		agents: &memoryAgentStore{saved: map[string]*agent.Agent{}},
		grants: &memoryGrantStore{},
		tokens: &memoryTaskTokenStore{},
		audit:  &memoryAuditRecorder{},
		clock:  &fixedClock{now: time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)},
	}
	svc := Service{
		Agents:           stores.agents,
		CapabilityGrants: stores.grants,
		TaskTokens:       stores.tokens,
		Audit:            stores.audit,
		IDs:              &sequenceIDs{},
		Tokens:           staticTokenGenerator{},
		Clock:            stores.clock,
	}
	return svc, stores
}

type memoryAgentStore struct {
	saved map[string]*agent.Agent
}

func (s *memoryAgentStore) Save(_ context.Context, value *agent.Agent) error {
	s.saved[value.ID] = value
	return nil
}

func (s *memoryAgentStore) Find(_ context.Context, id string) (*agent.Agent, error) {
	value := s.saved[id]
	if value == nil {
		return nil, errors.New("agent not found")
	}
	return value, nil
}

type memoryGrantStore struct {
	grants []capability.Grant
}

func (s *memoryGrantStore) Save(_ context.Context, grant *capability.Grant) error {
	s.grants = append(s.grants, *grant)
	return nil
}

func (s *memoryGrantStore) ListForPrincipal(_ context.Context, principal capability.Principal) ([]capability.Grant, error) {
	var grants []capability.Grant
	for _, grant := range s.grants {
		if grant.Principal == principal {
			grants = append(grants, grant)
		}
	}
	return grants, nil
}

type memoryTaskTokenStore struct {
	tokens []*agent.TaskToken
}

func (s *memoryTaskTokenStore) Save(_ context.Context, token *agent.TaskToken) error {
	s.tokens = append(s.tokens, token)
	return nil
}

type memoryAuditRecorder struct {
	events []*audit.Event
}

func (r *memoryAuditRecorder) Record(_ context.Context, event *audit.Event) error {
	r.events = append(r.events, event)
	return nil
}

type sequenceIDs struct {
	next int
}

func (g *sequenceIDs) NewID() string {
	g.next++
	return "id-" + string(rune('0'+g.next))
}

type staticTokenGenerator struct{}

func (g staticTokenGenerator) NewToken(context.Context) (string, string, error) {
	return "plain-token", "sha256:plain-token", nil
}

type fixedClock struct {
	now time.Time
}

func (c *fixedClock) Now() time.Time {
	return c.now
}

func mustCapabilityGrant(t *testing.T, spec capability.GrantSpec) capability.Grant {
	t.Helper()
	grant, err := capability.NewGrant(spec)
	if err != nil {
		t.Fatalf("new grant: %v", err)
	}
	return *grant
}
