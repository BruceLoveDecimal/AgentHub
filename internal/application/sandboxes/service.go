package sandboxes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/sandbox"
	sandboxsvc "github.com/BruceLoveDecimal/AgentHub/internal/service/sandbox"
)

type Service struct {
	Policies        PolicyStore
	Instances       InstanceStore
	ToolInvocations ToolInvocationStore
	CommandRuns     CommandRunStore
	Artifacts       ArtifactStore
	Executor        CommandExecutor
	Audit           AuditRecorder
	IDs             IDGenerator
	Clock           Clock
	Policy          sandboxsvc.Service
}

type CreatePolicyCommand struct {
	OrgID             string
	Name              string
	NetworkAllowed    bool
	SecretsAllowed    bool
	PrivilegedAllowed bool
	WritablePaths     []string
	CreatedByUserID   string
}

func (s Service) CreatePolicy(ctx context.Context, cmd CreatePolicyCommand) (*sandbox.Policy, error) {
	if err := s.requireCommonPorts(); err != nil {
		return nil, err
	}
	if s.Policies == nil {
		return nil, errors.New("policy store is required")
	}
	now := s.Clock.Now().UTC()
	policy, err := sandbox.NewPolicy(sandbox.Policy{
		ID:                s.IDs.NewID(),
		OrgID:             cmd.OrgID,
		Name:              cmd.Name,
		NetworkAllowed:    cmd.NetworkAllowed,
		SecretsAllowed:    cmd.SecretsAllowed,
		PrivilegedAllowed: cmd.PrivilegedAllowed,
		WritablePaths:     cmd.WritablePaths,
		CreatedByUserID:   cmd.CreatedByUserID,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		return nil, err
	}
	if err := s.Policies.Save(ctx, policy); err != nil {
		return nil, err
	}
	return policy, nil
}

type CreateInstanceCommand struct {
	WorkspaceID string
	AgentRunID  string
	PolicyID    string
	RunnerID    string
	ImageRef    string
	Metadata    map[string]string
}

func (s Service) CreateInstance(ctx context.Context, cmd CreateInstanceCommand) (*sandbox.Instance, error) {
	if err := s.requireCommonPorts(); err != nil {
		return nil, err
	}
	if s.Instances == nil {
		return nil, errors.New("instance store is required")
	}
	now := s.Clock.Now().UTC()
	instance, err := sandbox.NewInstance(sandbox.Instance{
		ID:          s.IDs.NewID(),
		WorkspaceID: cmd.WorkspaceID,
		AgentRunID:  cmd.AgentRunID,
		PolicyID:    cmd.PolicyID,
		RunnerID:    cmd.RunnerID,
		ImageRef:    cmd.ImageRef,
		CreatedAt:   now,
		Metadata:    cmd.Metadata,
	})
	if err != nil {
		return nil, err
	}
	if err := instance.Start(now); err != nil {
		return nil, err
	}
	if err := s.Instances.Save(ctx, instance); err != nil {
		return nil, err
	}
	return instance, nil
}

type RunCommandRequest struct {
	OrgID       string
	WorkspaceID string
	AgentRunID  string
	AgentID     string
	SandboxID   string
	PolicyID    string
	ActorChain  audit.ActorChain
	Command     sandbox.CommandRequest
}

type RunCommandResult struct {
	Invocation *sandbox.ToolInvocation
	CommandRun *sandbox.CommandRun
	Stdout     *sandbox.Artifact
	Stderr     *sandbox.Artifact
}

func (s Service) RunCommand(ctx context.Context, req RunCommandRequest) (*RunCommandResult, error) {
	if err := s.requireRunPorts(); err != nil {
		return nil, err
	}
	if err := req.ActorChain.Validate(); err != nil {
		return nil, err
	}
	policy, err := s.Policies.Find(ctx, req.PolicyID)
	if err != nil {
		return nil, err
	}
	instance, err := s.Instances.Find(ctx, req.SandboxID)
	if err != nil {
		return nil, err
	}
	now := s.Clock.Now().UTC()
	inputHash := sandbox.HashContent(fmt.Sprintf("%s %v %s", req.Command.Command, req.Command.Args, req.Command.CWD))
	invocation, err := sandbox.NewToolInvocation(sandbox.ToolInvocation{
		ID:           s.IDs.NewID(),
		WorkspaceID:  req.WorkspaceID,
		AgentRunID:   req.AgentRunID,
		AgentID:      req.AgentID,
		ActorChainID: req.ActorChain.ID,
		ToolName:     "shell.exec",
		ToolType:     sandbox.ToolShell,
		InputHash:    inputHash,
		StartedAt:    now,
	})
	if err != nil {
		return nil, err
	}
	run, err := sandbox.NewCommandRun(sandbox.CommandRun{
		ID:               s.IDs.NewID(),
		WorkspaceID:      req.WorkspaceID,
		SandboxID:        req.SandboxID,
		ToolInvocationID: invocation.ID,
		Request:          req.Command,
		NetworkAllowed:   policy.NetworkAllowed,
		SecretsAllowed:   policy.SecretsAllowed,
		Privileged:       req.Command.Privileged,
		CreatedAt:        now,
	})
	if err != nil {
		return nil, err
	}
	if err := s.Policy.ValidateCommand(*policy, *instance, req.Command); err != nil {
		invocation.Status = sandbox.ToolFailed
		invocation.Error = err.Error()
		run.Status = sandbox.CommandFailed
		if saveErr := s.persistDenied(ctx, req, invocation, run, err); saveErr != nil {
			return nil, saveErr
		}
		return nil, err
	}

	run.Start(now)
	if err := s.CommandRuns.Save(ctx, run); err != nil {
		return nil, err
	}
	result, execErr := s.Executor.Execute(ctx, req.Command)
	finishedAt := s.Clock.Now().UTC()
	if execErr != nil {
		result.ExitCode = 1
		result.Stderr = execErr.Error()
	}
	stdout, err := s.artifact(ctx, req.OrgID, req.WorkspaceID, run.ID, sandbox.ArtifactLog, "stdout.log", result.Stdout, finishedAt)
	if err != nil {
		return nil, err
	}
	stderr, err := s.artifact(ctx, req.OrgID, req.WorkspaceID, run.ID, sandbox.ArtifactLog, "stderr.log", result.Stderr, finishedAt)
	if err != nil {
		return nil, err
	}
	run.Finish(result.ExitCode, stdout.StorageURI, stderr.StorageURI, finishedAt)
	if result.ExitCode == 0 && execErr == nil {
		invocation.Status = sandbox.ToolSucceeded
	} else {
		invocation.Status = sandbox.ToolFailed
		if execErr != nil {
			invocation.Error = execErr.Error()
		}
	}
	invocation.FinishedAt = &finishedAt
	invocation.OutputRef = stdout.StorageURI
	if err := s.CommandRuns.Save(ctx, run); err != nil {
		return nil, err
	}
	if err := s.ToolInvocations.Save(ctx, invocation); err != nil {
		return nil, err
	}
	if err := s.record(ctx, req, audit.DecisionRecorded, "command_executed", run.ID); err != nil {
		return nil, err
	}
	return &RunCommandResult{Invocation: invocation, CommandRun: run, Stdout: stdout, Stderr: stderr}, execErr
}

func (s Service) persistDenied(ctx context.Context, req RunCommandRequest, invocation *sandbox.ToolInvocation, run *sandbox.CommandRun, cause error) error {
	if err := s.ToolInvocations.Save(ctx, invocation); err != nil {
		return err
	}
	if err := s.CommandRuns.Save(ctx, run); err != nil {
		return err
	}
	return s.record(ctx, req, audit.DecisionDenied, "command_denied", cause.Error())
}

func (s Service) artifact(ctx context.Context, orgID, workspaceID, commandID string, kind sandbox.ArtifactKind, name, content string, now time.Time) (*sandbox.Artifact, error) {
	artifact, err := sandbox.NewArtifact(sandbox.Artifact{
		ID:                  s.IDs.NewID(),
		OrgID:               orgID,
		WorkspaceID:         workspaceID,
		ProducedByCommandID: commandID,
		Kind:                kind,
		Name:                name,
		StorageURI:          "memory://" + commandID + "/" + name,
		ContentHash:         sandbox.HashContent(content),
		SizeBytes:           int64(len(content)),
		MimeType:            "text/plain",
		CreatedAt:           now,
	})
	if err != nil {
		return nil, err
	}
	if err := s.Artifacts.Save(ctx, artifact); err != nil {
		return nil, err
	}
	return artifact, nil
}

func (s Service) requireCommonPorts() error {
	if s.IDs == nil || s.Clock == nil {
		return errors.New("sandbox service id generator and clock are required")
	}
	return nil
}

func (s Service) requireRunPorts() error {
	if err := s.requireCommonPorts(); err != nil {
		return err
	}
	if s.Policies == nil || s.Instances == nil || s.ToolInvocations == nil || s.CommandRuns == nil || s.Artifacts == nil || s.Executor == nil || s.Audit == nil {
		return errors.New("sandbox run ports are required")
	}
	return nil
}

func (s Service) record(ctx context.Context, req RunCommandRequest, decision audit.Decision, action, subjectID string) error {
	event, err := audit.NewEvent(audit.EventSpec{
		ID:           s.IDs.NewID(),
		OrgID:        req.OrgID,
		WorkspaceID:  req.WorkspaceID,
		ActorChainID: req.ActorChain.ID,
		Type:         audit.EventCommandExecuted,
		SubjectKind:  "command_run",
		SubjectID:    subjectID,
		Action:       action,
		Decision:     decision,
		OccurredAt:   s.Clock.Now().UTC(),
	})
	if err != nil {
		return err
	}
	return s.Audit.Record(ctx, event)
}
