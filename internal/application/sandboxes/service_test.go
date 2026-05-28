package sandboxes

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/sandbox"
)

func TestRunCommandPersistsInvocationRunArtifactsAndAudit(t *testing.T) {
	ctx := context.Background()
	svc, stores := newTestService()
	chain := agentChain(t, stores.clock.now)

	result, err := svc.RunCommand(ctx, RunCommandRequest{
		OrgID:       "org-1",
		WorkspaceID: "workspace-1",
		AgentRunID:  "run-1",
		AgentID:     "agent-1",
		SandboxID:   "sandbox-1",
		PolicyID:    "policy-1",
		ActorChain:  chain,
		Command: sandbox.CommandRequest{
			Command: "go",
			Args:    []string{"test", "./..."},
			CWD:     ".",
		},
	})
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.CommandRun.Status != sandbox.CommandSucceeded {
		t.Fatalf("expected succeeded command, got %s", result.CommandRun.Status)
	}
	if len(stores.artifacts.artifacts) != 2 {
		t.Fatalf("expected stdout/stderr artifacts, got %d", len(stores.artifacts.artifacts))
	}
	if stores.audit.events[0].Type != audit.EventCommandExecuted {
		t.Fatalf("unexpected audit event: %s", stores.audit.events[0].Type)
	}
}

func TestRunCommandDeniedByPolicyStillAudits(t *testing.T) {
	ctx := context.Background()
	svc, stores := newTestService()
	chain := agentChain(t, stores.clock.now)

	_, err := svc.RunCommand(ctx, RunCommandRequest{
		OrgID:       "org-1",
		WorkspaceID: "workspace-1",
		AgentRunID:  "run-1",
		AgentID:     "agent-1",
		SandboxID:   "sandbox-1",
		PolicyID:    "policy-1",
		ActorChain:  chain,
		Command: sandbox.CommandRequest{
			Command:         "curl",
			Args:            []string{"https://example.com"},
			NetworkRequired: true,
		},
	})
	if err == nil {
		t.Fatal("expected network policy denial")
	}
	if len(stores.invocations.invocations) != 1 || stores.invocations.invocations[0].Status != sandbox.ToolFailed {
		t.Fatalf("expected failed invocation to be stored")
	}
	if stores.audit.events[0].Decision != audit.DecisionDenied {
		t.Fatalf("expected denied audit decision, got %s", stores.audit.events[0].Decision)
	}
}

func TestRunCommandNonZeroExitMarksFailure(t *testing.T) {
	ctx := context.Background()
	svc, stores := newTestService()
	stores.executor.result = CommandResult{ExitCode: 2, Stderr: "failed"}
	chain := agentChain(t, stores.clock.now)

	result, err := svc.RunCommand(ctx, RunCommandRequest{
		OrgID:       "org-1",
		WorkspaceID: "workspace-1",
		AgentRunID:  "run-1",
		AgentID:     "agent-1",
		SandboxID:   "sandbox-1",
		PolicyID:    "policy-1",
		ActorChain:  chain,
		Command:     sandbox.CommandRequest{Command: "go", Args: []string{"test"}},
	})
	if err != nil {
		t.Fatalf("non-zero exit should be captured without executor error: %v", err)
	}
	if result.CommandRun.Status != sandbox.CommandFailed {
		t.Fatalf("expected failed command, got %s", result.CommandRun.Status)
	}
}

type testStores struct {
	policies    *memoryPolicyStore
	instances   *memoryInstanceStore
	invocations *memoryToolInvocationStore
	runs        *memoryCommandRunStore
	artifacts   *memoryArtifactStore
	executor    *fakeExecutor
	audit       *memoryAuditRecorder
	clock       *fixedClock
}

