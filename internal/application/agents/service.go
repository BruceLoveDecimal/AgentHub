package agents

import (
	"context"
	"errors"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/agent"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/capability"
	authz "github.com/BruceLoveDecimal/AgentHub/internal/service/authorization"
)

type Service struct {
	Agents           AgentStore
	CapabilityGrants CapabilityGrantStore
	TaskTokens       TaskTokenStore
	Audit            AuditRecorder
	IDs              IDGenerator
	Tokens           TokenGenerator
	Clock            Clock
}

type CreateAgentCommand struct {
	OrgID               string
	Slug                string
	DisplayName         string
	Purpose             string
	OwnerUserID         string
	RecoveryOwnerUserID string
	DefaultModel        string
	MaxTokenTTL         time.Duration
	CreatedByUserID     string
	Metadata            map[string]string
}

func (s Service) CreateAgent(ctx context.Context, cmd CreateAgentCommand) (*agent.Agent, error) {
	if err := s.requireCommonPorts(); err != nil {
		return nil, err
	}
	if s.Agents == nil {
		return nil, errors.New("agent store is required")
	}
	now := s.Clock.Now().UTC()
	created, err := agent.New(agent.Profile{
		ID:                  s.IDs.NewID(),
		OrgID:               cmd.OrgID,
		Slug:                cmd.Slug,
		DisplayName:         cmd.DisplayName,
		Purpose:             cmd.Purpose,
		OwnerUserID:         cmd.OwnerUserID,
		RecoveryOwnerUserID: cmd.RecoveryOwnerUserID,
		Status:              agent.StatusActive,
		DefaultModel:        cmd.DefaultModel,
		MaxTokenTTL:         cmd.MaxTokenTTL,
		CreatedByUserID:     cmd.CreatedByUserID,
		CreatedAt:           now,
		UpdatedAt:           now,
		Metadata:            cmd.Metadata,
	})
	if err != nil {
		return nil, err
	}
	if err := s.Agents.Save(ctx, created); err != nil {
		return nil, err
	}

	chain, err := audit.NewHumanActorChain(s.IDs.NewID(), cmd.OrgID, "", cmd.CreatedByUserID, now)
	if err != nil {
		return nil, err
	}
	if err := s.record(ctx, audit.EventSpec{
		ID:           s.IDs.NewID(),
		OrgID:        cmd.OrgID,
		ActorChainID: chain.ID,
		Type:         audit.EventAgentCreated,
		SubjectKind:  "agent",
		SubjectID:    created.ID,
		Action:       "create_agent",
		Decision:     audit.DecisionRecorded,
		OccurredAt:   now,
	}); err != nil {
		return nil, err
	}

	return created, nil
}

type GrantCapabilityCommand struct {
	OrgID           string
	Principal       capability.Principal
	Capability      capability.Name
	Effect          capability.Effect
	ResourceKind    capability.ResourceKind
	ResourceID      string
	RepoID          string
	PathGlob        string
	BranchPattern   string
	Conditions      capability.Conditions
	ExpiresAt       *time.Time
	CreatedByUserID string
}

func (s Service) GrantCapability(ctx context.Context, cmd GrantCapabilityCommand) (*capability.Grant, error) {
	if err := s.requireCommonPorts(); err != nil {
		return nil, err
	}
	if s.CapabilityGrants == nil {
		return nil, errors.New("capability grant store is required")
	}
	now := s.Clock.Now().UTC()
	grant, err := capability.NewGrant(capability.GrantSpec{
		ID:              s.IDs.NewID(),
		OrgID:           cmd.OrgID,
		Principal:       cmd.Principal,
		Capability:      cmd.Capability,
		Effect:          cmd.Effect,
		ResourceKind:    cmd.ResourceKind,
		ResourceID:      cmd.ResourceID,
		RepoID:          cmd.RepoID,
		PathGlob:        cmd.PathGlob,
		BranchPattern:   cmd.BranchPattern,
		Conditions:      cmd.Conditions,
		ExpiresAt:       cmd.ExpiresAt,
		CreatedByUserID: cmd.CreatedByUserID,
		CreatedAt:       now,
	})
	if err != nil {
		return nil, err
	}
	if err := s.CapabilityGrants.Save(ctx, grant); err != nil {
		return nil, err
	}

	chain, err := audit.NewHumanActorChain(s.IDs.NewID(), cmd.OrgID, "", cmd.CreatedByUserID, now)
	if err != nil {
		return nil, err
	}
	if err := s.record(ctx, audit.EventSpec{
		ID:           s.IDs.NewID(),
		OrgID:        cmd.OrgID,
		ActorChainID: chain.ID,
		Type:         audit.EventCapabilityGranted,
		SubjectKind:  "capability_grant",
		SubjectID:    grant.ID,
		Action:       "grant_capability",
		Decision:     audit.DecisionRecorded,
		Payload: map[string]string{
			"capability": string(grant.Capability),
			"effect":     string(grant.Effect),
		},
		OccurredAt: now,
	}); err != nil {
		return nil, err
	}

	return grant, nil
}

type AuthorizeCapabilityCommand struct {
	ActorChain audit.ActorChain
	Principal  capability.Principal
	Capability capability.Name
	Resource   capability.Resource
	At         time.Time
}

