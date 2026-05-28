package approvals

import (
	"context"
	"errors"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/approval"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	authz "github.com/BruceLoveDecimal/AgentHub/internal/service/authorization"
)

type Service struct {
	Gates GateStore
	Audit AuditRecorder
	IDs   IDGenerator
	Clock Clock
	Authz authz.Service
}

type RequestGateCommand struct {
	OrgID              string
	WorkspaceID        string
	Type               approval.GateType
	Action             string
	ResourceKind       approval.ResourceKind
	ResourceID         string
	RequestedByAgentID string
	RequestedByUserID  string
	ContextHash        string
	DiffHash           string
	ContextArtifactID  string
	ExpiresIn          time.Duration
	Metadata           map[string]string
}

func (s Service) RequestGate(ctx context.Context, cmd RequestGateCommand) (*approval.Gate, error) {
	if err := s.requirePorts(); err != nil {
		return nil, err
	}
	now := s.Clock.Now().UTC()
	var expiresAt *time.Time
	if cmd.ExpiresIn > 0 {
		expires := now.Add(cmd.ExpiresIn)
		expiresAt = &expires
	}
	gate, err := approval.NewGate(approval.GateSpec{
		ID:                 s.IDs.NewID(),
		OrgID:              cmd.OrgID,
		WorkspaceID:        cmd.WorkspaceID,
		Type:               cmd.Type,
		Action:             cmd.Action,
		ResourceKind:       cmd.ResourceKind,
		ResourceID:         cmd.ResourceID,
		RequestedByAgentID: cmd.RequestedByAgentID,
		RequestedByUserID:  cmd.RequestedByUserID,
		ContextHash:        cmd.ContextHash,
		DiffHash:           cmd.DiffHash,
		ContextArtifactID:  cmd.ContextArtifactID,
		ExpiresAt:          expiresAt,
		CreatedAt:          now,
		Metadata:           cmd.Metadata,
	})
	if err != nil {
		return nil, err
	}
	if err := s.Gates.Save(ctx, gate); err != nil {
		return nil, err
	}
	if err := s.record(ctx, *gate, audit.EventApprovalRequested, "request_approval", audit.DecisionRecorded); err != nil {
		return nil, err
	}
	return gate, nil
}

type DecideGateCommand struct {
	GateID          string
	Decision        approval.DecisionKind
	DecidedByUserID string
	ContextHash     string
	Comment         string
}

func (s Service) DecideGate(ctx context.Context, cmd DecideGateCommand) (*approval.Gate, *approval.Decision, error) {
	if err := s.requirePorts(); err != nil {
		return nil, nil, err
	}
	gate, err := s.Gates.Find(ctx, cmd.GateID)
	if err != nil {
		return nil, nil, err
	}
	now := s.Clock.Now().UTC()
	var decision *approval.Decision
	switch cmd.Decision {
	case approval.DecisionApproved:
		decision, err = gate.Approve(s.IDs.NewID(), cmd.DecidedByUserID, cmd.ContextHash, cmd.Comment, now)
	case approval.DecisionDenied:
		decision, err = gate.Deny(s.IDs.NewID(), cmd.DecidedByUserID, cmd.Comment, now)
	default:
		return nil, nil, errors.New("unsupported approval decision")
	}
	if err != nil {
		return nil, nil, err
	}
	if err := s.Gates.Save(ctx, gate); err != nil {
		return nil, nil, err
	}
	eventType := audit.EventApprovalApproved
	auditDecision := audit.DecisionAllowed
	if decision.Decision == approval.DecisionDenied {
		eventType = audit.EventApprovalDenied
		auditDecision = audit.DecisionDenied
	}
	if err := s.record(ctx, *gate, eventType, "decide_approval", auditDecision); err != nil {
		return nil, nil, err
	}
	return gate, decision, nil
}

type EnforceGateCommand struct {
	GateID      string
	ContextHash string
	Consume     bool
}

func (s Service) EnforceGate(ctx context.Context, cmd EnforceGateCommand) (authz.ApprovalDecision, error) {
	if err := s.requirePorts(); err != nil {
		return authz.ApprovalDecision{}, err
	}
	gate, err := s.Gates.Find(ctx, cmd.GateID)
	if err != nil {
		return authz.ApprovalDecision{}, err
	}
	now := s.Clock.Now().UTC()
	decision := s.Authz.EnforceApproval(authz.ApprovalRequest{
		Gate:        *gate,
		ContextHash: cmd.ContextHash,
		At:          now,
		Consume:     cmd.Consume,
	})
	if !decision.Allowed {
		_ = s.record(ctx, *gate, audit.EventApprovalEnforced, "enforce_approval", audit.DecisionDenied)
		return decision, nil
	}
	if cmd.Consume {
		if err := s.Authz.ConsumeApproval(gate, cmd.ContextHash, now); err != nil {
			return authz.ApprovalDecision{}, err
		}
		if err := s.Gates.Save(ctx, gate); err != nil {
			return authz.ApprovalDecision{}, err
		}
	}
	if err := s.record(ctx, *gate, audit.EventApprovalEnforced, "enforce_approval", audit.DecisionAllowed); err != nil {
		return authz.ApprovalDecision{}, err
	}
	return decision, nil
}

func (s Service) requirePorts() error {
	if s.Gates == nil || s.Audit == nil || s.IDs == nil || s.Clock == nil {
		return errors.New("approval service ports are required")
	}
	return nil
}

func (s Service) record(ctx context.Context, gate approval.Gate, eventType audit.EventType, action string, decision audit.Decision) error {
	event, err := audit.NewEvent(audit.EventSpec{
		ID:          s.IDs.NewID(),
		OrgID:       gate.OrgID,
		WorkspaceID: gate.WorkspaceID,
		Type:        eventType,
		SubjectKind: "approval_gate",
		SubjectID:   gate.ID,
		Action:      action,
		Decision:    decision,
		Payload: map[string]string{
			"gate_type":    string(gate.Type),
			"resource":     string(gate.ResourceKind),
			"resource_id":  gate.ResourceID,
			"context_hash": gate.ContextHash,
		},
		OccurredAt: s.Clock.Now().UTC(),
	})
	if err != nil {
		return err
	}
	return s.Audit.Record(ctx, event)
}
