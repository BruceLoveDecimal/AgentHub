package workspaces

import (
	"context"
	"errors"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/workspace"
	"github.com/BruceLoveDecimal/AgentHub/internal/service/workflow"
)

type Service struct {
	Workspaces WorkspaceStore
	Audit      AuditRecorder
	IDs        IDGenerator
	Clock      Clock
	Workflow   workflow.Service
}

type CreateWorkspaceCommand struct {
	OrgID           string
	Key             string
	Title           string
	Description     string
	Requirement     workspace.Requirement
	CreatedByUserID string
	PrimaryAgentID  string
	SandboxPolicyID string
	MemoryScope     map[string]string
	Metadata        map[string]string
	Repositories    []BindRepositoryInput
	Agents          []AssignAgentInput
}

type BindRepositoryInput struct {
	RepoID       string
	Role         workspace.RepositoryRole
	BaseBranch   string
	TargetBranch string
}

type AssignAgentInput struct {
	AgentID          string
	Role             workspace.AgentRole
	AssignedByUserID string
}

func (s Service) CreateWorkspace(ctx context.Context, cmd CreateWorkspaceCommand) (*workspace.Workspace, error) {
	if err := s.requirePorts(); err != nil {
		return nil, err
	}
	now := s.Clock.Now().UTC()
	created, err := workspace.New(workspace.Spec{
		ID:              s.IDs.NewID(),
		OrgID:           cmd.OrgID,
		Key:             cmd.Key,
		Title:           cmd.Title,
		Description:     cmd.Description,
		Requirement:     cmd.Requirement,
		CreatedByUserID: cmd.CreatedByUserID,
		PrimaryAgentID:  cmd.PrimaryAgentID,
		SandboxPolicyID: cmd.SandboxPolicyID,
		MemoryScope:     cmd.MemoryScope,
		CreatedAt:       now,
		Metadata:        cmd.Metadata,
	})
	if err != nil {
		return nil, err
	}
	for _, repo := range cmd.Repositories {
		if err := created.BindRepository(workspace.RepositoryBinding{
			RepoID:       repo.RepoID,
			Role:         repo.Role,
			BaseBranch:   repo.BaseBranch,
			TargetBranch: repo.TargetBranch,
		}, now); err != nil {
			return nil, err
		}
	}
	for _, assignment := range cmd.Agents {
		assignedBy := assignment.AssignedByUserID
		if assignedBy == "" {
			assignedBy = cmd.CreatedByUserID
		}
		if err := created.AssignAgent(workspace.AgentAssignment{
			AgentID:          assignment.AgentID,
			Role:             assignment.Role,
			AssignedByUserID: assignedBy,
		}, now); err != nil {
			return nil, err
		}
	}
	if err := s.Workspaces.Save(ctx, created); err != nil {
		return nil, err
	}
	chain, err := audit.NewHumanActorChain(s.IDs.NewID(), cmd.OrgID, created.ID, cmd.CreatedByUserID, now)
	if err != nil {
		return nil, err
	}
	if err := s.record(ctx, audit.EventSpec{
		ID:           s.IDs.NewID(),
		OrgID:        cmd.OrgID,
		WorkspaceID:  created.ID,
		ActorChainID: chain.ID,
		Type:         audit.EventWorkspaceCreated,
		SubjectKind:  "workspace",
		SubjectID:    created.ID,
		Action:       "create_workspace",
		Decision:     audit.DecisionRecorded,
		Payload: map[string]string{
			"key": created.Key,
		},
		OccurredAt: now,
	}); err != nil {
		return nil, err
	}
	return created, nil
}

type BindRepositoryCommand struct {
	WorkspaceID   string
	RepoID        string
	Role          workspace.RepositoryRole
	BaseBranch    string
	TargetBranch  string
	RequestedByID string
}

func (s Service) BindRepository(ctx context.Context, cmd BindRepositoryCommand) (*workspace.Workspace, error) {
	if err := s.requirePorts(); err != nil {
		return nil, err
	}
	loaded, err := s.Workspaces.Find(ctx, cmd.WorkspaceID)
	if err != nil {
		return nil, err
	}
	now := s.Clock.Now().UTC()
	if err := loaded.BindRepository(workspace.RepositoryBinding{
		RepoID:       cmd.RepoID,
		Role:         cmd.Role,
		BaseBranch:   cmd.BaseBranch,
		TargetBranch: cmd.TargetBranch,
	}, now); err != nil {
		return nil, err
	}
	if err := s.Workspaces.Save(ctx, loaded); err != nil {
		return nil, err
	}
	if err := s.recordHumanWorkspaceEvent(ctx, *loaded, cmd.RequestedByID, audit.EventWorkspaceRepositoryBound, "workspace_repository", cmd.RepoID, "bind_repository"); err != nil {
		return nil, err
	}
	return loaded, nil
}

