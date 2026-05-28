package changes

import (
	"context"
	"errors"
	"fmt"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/change"
	"github.com/BruceLoveDecimal/AgentHub/internal/service/provenance"
)

type Service struct {
	Bundles     BundleStore
	Commits     CommitStore
	RevertPlans RevertPlanStore
	Audit       AuditRecorder
	IDs         IDGenerator
	Clock       Clock
	Provenance  provenance.Service
}

type CreateBundleCommand struct {
	OrgID            string
	WorkspaceID      string
	Title            string
	RiskLevel        change.RiskLevel
	CreatedByAgentID string
	ActorChain       audit.ActorChain
	Metadata         map[string]string
}

func (s Service) CreateBundle(ctx context.Context, cmd CreateBundleCommand) (*change.Bundle, error) {
	if err := s.requireBundlePorts(); err != nil {
		return nil, err
	}
	now := s.Clock.Now().UTC()
	bundle, err := change.NewBundle(change.BundleSpec{
		ID:               s.IDs.NewID(),
		OrgID:            cmd.OrgID,
		WorkspaceID:      cmd.WorkspaceID,
		Title:            cmd.Title,
		RiskLevel:        cmd.RiskLevel,
		CreatedByAgentID: cmd.CreatedByAgentID,
		CreatedAt:        now,
		Metadata:         cmd.Metadata,
	})
	if err != nil {
		return nil, err
	}
	if err := s.Bundles.Save(ctx, bundle); err != nil {
		return nil, err
	}
	if err := s.record(ctx, cmd.ActorChain, cmd.OrgID, cmd.WorkspaceID, audit.EventChangeBundleCreated, "change_bundle", bundle.ID, "create_change_bundle"); err != nil {
		return nil, err
	}
	return bundle, nil
}

type AddBundleItemCommand struct {
	BundleID          string
	RepoID            string
	WorkspaceBranchID string
	PullRequestID     string
	MergeOrder        int
	ActorChain        audit.ActorChain
}

func (s Service) AddBundleItem(ctx context.Context, cmd AddBundleItemCommand) (*change.Bundle, error) {
	if err := s.requireBundlePorts(); err != nil {
		return nil, err
	}
	bundle, err := s.Bundles.Find(ctx, cmd.BundleID)
	if err != nil {
		return nil, err
	}
	now := s.Clock.Now().UTC()
	if err := bundle.AddItem(change.BundleItem{
		ID:                s.IDs.NewID(),
		RepoID:            cmd.RepoID,
		WorkspaceBranchID: cmd.WorkspaceBranchID,
		PullRequestID:     cmd.PullRequestID,
		MergeOrder:        cmd.MergeOrder,
	}, now); err != nil {
		return nil, err
	}
	if err := s.Bundles.Save(ctx, bundle); err != nil {
		return nil, err
	}
	if err := s.record(ctx, cmd.ActorChain, bundle.OrgID, bundle.WorkspaceID, audit.EventChangeBundleItemAdded, "change_bundle_item", bundle.Items[len(bundle.Items)-1].ID, "add_change_bundle_item"); err != nil {
		return nil, err
	}
	return bundle, nil
}

type RecordCommitCommand struct {
	OrgID            string
	RepoID           string
	WorkspaceID      string
	BundleID         string
	SHA              string
	Message          string
	ParentSHAs       []string
	AuthorAgentID    string
	AuthorUserID     string
	AgentRunID       string
	ActorChain       audit.ActorChain
	PromptHash       string
	ContextHash      string
	DiffHash         string
	ToolLogRefs      []string
	TestEvidenceRefs []string
}

type RecordCommitResult struct {
	Commit     *change.Commit
	Provenance *change.Provenance
}

