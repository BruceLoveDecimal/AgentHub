package authorization

import (
	"errors"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/approval"
)

type ApprovalRequest struct {
	Gate        approval.Gate
	ContextHash string
	At          time.Time
	Consume     bool
}

type ApprovalDecision struct {
	Allowed bool
	Reason  string
}

func (Service) EnforceApproval(req ApprovalRequest) ApprovalDecision {
	if req.At.IsZero() {
		req.At = time.Now().UTC()
	}
	if req.Gate.IsApprovedFor(req.ContextHash, req.At) {
		return ApprovalDecision{Allowed: true, Reason: "approved"}
	}
	if req.Gate.Status == approval.StatusPending {
		return ApprovalDecision{Allowed: false, Reason: "approval_pending"}
	}
	if req.Gate.Status == approval.StatusExpired || (req.Gate.ExpiresAt != nil && !req.At.Before(*req.Gate.ExpiresAt)) {
		return ApprovalDecision{Allowed: false, Reason: "approval_expired"}
	}
	if req.Gate.ContextHash != req.ContextHash {
		return ApprovalDecision{Allowed: false, Reason: "context_mismatch"}
	}
	return ApprovalDecision{Allowed: false, Reason: string(req.Gate.Status)}
}

func (Service) ConsumeApproval(gate *approval.Gate, contextHash string, now time.Time) error {
	if gate == nil {
		return errors.New("approval gate is required")
	}
	return gate.MarkUsed(contextHash, now)
}