type AssignAgentCommand struct {
	WorkspaceID      string
	AgentID          string
	Role             workspace.AgentRole
	AssignedByUserID string
}

func (s Service) AssignAgent(ctx context.Context, cmd AssignAgentCommand) (*workspace.Workspace, error) {
	if err := s.requirePorts(); err != nil {
		return nil, err
	}
	loaded, err := s.Workspaces.Find(ctx, cmd.WorkspaceID)
	if err != nil {
		return nil, err
	}
	now := s.Clock.Now().UTC()
	if err := loaded.AssignAgent(workspace.AgentAssignment{
		AgentID:          cmd.AgentID,
		Role:             cmd.Role,
		AssignedByUserID: cmd.AssignedByUserID,
	}, now); err != nil {
		return nil, err
	}
	if err := s.Workspaces.Save(ctx, loaded); err != nil {
		return nil, err
	}
	if err := s.recordHumanWorkspaceEvent(ctx, *loaded, cmd.AssignedByUserID, audit.EventWorkspaceAgentAssigned, "workspace_agent", cmd.AgentID, "assign_agent"); err != nil {
		return nil, err
	}
	return loaded, nil
}

type PlanBranchCommand struct {
	WorkspaceID      string
	RepoID           string
	Name             string
	BaseBranch       string
	BaseSHA          string
	CreatedByAgentID string
	RequestedByID    string
}

func (s Service) PlanBranch(ctx context.Context, cmd PlanBranchCommand) (*workspace.Workspace, error) {
	if err := s.requirePorts(); err != nil {
		return nil, err
	}
	loaded, err := s.Workspaces.Find(ctx, cmd.WorkspaceID)
	if err != nil {
		return nil, err
	}
	now := s.Clock.Now().UTC()
	name := cmd.Name
	if name == "" {
		name = s.Workflow.DefaultTaskBranchName(*loaded)
	}
	if err := loaded.PlanBranch(workspace.Branch{
		ID:               s.IDs.NewID(),
		RepoID:           cmd.RepoID,
		Name:             name,
		BaseBranch:       cmd.BaseBranch,
		BaseSHA:          cmd.BaseSHA,
		CreatedByAgentID: cmd.CreatedByAgentID,
	}, now); err != nil {
		return nil, err
	}
	if err := s.Workspaces.Save(ctx, loaded); err != nil {
		return nil, err
	}
	if err := s.recordHumanWorkspaceEvent(ctx, *loaded, cmd.RequestedByID, audit.EventWorkspaceBranchPlanned, "workspace_branch", name, "plan_branch"); err != nil {
		return nil, err
	}
	return loaded, nil
}

type StartWorkspaceCommand struct {
	WorkspaceID   string
	RequestedByID string
}

func (s Service) StartWorkspace(ctx context.Context, cmd StartWorkspaceCommand) (*workspace.Workspace, error) {
	if err := s.requirePorts(); err != nil {
		return nil, err
	}
	loaded, err := s.Workspaces.Find(ctx, cmd.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.Workflow.ValidateReadyToRun(*loaded); err != nil {
		return nil, err
	}
	now := s.Clock.Now().UTC()
	if err := loaded.Transition(workspace.StatusRunning, now); err != nil {
		return nil, err
	}
	if err := s.Workspaces.Save(ctx, loaded); err != nil {
		return nil, err
	}
	if err := s.recordHumanWorkspaceEvent(ctx, *loaded, cmd.RequestedByID, audit.EventWorkspaceStatusChanged, "workspace", loaded.ID, "start_workspace"); err != nil {
		return nil, err
	}
	return loaded, nil
}

func (s Service) requirePorts() error {
	if s.Workspaces == nil {
		return errors.New("workspace store is required")
	}
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

func (s Service) recordHumanWorkspaceEvent(ctx context.Context, w workspace.Workspace, userID string, eventType audit.EventType, subjectKind, subjectID, action string) error {
	if userID == "" {
		userID = w.CreatedByUserID
	}
	now := s.Clock.Now().UTC()
	chain, err := audit.NewHumanActorChain(s.IDs.NewID(), w.OrgID, w.ID, userID, now)
	if err != nil {
		return err
	}
	return s.record(ctx, audit.EventSpec{
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
}

func (s Service) record(ctx context.Context, spec audit.EventSpec) error {
	event, err := audit.NewEvent(spec)
	if err != nil {
		return err
	}
	return s.Audit.Record(ctx, event)
}
