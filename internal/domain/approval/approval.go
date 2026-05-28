package approval

import (
	"errors"
	"fmt"
	"time"
)

type GateType string

const (
	GateBeforePush        GateType = "before_push"
	GateOpenPR            GateType = "open_pr"
	GateProtectedPath     GateType = "protected_path"
	GatePrivilegedCommand GateType = "privileged_command"
	GateMerge             GateType = "merge"
	GateSecret            GateType = "secret"
	GateNetwork           GateType = "network"
	GateCrossRepo         GateType = "cross_repo"
)

type ResourceKind string

const (
	ResourceRepo    ResourceKind = "repo"
	ResourceBranch  ResourceKind = "branch"
	ResourcePath    ResourceKind = "path"
	ResourceCommand ResourceKind = "command"
	ResourceSecret  ResourceKind = "secret"
	ResourceBundle  ResourceKind = "bundle"
)

type GateStatus string

const (
	StatusPending   GateStatus = "pending"
	StatusApproved  GateStatus = "approved"
	StatusDenied    GateStatus = "denied"
	StatusExpired   GateStatus = "expired"
	StatusCancelled GateStatus = "cancelled"
	StatusUsed      GateStatus = "used"
)

type DecisionKind string

const (
	DecisionApproved DecisionKind = "approved"
	DecisionDenied   DecisionKind = "denied"
	DecisionRevoked  DecisionKind = "revoked"
)

type Gate struct {
	ID                 string
	OrgID              string
	WorkspaceID        string
	Type               GateType
	Action             string
	ResourceKind       ResourceKind
	ResourceID         string
	RequestedByAgentID string
	RequestedByUserID  string
	Status             GateStatus
	ContextHash        string
	DiffHash           string
	ContextArtifactID  string
	ExpiresAt          *time.Time
	Decisions          []Decision
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Metadata           map[string]string
}

type Decision struct {
	ID                  string
	GateID              string
	Decision            DecisionKind
	DecidedByUserID     string
	DecidedAt           time.Time
	ApprovedContextHash string
	Comment             string
	Metadata            map[string]string
}

type GateSpec struct {
	ID                 string
	OrgID              string
	WorkspaceID        string
	Type               GateType
	Action             string
	ResourceKind       ResourceKind
	ResourceID         string
	RequestedByAgentID string
	RequestedByUserID  string
	ContextHash        string
	DiffHash           string
	ContextArtifactID  string
	ExpiresAt          *time.Time
	CreatedAt          time.Time
	Metadata           map[string]string
}

func NewGate(spec GateSpec) (*Gate, error) {
	gate := &Gate{
		ID:                 spec.ID,
		OrgID:              spec.OrgID,
		WorkspaceID:        spec.WorkspaceID,
		Type:               spec.Type,
		Action:             spec.Action,
		ResourceKind:       spec.ResourceKind,
		ResourceID:         spec.ResourceID,
		RequestedByAgentID: spec.RequestedByAgentID,
		RequestedByUserID:  spec.RequestedByUserID,
		Status:             StatusPending,
		ContextHash:        spec.ContextHash,
		DiffHash:           spec.DiffHash,
		ContextArtifactID:  spec.ContextArtifactID,
		ExpiresAt:          spec.ExpiresAt,
		CreatedAt:          spec.CreatedAt,
		UpdatedAt:          spec.CreatedAt,
		Metadata:           cloneMap(spec.Metadata),
	}
	return gate, gate.Validate()
}

