package sandbox

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type InstanceStatus string

const (
	InstanceCreating InstanceStatus = "creating"
	InstanceRunning  InstanceStatus = "running"
	InstancePaused   InstanceStatus = "paused"
	InstanceStopped  InstanceStatus = "stopped"
	InstanceFailed   InstanceStatus = "failed"
)

type ToolStatus string

const (
	ToolStarted   ToolStatus = "started"
	ToolSucceeded ToolStatus = "succeeded"
	ToolFailed    ToolStatus = "failed"
	ToolCancelled ToolStatus = "cancelled"
)

type ToolType string

const (
	ToolShell   ToolType = "shell"
	ToolGit     ToolType = "git"
	ToolSearch  ToolType = "search"
	ToolGitHub  ToolType = "github"
	ToolSandbox ToolType = "sandbox"
)

type CommandStatus string

const (
	CommandQueued    CommandStatus = "queued"
	CommandRunning   CommandStatus = "running"
	CommandSucceeded CommandStatus = "succeeded"
	CommandFailed    CommandStatus = "failed"
	CommandCancelled CommandStatus = "cancelled"
	CommandTimedOut  CommandStatus = "timed_out"
)

type ArtifactKind string

const (
	ArtifactLog        ArtifactKind = "log"
	ArtifactPatch      ArtifactKind = "patch"
	ArtifactSnapshot   ArtifactKind = "snapshot"
	ArtifactTestReport ArtifactKind = "test_report"
	ArtifactIndex      ArtifactKind = "index"
	ArtifactSummary    ArtifactKind = "summary"
)

type Policy struct {
	ID                string
	OrgID             string
	Name              string
	NetworkAllowed    bool
	SecretsAllowed    bool
	PrivilegedAllowed bool
	WritablePaths     []string
	CreatedByUserID   string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func NewPolicy(policy Policy) (*Policy, error) {
	policy.WritablePaths = cloneSlice(policy.WritablePaths)
	return &policy, policy.Validate()
}

func (p Policy) Validate() error {
	if p.ID == "" || p.OrgID == "" || p.Name == "" {
		return errors.New("sandbox policy id, org id, and name are required")
	}
	if p.CreatedByUserID == "" {
		return errors.New("sandbox policy creator is required")
	}
	if p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		return errors.New("sandbox policy timestamps are required")
	}
	for _, path := range p.WritablePaths {
		if !safePath(path) {
			return fmt.Errorf("unsafe writable path %q", path)
		}
	}
	return nil
}

func (p Policy) Allows(req CommandRequest) error {
	if req.NetworkRequired && !p.NetworkAllowed {
		return errors.New("network access is denied by sandbox policy")
	}
	if req.SecretsRequired && !p.SecretsAllowed {
		return errors.New("secret access is denied by sandbox policy")
	}
	if req.Privileged && !p.PrivilegedAllowed {
		return errors.New("privileged command is denied by sandbox policy")
	}
	if req.WritePath != "" && !p.allowsWritePath(req.WritePath) {
		return fmt.Errorf("write path %q is denied by sandbox policy", req.WritePath)
	}
	return nil
}

func (p Policy) allowsWritePath(path string) bool {
	if len(p.WritablePaths) == 0 {
		return path == ""
	}
	for _, allowed := range p.WritablePaths {
		if allowed == "." || strings.HasPrefix(filepath.Clean(path), filepath.Clean(allowed)) {
			return true
		}
	}
	return false
}

type Instance struct {
	ID                string
	WorkspaceID       string
	AgentRunID        string
	PolicyID          string
	RunnerID          string
	ImageRef          string
	Status            InstanceStatus
	InitialSnapshotID string
	CurrentSnapshotID string
	StartedAt         *time.Time
	StoppedAt         *time.Time
	CreatedAt         time.Time
	Metadata          map[string]string
}

func NewInstance(instance Instance) (*Instance, error) {
	if instance.Status == "" {
		instance.Status = InstanceCreating
	}
	instance.Metadata = cloneMap(instance.Metadata)
	return &instance, instance.Validate()
}

func (i Instance) Validate() error {
	if i.ID == "" || i.WorkspaceID == "" || i.PolicyID == "" || i.RunnerID == "" {
		return errors.New("sandbox instance id, workspace id, policy id, and runner id are required")
	}
	switch i.Status {
	case InstanceCreating, InstanceRunning, InstancePaused, InstanceStopped, InstanceFailed:
	default:
		return fmt.Errorf("unsupported sandbox instance status %q", i.Status)
	}
	if i.CreatedAt.IsZero() {
		return errors.New("sandbox instance created_at is required")
	}
	return nil
}

func (i *Instance) Start(now time.Time) error {
	if i == nil {
		return errors.New("sandbox instance is required")
	}
	if i.Status != InstanceCreating && i.Status != InstancePaused {
		return fmt.Errorf("cannot start sandbox instance in status %s", i.Status)
	}
	i.Status = InstanceRunning
	i.StartedAt = &now
	return nil
}

func (i Instance) CanRunCommand() error {
	if i.Status != InstanceRunning {
		return fmt.Errorf("sandbox instance %s is not running", i.ID)
	}
	return nil
}

type CommandRequest struct {
	Command         string
	Args            []string
	CWD             string
	Env             map[string]string
	NetworkRequired bool
	SecretsRequired bool
	Privileged      bool
	WritePath       string
	TimeoutSeconds  int
}

