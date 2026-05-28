package agent

import (
	"errors"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/capability"
)

type TaskToken struct {
	ID             string
	OrgID          string
	WorkspaceID    string
	AgentID        string
	IssuedByUserID string
	TokenHash      string
	Scope          []capability.Grant
	ActorChain     audit.ActorChain
	ExpiresAt      time.Time
	LastUsedAt     *time.Time
	RevokedAt      *time.Time
	CreatedAt      time.Time
}

type TaskTokenSpec struct {
	ID             string
	OrgID          string
	WorkspaceID    string
	IssuedByUserID string
	TokenHash      string
	Scope          []capability.Grant
	ActorChain     audit.ActorChain
	TTL            time.Duration
	Now            time.Time
}

func IssueTaskToken(agent *Agent, spec TaskTokenSpec) (*TaskToken, error) {
	if agent == nil {
		return nil, errors.New("agent is required")
	}
	if err := agent.CanIssueTaskToken(); err != nil {
		return nil, err
	}
	if spec.TTL <= 0 {
		return nil, errors.New("task token ttl must be positive")
	}
	if spec.TTL > agent.MaxTokenTTL {
		return nil, errors.New("task token ttl exceeds agent max token ttl")
	}
	if spec.Now.IsZero() {
		return nil, errors.New("task token created_at is required")
	}
	if err := spec.ActorChain.Validate(); err != nil {
		return nil, err
	}
	if spec.ActorChain.AgentID() != agent.ID {
		return nil, errors.New("task token actor chain agent does not match target agent")
	}

	token := &TaskToken{
		ID:             spec.ID,
		OrgID:          spec.OrgID,
		WorkspaceID:    spec.WorkspaceID,
		AgentID:        agent.ID,
		IssuedByUserID: spec.IssuedByUserID,
		TokenHash:      spec.TokenHash,
		Scope:          cloneGrants(spec.Scope),
		ActorChain:     spec.ActorChain,
		ExpiresAt:      spec.Now.Add(spec.TTL),
		CreatedAt:      spec.Now,
	}
	return token, token.Validate()
}

func (t TaskToken) Validate() error {
	if t.ID == "" {
		return errors.New("task token id is required")
	}
	if t.OrgID == "" {
		return errors.New("task token org id is required")
	}
	if t.WorkspaceID == "" {
		return errors.New("task token workspace id is required")
	}
	if t.AgentID == "" {
		return errors.New("task token agent id is required")
	}
	if t.IssuedByUserID == "" {
		return errors.New("task token issuer user id is required")
	}
	if t.TokenHash == "" {
		return errors.New("task token hash is required")
	}
	if err := t.ActorChain.Validate(); err != nil {
		return err
	}
	if !t.ExpiresAt.After(t.CreatedAt) {
		return errors.New("task token expires_at must be after created_at")
	}
	return nil
}

func (t TaskToken) IsExpired(now time.Time) bool {
	return !now.Before(t.ExpiresAt)
}

func (t TaskToken) IsRevoked() bool {
	return t.RevokedAt != nil
}

func cloneGrants(grants []capability.Grant) []capability.Grant {
	if len(grants) == 0 {
		return nil
	}
	clone := make([]capability.Grant, len(grants))
	copy(clone, grants)
	return clone
}