func (s Service) AuthorizeCapability(ctx context.Context, cmd AuthorizeCapabilityCommand) (authz.Decision, error) {
	if err := s.requireCommonPorts(); err != nil {
		return authz.Decision{}, err
	}
	if s.CapabilityGrants == nil {
		return authz.Decision{}, errors.New("capability grant store is required")
	}
	at := cmd.At
	if at.IsZero() {
		at = s.Clock.Now().UTC()
	}
	grants, err := s.CapabilityGrants.ListForPrincipal(ctx, cmd.Principal)
	if err != nil {
		return authz.Decision{}, err
	}
	decision := authz.Service{}.Authorize(authz.Request{
		ActorChain: cmd.ActorChain,
		Principal:  cmd.Principal,
		Capability: cmd.Capability,
		Resource:   cmd.Resource,
		At:         at,
	}, grants)

	auditDecision := audit.DecisionDenied
	if decision.Allowed {
		auditDecision = audit.DecisionAllowed
	}
	if err := s.record(ctx, audit.EventSpec{
		ID:           s.IDs.NewID(),
		OrgID:        cmd.Resource.OrgID,
		WorkspaceID:  cmd.ActorChain.WorkspaceID,
		ActorChainID: cmd.ActorChain.ID,
		Type:         audit.EventCapabilityChecked,
		SubjectKind:  "capability",
		SubjectID:    string(cmd.Capability),
		Action:       "check_capability",
		Decision:     auditDecision,
		Payload: map[string]string{
			"reason":           decision.Reason,
			"matched_grant_id": decision.MatchedGrantID,
		},
		OccurredAt: at,
	}); err != nil {
		return authz.Decision{}, err
	}

	return decision, nil
}

type IssueTaskTokenCommand struct {
	OrgID          string
	WorkspaceID    string
	AgentID        string
	IssuedByUserID string
	TTL            time.Duration
}

type IssueTaskTokenResult struct {
	Token      *agent.TaskToken
	Plaintext  string
	ActorChain audit.ActorChain
}

func (s Service) IssueTaskToken(ctx context.Context, cmd IssueTaskTokenCommand) (*IssueTaskTokenResult, error) {
	if err := s.requireCommonPorts(); err != nil {
		return nil, err
	}
	if s.Agents == nil {
		return nil, errors.New("agent store is required")
	}
	if s.CapabilityGrants == nil {
		return nil, errors.New("capability grant store is required")
	}
	if s.TaskTokens == nil {
		return nil, errors.New("task token store is required")
	}
	if s.Tokens == nil {
		return nil, errors.New("token generator is required")
	}
	loaded, err := s.Agents.Find(ctx, cmd.AgentID)
	if err != nil {
		return nil, err
	}
	now := s.Clock.Now().UTC()
	chain, err := audit.NewAgentActorChain(s.IDs.NewID(), cmd.OrgID, cmd.WorkspaceID, cmd.IssuedByUserID, cmd.AgentID, now)
	if err != nil {
		return nil, err
	}
	plaintext, hash, err := s.Tokens.NewToken(ctx)
	if err != nil {
		return nil, err
	}
	scope, err := s.CapabilityGrants.ListForPrincipal(ctx, capability.Principal{
		Kind: capability.PrincipalAgent,
		ID:   cmd.AgentID,
	})
	if err != nil {
		return nil, err
	}
	taskToken, err := agent.IssueTaskToken(loaded, agent.TaskTokenSpec{
		ID:             s.IDs.NewID(),
		OrgID:          cmd.OrgID,
		WorkspaceID:    cmd.WorkspaceID,
		IssuedByUserID: cmd.IssuedByUserID,
		TokenHash:      hash,
		Scope:          scope,
		ActorChain:     chain,
		TTL:            cmd.TTL,
		Now:            now,
	})
	if err != nil {
		return nil, err
	}
	if err := s.TaskTokens.Save(ctx, taskToken); err != nil {
		return nil, err
	}
	if err := s.record(ctx, audit.EventSpec{
		ID:           s.IDs.NewID(),
		OrgID:        cmd.OrgID,
		WorkspaceID:  cmd.WorkspaceID,
		ActorChainID: chain.ID,
		Type:         audit.EventTaskTokenIssued,
		SubjectKind:  "task_token",
		SubjectID:    taskToken.ID,
		Action:       "issue_task_token",
		Decision:     audit.DecisionRecorded,
		Payload: map[string]string{
			"agent_id": cmd.AgentID,
		},
		OccurredAt: now,
	}); err != nil {
		return nil, err
	}

	return &IssueTaskTokenResult{
		Token:      taskToken,
		Plaintext:  plaintext,
		ActorChain: chain,
	}, nil
}

func (s Service) requireCommonPorts() error {
	if s.Audit == nil {
		return errors.New("audit recorder is required")
	}
	if s.IDs == nil {
		return errors.New("id generator is required")
	}
	if s.Clock == nil {
		return errors.New("clock is required")
	}
	return nil
}

func (s Service) record(ctx context.Context, spec audit.EventSpec) error {
	event, err := audit.NewEvent(spec)
	if err != nil {
		return err
	}
	return s.Audit.Record(ctx, event)
}