func (r CommandRequest) Validate() error {
	if strings.TrimSpace(r.Command) == "" {
		return errors.New("command is required")
	}
	if r.CWD != "" && !safePath(r.CWD) {
		return fmt.Errorf("unsafe cwd %q", r.CWD)
	}
	if r.WritePath != "" && !safePath(r.WritePath) {
		return fmt.Errorf("unsafe write path %q", r.WritePath)
	}
	if r.TimeoutSeconds < 0 {
		return errors.New("timeout cannot be negative")
	}
	return nil
}

func (r CommandRequest) EnvHash() string {
	if len(r.Env) == 0 {
		return HashContent("")
	}
	var b strings.Builder
	for k, v := range r.Env {
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(v)
		b.WriteString("\n")
	}
	return HashContent(b.String())
}

type ToolInvocation struct {
	ID           string
	WorkspaceID  string
	AgentRunID   string
	AgentID      string
	ActorChainID string
	ToolName     string
	ToolType     ToolType
	InputHash    string
	Status       ToolStatus
	StartedAt    time.Time
	FinishedAt   *time.Time
	OutputRef    string
	Error        string
	Metadata     map[string]string
}

func NewToolInvocation(invocation ToolInvocation) (*ToolInvocation, error) {
	if invocation.Status == "" {
		invocation.Status = ToolStarted
	}
	invocation.Metadata = cloneMap(invocation.Metadata)
	return &invocation, invocation.Validate()
}

func (t ToolInvocation) Validate() error {
	if t.ID == "" || t.WorkspaceID == "" || t.AgentID == "" || t.ActorChainID == "" || t.ToolName == "" {
		return errors.New("tool invocation id, workspace id, agent id, actor chain id, and tool name are required")
	}
	switch t.ToolType {
	case ToolShell, ToolGit, ToolSearch, ToolGitHub, ToolSandbox:
	default:
		return fmt.Errorf("unsupported tool type %q", t.ToolType)
	}
	switch t.Status {
	case ToolStarted, ToolSucceeded, ToolFailed, ToolCancelled:
	default:
		return fmt.Errorf("unsupported tool status %q", t.Status)
	}
	if t.InputHash == "" {
		return errors.New("tool invocation input hash is required")
	}
	if t.StartedAt.IsZero() {
		return errors.New("tool invocation started_at is required")
	}
	return nil
}

type CommandRun struct {
	ID               string
	WorkspaceID      string
	SandboxID        string
	ToolInvocationID string
	Request          CommandRequest
	EnvHash          string
	NetworkAllowed   bool
	SecretsAllowed   bool
	Privileged       bool
	Status           CommandStatus
	ExitCode         *int
	StdoutURI        string
	StderrURI        string
	StartedAt        *time.Time
	FinishedAt       *time.Time
	CreatedAt        time.Time
	Metadata         map[string]string
}

func NewCommandRun(run CommandRun) (*CommandRun, error) {
	if run.Status == "" {
		run.Status = CommandQueued
	}
	run.Request.Args = cloneSlice(run.Request.Args)
	run.Request.Env = cloneMap(run.Request.Env)
	run.Metadata = cloneMap(run.Metadata)
	if run.EnvHash == "" {
		run.EnvHash = run.Request.EnvHash()
	}
	return &run, run.Validate()
}

func (r CommandRun) Validate() error {
	if r.ID == "" || r.WorkspaceID == "" || r.SandboxID == "" || r.ToolInvocationID == "" {
		return errors.New("command run id, workspace id, sandbox id, and tool invocation id are required")
	}
	if err := r.Request.Validate(); err != nil {
		return err
	}
	switch r.Status {
	case CommandQueued, CommandRunning, CommandSucceeded, CommandFailed, CommandCancelled, CommandTimedOut:
	default:
		return fmt.Errorf("unsupported command status %q", r.Status)
	}
	if r.EnvHash == "" {
		return errors.New("command run env hash is required")
	}
	if r.CreatedAt.IsZero() {
		return errors.New("command run created_at is required")
	}
	return nil
}

func (r *CommandRun) Start(now time.Time) {
	r.Status = CommandRunning
	r.StartedAt = &now
}

func (r *CommandRun) Finish(exitCode int, stdoutURI, stderrURI string, now time.Time) {
	r.ExitCode = &exitCode
	r.StdoutURI = stdoutURI
	r.StderrURI = stderrURI
	r.FinishedAt = &now
	if exitCode == 0 {
		r.Status = CommandSucceeded
	} else {
		r.Status = CommandFailed
	}
}

type Artifact struct {
	ID                  string
	OrgID               string
	WorkspaceID         string
	ProducedByCommandID string
	Kind                ArtifactKind
	Name                string
	StorageURI          string
	ContentHash         string
	SizeBytes           int64
	MimeType            string
	CreatedAt           time.Time
	Metadata            map[string]string
}

func NewArtifact(artifact Artifact) (*Artifact, error) {
	artifact.Metadata = cloneMap(artifact.Metadata)
	return &artifact, artifact.Validate()
}

func (a Artifact) Validate() error {
	if a.ID == "" || a.OrgID == "" || a.Kind == "" || a.Name == "" || a.StorageURI == "" || a.ContentHash == "" {
		return errors.New("artifact id, org id, kind, name, storage uri, and content hash are required")
	}
	if a.CreatedAt.IsZero() {
		return errors.New("artifact created_at is required")
	}
	return nil
}

func HashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func safePath(path string) bool {
	if filepath.IsAbs(path) {
		return false
	}
	clean := filepath.Clean(path)
	return clean != ".." && !strings.HasPrefix(clean, "../")
}

func cloneSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	clone := make([]string, len(values))
	copy(clone, values)
	return clone
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
