package branches

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/branch"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/workspace"
	"github.com/BruceLoveDecimal/AgentHub/internal/service/orchestration"
	"github.com/BruceLoveDecimal/AgentHub/internal/service/workflow"
)

type Service struct {
	Workspaces  WorkspaceStore
	Leases      LeaseStore
	Rebase      RebaseQueue
	Audit       AuditRecorder
	IDs         IDGenerator
	Clock       Clock
	Workflow    workflow.Service
	Orchestrate orchestration.Service
}

type PlanTaskBranchCommand struct {
	WorkspaceID       string
	RepoID            string
	BranchName        string
	BaseBranch        string
	BaseSHA           string
	CreatedByAgentID  string
	RequestedByUserID string
	LeaseTTL          time.Duration
}

type PlanTaskBranchResult struct {
	Workspace *workspace.Workspace
	Lease     *branch.Lease
	Branch    workspace.Branch
}

func (s Service) PlanTaskBranch(ctx context.Context, cmd PlanTaskBranchCommand) (*PlanTaskBranchResult, error) {
	if err := s.requirePorts(false); err != nil {
		return nil, err
	}
	loaded, err := s.Workspaces.Find(ctx, cmd.WorkspaceID)
	if err != nil {
		return nil, err
	}
	name := cmd.BranchName
	if name == "" {
		name = s.Workflow.DefaultTaskBranchName(*loaded)
	}
	if active, err := s.Leases.FindActive(ctx, cmd.RepoID, name); err != nil {
		return nil, err
	} else if active != nil && active.IsActive(s.Clock.Now().UTC()) {
		return nil, fmt.Errorf("branch %s already has an active lease", name)
	}
	token, err := s.Leases.NextFencingToken(ctx, cmd.RepoID, name)
	if err != nil {
		return nil, err
	}
	now := s.Clock.Now().UTC()
	ttl := cmd.LeaseTTL
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	lease, err := branch.NewLease(branch.LeaseSpec{
		ID:            s.IDs.NewID(),
		OrgID:         loaded.OrgID,
		WorkspaceID:   loaded.ID,
		RepoID:        cmd.RepoID,
		BranchName:    name,
		Kind:          branch.LeaseBranch,
		HolderAgentID: cmd.CreatedByAgentID,
		FencingToken:  token,
		TTL:           ttl,
		Now:           now,
	})
	if err != nil {
		return nil, err
	}
	planned := workspace.Branch{
		ID:               s.IDs.NewID(),
		RepoID:           cmd.RepoID,
		Name:             name,
		BaseBranch:       cmd.BaseBranch,
		BaseSHA:          cmd.BaseSHA,
		CreatedByAgentID: cmd.CreatedByAgentID,
	}
	if err := loaded.PlanBranch(planned, now); err != nil {
		return nil, err
	}
	planned = loaded.Branches[len(loaded.Branches)-1]
	if err := s.Leases.Save(ctx, lease); err != nil {
		return nil, err
	}
	if err := s.Workspaces.Save(ctx, loaded); err != nil {
		return nil, err
	}
	if err := s.record(ctx, *loaded, cmd.RequestedByUserID, audit.EventBranchLeaseAcquired, "branch_lease", lease.ID, "acquire_branch_lease"); err != nil {
		return nil, err
	}
	return &PlanTaskBranchResult{Workspace: loaded, Lease: lease, Branch: planned}, nil
}

type ReleaseLeaseCommand struct {
	LeaseID           string
	WorkspaceID       string
	RepoID            string
	BranchName        string
	HolderAgentID     string
	RequestedByUserID string
}

func (s Service) ReleaseLease(ctx context.Context, cmd ReleaseLeaseCommand) (*branch.Lease, error) {
	if err := s.requirePorts(false); err != nil {
		return nil, err
	}
	lease, err := s.Leases.FindActive(ctx, cmd.RepoID, cmd.BranchName)
	if err != nil {
		return nil, err
	}
	if lease == nil {
		return nil, errors.New("active branch lease not found")
	}
	now := s.Clock.Now().UTC()
	if err := lease.Release(now, cmd.HolderAgentID); err != nil {
		return nil, err
	}
	if err := s.Leases.Save(ctx, lease); err != nil {
		return nil, err
	}
	return lease, nil
}

type DetectStaleBranchCommand struct {
	WorkspaceID       string
	RepoID            string
	BranchName        string
	CurrentBaseSHA    string
	RequestedByUserID string
}

func (s Service) DetectStaleBranch(ctx context.Context, cmd DetectStaleBranchCommand) (bool, error) {
	if err := s.requirePorts(true); err != nil {
		return false, err
	}
	loaded, err := s.Workspaces.Find(ctx, cmd.WorkspaceID)
	if err != nil {
		return false, err
	}
	for i := range loaded.Branches {
		if loaded.Branches[i].RepoID != cmd.RepoID || loaded.Branches[i].Name != cmd.BranchName {
			continue
		}
		if !s.Orchestrate.IsStale(loaded.Branches[i], cmd.CurrentBaseSHA) {
			return false, nil
		}
		item, err := s.Orchestrate.QueueRebase(loaded.ID, loaded.Branches[i], cmd.CurrentBaseSHA, s.Clock.Now().UTC())
		if err != nil {
			return false, err
		}
		loaded.Branches[i].Status = workspace.BranchStale
		if err := s.Rebase.Enqueue(ctx, item); err != nil {
			return false, err
		}
		if err := s.Workspaces.Save(ctx, loaded); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, errors.New("workspace branch not found")
}

func (s Service) requirePorts(requireRebase bool) error {
	if s.Workspaces == nil || s.Leases == nil || s.Audit == nil || s.IDs == nil || s.Clock == nil {
		return errors.New("branches service ports are required")
	}
	if requireRebase && s.Rebase == nil {
		return errors.New("rebase queue is required")
	}
	return nil
}

func (s Service) record(ctx context.Context, w workspace.Workspace, userID string, eventType audit.EventType, subjectKind, subjectID, action string) error {
	if userID == "" {
		userID = w.CreatedByUserID
	}
	now := s.Clock.Now().UTC()
	chain, err := audit.NewHumanActorChain(s.IDs.NewID(), w.OrgID, w.ID, userID, now)
	if err != nil {
		return err
	}
	event, err := audit.NewEvent(audit.EventSpec{
		ID:           s.IDs.NewID(),
		OrgID:        w.OrgID,
		WorkspaceID:  w.ID,
		ActorChainID: chain.ID,
		Type:         eventType,
		SubjectKind:  subjectKind,
		SubjectID:    subjectID,
		Action:       action,
		Decision:     audit.DecisionRecorded,
		OccurredAt:   now,
	})
	if err != nil {
		return err
	}
	return s.Audit.Record(ctx, event)
}