func (g Gate) Validate() error {
	if g.ID == "" || g.OrgID == "" || g.WorkspaceID == "" {
		return errors.New("approval gate id, org id, and workspace id are required")
	}
	switch g.Type {
	case GateBeforePush, GateOpenPR, GateProtectedPath, GatePrivilegedCommand, GateMerge, GateSecret, GateNetwork, GateCrossRepo:
	default:
		return fmt.Errorf("unsupported approval gate type %q", g.Type)
	}
	if g.Action == "" {
		return errors.New("approval gate action is required")
	}
	switch g.ResourceKind {
	case ResourceRepo, ResourceBranch, ResourcePath, ResourceCommand, ResourceSecret, ResourceBundle:
	default:
		return fmt.Errorf("unsupported approval resource kind %q", g.ResourceKind)
	}
	if g.ResourceID == "" {
		return errors.New("approval gate resource id is required")
	}
	if g.RequestedByAgentID == "" && g.RequestedByUserID == "" {
		return errors.New("approval gate requester is required")
	}
	switch g.Status {
	case StatusPending, StatusApproved, StatusDenied, StatusExpired, StatusCancelled, StatusUsed:
	default:
		return fmt.Errorf("unsupported approval gate status %q", g.Status)
	}
	if g.ContextHash == "" {
		return errors.New("approval gate context hash is required")
	}
	if g.ExpiresAt != nil && !g.ExpiresAt.After(g.CreatedAt) {
		return errors.New("approval gate expires_at must be after created_at")
	}
	if g.CreatedAt.IsZero() || g.UpdatedAt.IsZero() {
		return errors.New("approval gate timestamps are required")
	}
	for _, decision := range g.Decisions {
		if err := decision.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (g *Gate) Approve(decisionID, userID, contextHash, comment string, now time.Time) (*Decision, error) {
	if err := g.ensurePending(now); err != nil {
		return nil, err
	}
	if contextHash != g.ContextHash {
		return nil, errors.New("approved context hash does not match gate context hash")
	}
	decision := Decision{
		ID:                  decisionID,
		GateID:              g.ID,
		Decision:            DecisionApproved,
		DecidedByUserID:     userID,
		DecidedAt:           now,
		ApprovedContextHash: contextHash,
		Comment:             comment,
	}
	if err := decision.Validate(); err != nil {
		return nil, err
	}
	g.Status = StatusApproved
	g.UpdatedAt = now
	g.Decisions = append(g.Decisions, decision)
	return &decision, nil
}

func (g *Gate) Deny(decisionID, userID, comment string, now time.Time) (*Decision, error) {
	if err := g.ensurePending(now); err != nil {
		return nil, err
	}
	decision := Decision{
		ID:              decisionID,
		GateID:          g.ID,
		Decision:        DecisionDenied,
		DecidedByUserID: userID,
		DecidedAt:       now,
		Comment:         comment,
	}
	if err := decision.Validate(); err != nil {
		return nil, err
	}
	g.Status = StatusDenied
	g.UpdatedAt = now
	g.Decisions = append(g.Decisions, decision)
	return &decision, nil
}

func (g *Gate) MarkUsed(contextHash string, now time.Time) error {
	if g == nil {
		return errors.New("approval gate is required")
	}
	if g.Status != StatusApproved {
		return fmt.Errorf("approval gate is not approved: %s", g.Status)
	}
	if g.ExpiresAt != nil && !now.Before(*g.ExpiresAt) {
		g.Status = StatusExpired
		g.UpdatedAt = now
		return errors.New("approval gate is expired")
	}
	if contextHash != g.ContextHash {
		return errors.New("used context hash does not match approved context hash")
	}
	g.Status = StatusUsed
	g.UpdatedAt = now
	return nil
}

func (g *Gate) Expire(now time.Time) bool {
	if g == nil || g.Status != StatusPending || g.ExpiresAt == nil || now.Before(*g.ExpiresAt) {
		return false
	}
	g.Status = StatusExpired
	g.UpdatedAt = now
	return true
}

func (g Gate) IsApprovedFor(contextHash string, now time.Time) bool {
	if g.Status != StatusApproved {
		return false
	}
	if g.ExpiresAt != nil && !now.Before(*g.ExpiresAt) {
		return false
	}
	return g.ContextHash == contextHash
}

func (g *Gate) ensurePending(now time.Time) error {
	if g == nil {
		return errors.New("approval gate is required")
	}
	if g.Status != StatusPending {
		return fmt.Errorf("approval gate is not pending: %s", g.Status)
	}
	if g.ExpiresAt != nil && !now.Before(*g.ExpiresAt) {
		g.Status = StatusExpired
		g.UpdatedAt = now
		return errors.New("approval gate is expired")
	}
	return nil
}

func (d Decision) Validate() error {
	if d.ID == "" || d.GateID == "" {
		return errors.New("approval decision id and gate id are required")
	}
	switch d.Decision {
	case DecisionApproved:
		if d.ApprovedContextHash == "" {
			return errors.New("approved context hash is required")
		}
	case DecisionDenied, DecisionRevoked:
	default:
		return fmt.Errorf("unsupported approval decision %q", d.Decision)
	}
	if d.DecidedByUserID == "" {
		return errors.New("approval decision decided_by_user_id is required")
	}
	if d.DecidedAt.IsZero() {
		return errors.New("approval decision decided_at is required")
	}
	return nil
}

func cloneMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	clone := make(map[string]string, len(values))
	for k, v := range values {
		clone[k] = v
	}
	return clone
}
