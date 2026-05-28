package change

import (
	"errors"
	"fmt"
	"time"
)

type RevertPlanKind string

const (
	RevertCommit      RevertPlanKind = "revert_commit"
	RollbackMigration RevertPlanKind = "rollback_migration"
	ManualSteps       RevertPlanKind = "manual_steps"
)

type RevertPlanStatus string

const (
	RevertDraft       RevertPlanStatus = "draft"
	RevertApproved    RevertPlanStatus = "approved"
	RevertExecuted    RevertPlanStatus = "executed"
	RevertInvalidated RevertPlanStatus = "invalidated"
)

type RevertPlan struct {
	ID                 string
	BundleID           string
	CommitID           string
	Kind               RevertPlanKind
	Steps              []string
	Status             RevertPlanStatus
	GeneratedByAgentID string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func NewRevertPlan(plan RevertPlan) (*RevertPlan, error) {
	if plan.Status == "" {
		plan.Status = RevertDraft
	}
	plan.Steps = cloneSlice(plan.Steps)
	return &plan, plan.Validate()
}

func (p RevertPlan) Validate() error {
	if p.ID == "" {
		return errors.New("revert plan id is required")
	}
	if p.BundleID == "" && p.CommitID == "" {
		return errors.New("revert plan requires bundle id or commit id")
	}
	switch p.Kind {
	case RevertCommit, RollbackMigration, ManualSteps:
	default:
		return fmt.Errorf("unsupported revert plan kind %q", p.Kind)
	}
	if len(p.Steps) == 0 {
		return errors.New("revert plan steps are required")
	}
	switch p.Status {
	case RevertDraft, RevertApproved, RevertExecuted, RevertInvalidated:
	default:
		return fmt.Errorf("unsupported revert plan status %q", p.Status)
	}
	if p.GeneratedByAgentID == "" {
		return errors.New("revert plan generated_by_agent_id is required")
	}
	if p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		return errors.New("revert plan timestamps are required")
	}
	return nil
}