func (s Service) RecordCommitWithProvenance(ctx context.Context, cmd RecordCommitCommand) (*RecordCommitResult, error) {
	if err := s.requireCommitPorts(); err != nil {
		return nil, err
	}
	now := s.Clock.Now().UTC()
	commit, err := change.NewCommit(change.Commit{
		ID:            s.IDs.NewID(),
		OrgID:         cmd.OrgID,
		RepoID:        cmd.RepoID,
		WorkspaceID:   cmd.WorkspaceID,
		BundleID:      cmd.BundleID,
		SHA:           cmd.SHA,
		Message:       cmd.Message,
		ParentSHAs:    cmd.ParentSHAs,
		AuthorAgentID: cmd.AuthorAgentID,
		AuthorUserID:  cmd.AuthorUserID,
		CommittedAt:   now,
		CreatedAt:     now,
	})
	if err != nil {
		return nil, err
	}
	prov, err := s.Provenance.Build(provenance.BuildCommand{
		CommitID:         commit.ID,
		WorkspaceID:      cmd.WorkspaceID,
		AgentRunID:       cmd.AgentRunID,
		ActorChain:       cmd.ActorChain,
		PromptHash:       cmd.PromptHash,
		ContextHash:      cmd.ContextHash,
		DiffHash:         cmd.DiffHash,
		ToolLogRefs:      cmd.ToolLogRefs,
		TestEvidenceRefs: cmd.TestEvidenceRefs,
		CreatedAt:        now,
		Payload: map[string]string{
			"commit_sha": cmd.SHA,
			"bundle_id":  cmd.BundleID,
		},
	})
	if err != nil {
		return nil, err
	}
	if err := s.Commits.SaveCommit(ctx, commit); err != nil {
		return nil, err
	}
	if err := s.Commits.SaveProvenance(ctx, prov); err != nil {
		return nil, err
	}
	if err := s.record(ctx, cmd.ActorChain, cmd.OrgID, cmd.WorkspaceID, audit.EventCommitProvenanceRecorded, "commit", commit.ID, "record_commit_provenance"); err != nil {
		return nil, err
	}
	return &RecordCommitResult{Commit: commit, Provenance: prov}, nil
}

type GenerateRevertPlanCommand struct {
	OrgID              string
	WorkspaceID        string
	BundleID           string
	CommitID           string
	CommitSHA          string
	GeneratedByAgentID string
	ActorChain         audit.ActorChain
}

func (s Service) GenerateRevertPlan(ctx context.Context, cmd GenerateRevertPlanCommand) (*change.RevertPlan, error) {
	if err := s.requireRevertPorts(); err != nil {
		return nil, err
	}
	now := s.Clock.Now().UTC()
	steps := []string{}
	if cmd.CommitSHA != "" {
		steps = append(steps, fmt.Sprintf("git revert %s", cmd.CommitSHA))
	}
	if cmd.BundleID != "" {
		steps = append(steps, "revert bundle items in reverse merge order")
	}
	plan, err := change.NewRevertPlan(change.RevertPlan{
		ID:                 s.IDs.NewID(),
		BundleID:           cmd.BundleID,
		CommitID:           cmd.CommitID,
		Kind:               change.RevertCommit,
		Steps:              steps,
		GeneratedByAgentID: cmd.GeneratedByAgentID,
		CreatedAt:          now,
		UpdatedAt:          now,
	})
	if err != nil {
		return nil, err
	}
	if err := s.RevertPlans.Save(ctx, plan); err != nil {
		return nil, err
	}
	if err := s.record(ctx, cmd.ActorChain, cmd.OrgID, cmd.WorkspaceID, audit.EventRevertPlanGenerated, "revert_plan", plan.ID, "generate_revert_plan"); err != nil {
		return nil, err
	}
	return plan, nil
}

func (s Service) requireBundlePorts() error {
	if s.Bundles == nil || s.Audit == nil || s.IDs == nil || s.Clock == nil {
		return errors.New("changes bundle ports are required")
	}
	return nil
}

func (s Service) requireCommitPorts() error {
	if s.Commits == nil || s.Audit == nil || s.IDs == nil || s.Clock == nil {
		return errors.New("changes commit ports are required")
	}
	return nil
}

func (s Service) requireRevertPorts() error {
	if s.RevertPlans == nil || s.Audit == nil || s.IDs == nil || s.Clock == nil {
		return errors.New("changes revert ports are required")
	}
	return nil
}

func (s Service) record(ctx context.Context, chain audit.ActorChain, orgID, workspaceID string, eventType audit.EventType, subjectKind, subjectID, action string) error {
	if err := chain.Validate(); err != nil {
		return err
	}
	event, err := audit.NewEvent(audit.EventSpec{
		ID:           s.IDs.NewID(),
		OrgID:        orgID,
		WorkspaceID:  workspaceID,
		ActorChainID: chain.ID,
		Type:         eventType,
		SubjectKind:  subjectKind,
		SubjectID:    subjectID,
		Action:       action,
		Decision:     audit.DecisionRecorded,
		OccurredAt:   s.Clock.Now().UTC(),
	})
	if err != nil {
		return err
	}
	return s.Audit.Record(ctx, event)
}