func newTestService() (Service, testStores) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	policy, err := sandbox.NewPolicy(sandbox.Policy{
		ID:              "policy-1",
		OrgID:           "org-1",
		Name:            "default",
		WritablePaths:   []string{"."},
		CreatedByUserID: "user-1",
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	if err != nil {
		panic(err)
	}
	instance, err := sandbox.NewInstance(sandbox.Instance{
		ID:          "sandbox-1",
		WorkspaceID: "workspace-1",
		PolicyID:    "policy-1",
		RunnerID:    "runner-1",
		CreatedAt:   now,
	})
	if err != nil {
		panic(err)
	}
	if err := instance.Start(now); err != nil {
		panic(err)
	}
	stores := testStores{
		policies:    &memoryPolicyStore{policies: map[string]*sandbox.Policy{"policy-1": policy}},
		instances:   &memoryInstanceStore{instances: map[string]*sandbox.Instance{"sandbox-1": instance}},
		invocations: &memoryToolInvocationStore{},
		runs:        &memoryCommandRunStore{},
		artifacts:   &memoryArtifactStore{},
		executor:    &fakeExecutor{result: CommandResult{ExitCode: 0, Stdout: "ok\n"}},
		audit:       &memoryAuditRecorder{},
		clock:       &fixedClock{now: now},
	}
	return Service{
		Policies:        stores.policies,
		Instances:       stores.instances,
		ToolInvocations: stores.invocations,
		CommandRuns:     stores.runs,
		Artifacts:       stores.artifacts,
		Executor:        stores.executor,
		Audit:           stores.audit,
		IDs:             &sequenceIDs{},
		Clock:           stores.clock,
	}, stores
}

func agentChain(t *testing.T, now time.Time) audit.ActorChain {
	t.Helper()
	chain, err := audit.NewAgentActorChain("chain-1", "org-1", "workspace-1", "user-1", "agent-1", now)
	if err != nil {
		t.Fatalf("new chain: %v", err)
	}
	return chain
}

type memoryPolicyStore struct{ policies map[string]*sandbox.Policy }

func (s *memoryPolicyStore) Save(_ context.Context, policy *sandbox.Policy) error {
	s.policies[policy.ID] = policy
	return nil
}

func (s *memoryPolicyStore) Find(_ context.Context, id string) (*sandbox.Policy, error) {
	policy := s.policies[id]
	if policy == nil {
		return nil, errors.New("policy not found")
	}
	return policy, nil
}

type memoryInstanceStore struct{ instances map[string]*sandbox.Instance }

func (s *memoryInstanceStore) Save(_ context.Context, instance *sandbox.Instance) error {
	s.instances[instance.ID] = instance
	return nil
}

func (s *memoryInstanceStore) Find(_ context.Context, id string) (*sandbox.Instance, error) {
	instance := s.instances[id]
	if instance == nil {
		return nil, errors.New("instance not found")
	}
	return instance, nil
}

type memoryToolInvocationStore struct{ invocations []*sandbox.ToolInvocation }

func (s *memoryToolInvocationStore) Save(_ context.Context, invocation *sandbox.ToolInvocation) error {
	s.invocations = append(s.invocations, invocation)
	return nil
}

type memoryCommandRunStore struct{ runs []*sandbox.CommandRun }

func (s *memoryCommandRunStore) Save(_ context.Context, run *sandbox.CommandRun) error {
	s.runs = append(s.runs, run)
	return nil
}

type memoryArtifactStore struct{ artifacts []*sandbox.Artifact }

func (s *memoryArtifactStore) Save(_ context.Context, artifact *sandbox.Artifact) error {
	s.artifacts = append(s.artifacts, artifact)
	return nil
}

type fakeExecutor struct{ result CommandResult }

func (e *fakeExecutor) Execute(context.Context, sandbox.CommandRequest) (CommandResult, error) {
	return e.result, nil
}

type memoryAuditRecorder struct{ events []*audit.Event }

func (r *memoryAuditRecorder) Record(_ context.Context, event *audit.Event) error {
	r.events = append(r.events, event)
	return nil
}

type sequenceIDs struct{ next int }

func (g *sequenceIDs) NewID() string {
	g.next++
	return "id-" + string(rune('0'+g.next))
}

type fixedClock struct{ now time.Time }

func (c *fixedClock) Now() time.Time { return c.now }
