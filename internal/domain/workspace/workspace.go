package workspace

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type RequirementKind string

const (
	RequirementIssue    RequirementKind = "issue"
	RequirementIncident RequirementKind = "incident"
	RequirementManual   RequirementKind = "manual"
	RequirementAPI      RequirementKind = "api"
)

type Status string

const (
	StatusCreated         Status = "created"
	StatusRunning         Status = "running"
	StatusWaitingApproval Status = "waiting_approval"
	StatusBlocked         Status = "blocked"
	StatusCompleted       Status = "completed"
	StatusCancelled       Status = "cancelled"
)

type ApprovalState string

const (
	ApprovalNone     ApprovalState = "none"
	ApprovalPending  ApprovalState = "pending"
	ApprovalApproved ApprovalState = "approved"
	ApprovalDenied   ApprovalState = "denied"
	ApprovalExpired  ApprovalState = "expired"
)

type RepositoryRole string

const (
	RepositoryPrimary    RepositoryRole = "primary"
	RepositoryDependency RepositoryRole = "dependency"
	RepositoryGenerated  RepositoryRole = "generated"
	RepositoryReadonly   RepositoryRole = "readonly"
)

type AgentRole string

const (
	AgentCoordinator AgentRole = "coordinator"
	AgentImplementer AgentRole = "implementer"
	AgentReviewer    AgentRole = "reviewer"
	AgentFixer       AgentRole = "fixer"
)

type AgentStatus string

const (
	AgentAssigned AgentStatus = "assigned"
	AgentActive   AgentStatus = "active"
	AgentPaused   AgentStatus = "paused"
	AgentDone     AgentStatus = "done"
)

type BranchStatus string

const (
	BranchPlanned BranchStatus = "planned"
	BranchCreated BranchStatus = "created"
	BranchPushed  BranchStatus = "pushed"
	BranchStale   BranchStatus = "stale"
	BranchMerged  BranchStatus = "merged"
	BranchDeleted BranchStatus = "deleted"
)

type Requirement struct {
	Kind RequirementKind
	Ref  string
}

type RepositoryBinding struct {
	RepoID       string
	Role         RepositoryRole
	BaseBranch   string
	TargetBranch string
	CreatedAt    time.Time
}

type AgentAssignment struct {
	AgentID          string
	Role             AgentRole
	Status           AgentStatus
	AssignedByUserID string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Branch struct {
	ID               string
	RepoID           string
	Name             string
	BaseBranch       string
	BaseSHA          string
	HeadSHA          string
	Status           BranchStatus
	CreatedByAgentID string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Workspace struct {
	ID              string
	OrgID           string
	Key             string
	Title           string
	Description     string
	Requirement     Requirement
	Status          Status
	ApprovalState   ApprovalState
	CreatedByUserID string
	PrimaryAgentID  string
	SandboxPolicyID string
	MemoryScope     map[string]string
	Repositories    []RepositoryBinding
	Agents          []AgentAssignment
	Branches        []Branch
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ClosedAt        *time.Time
	Metadata        map[string]string
}

type Spec struct {
	ID              string
	OrgID           string
	Key             string
	Title           string
	Description     string
	Requirement     Requirement
	CreatedByUserID string
	PrimaryAgentID  string
	SandboxPolicyID string
	MemoryScope     map[string]string
	CreatedAt       time.Time
	Metadata        map[string]string
}

var keyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}-[1-9][0-9]{0,8}$`)

func New(spec Spec) (*Workspace, error) {
	workspace := &Workspace{
		ID:              strings.TrimSpace(spec.ID),
		OrgID:           strings.TrimSpace(spec.OrgID),
		Key:             strings.TrimSpace(spec.Key),
		Title:           strings.TrimSpace(spec.Title),
		Description:     strings.TrimSpace(spec.Description),
		Requirement:     spec.Requirement,
		Status:          StatusCreated,
		ApprovalState:   ApprovalNone,
		CreatedByUserID: strings.TrimSpace(spec.CreatedByUserID),
		PrimaryAgentID:  strings.TrimSpace(spec.PrimaryAgentID),
		SandboxPolicyID: strings.TrimSpace(spec.SandboxPolicyID),
		MemoryScope:     cloneMap(spec.MemoryScope),
		CreatedAt:       spec.CreatedAt,
		UpdatedAt:       spec.CreatedAt,
		Metadata:        cloneMap(spec.Metadata),
	}
	return workspace, workspace.Validate()
}

func (w Workspace) Validate() error {
	if w.ID == "" {
		return errors.New("workspace id is required")
	}
	if w.OrgID == "" {
		return errors.New("workspace org id is required")
	}
	if !keyPattern.MatchString(w.Key) {
		return fmt.Errorf("workspace key %q must look like AGH-1", w.Key)
	}
	if w.Title == "" {
		return errors.New("workspace title is required")
	}
	if w.CreatedByUserID == "" {
		return errors.New("workspace creator user id is required")
	}
	if err := w.Requirement.Validate(); err != nil {
		return err
	}
	if err := validateStatus(w.Status); err != nil {
		return err
	}
	if err := validateApprovalState(w.ApprovalState); err != nil {
		return err
	}
	if w.CreatedAt.IsZero() {
		return errors.New("workspace created_at is required")
	}
	if w.UpdatedAt.IsZero() {
		return errors.New("workspace updated_at is required")
	}
	if w.UpdatedAt.Before(w.CreatedAt) {
		return errors.New("workspace updated_at cannot be before created_at")
	}
	if w.ClosedAt != nil && w.ClosedAt.Before(w.CreatedAt) {
		return errors.New("workspace closed_at cannot be before created_at")
	}
	for _, repo := range w.Repositories {
		if err := repo.Validate(); err != nil {
			return err
		}
	}
	for _, assignment := range w.Agents {
		if err := assignment.Validate(); err != nil {
			return err
		}
	}
	for _, branch := range w.Branches {
		if err := branch.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (r Requirement) Validate() error {
	switch r.Kind {
	case RequirementIssue, RequirementIncident, RequirementManual, RequirementAPI:
	default:
		return fmt.Errorf("unsupported requirement kind %q", r.Kind)
	}
	if r.Kind != RequirementManual && strings.TrimSpace(r.Ref) == "" {
		return errors.New("requirement ref is required for non-manual requirements")
	}
	return nil
}

func (r RepositoryBinding) Validate() error {
	if r.RepoID == "" {
		return errors.New("workspace repository repo id is required")
	}
	switch r.Role {
	case RepositoryPrimary, RepositoryDependency, RepositoryGenerated, RepositoryReadonly:
	default:
		return fmt.Errorf("unsupported workspace repository role %q", r.Role)
	}
	if r.BaseBranch == "" {
		return errors.New("workspace repository base branch is required")
	}
	if r.TargetBranch == "" {
		return errors.New("workspace repository target branch is required")
	}
	if r.CreatedAt.IsZero() {
		return errors.New("workspace repository created_at is required")
	}
	return nil
}

func (a AgentAssignment) Validate() error {
	if a.AgentID == "" {
		return errors.New("workspace agent id is required")
	}
	switch a.Role {
	case AgentCoordinator, AgentImplementer, AgentReviewer, AgentFixer:
	default:
		return fmt.Errorf("unsupported workspace agent role %q", a.Role)
	}
	switch a.Status {
	case AgentAssigned, AgentActive, AgentPaused, AgentDone:
	default:
		return fmt.Errorf("unsupported workspace agent status %q", a.Status)
	}
	if a.AssignedByUserID == "" {
		return errors.New("workspace agent assigned_by_user_id is required")
	}
	if a.CreatedAt.IsZero() {
		return errors.New("workspace agent created_at is required")
	}
	if a.UpdatedAt.IsZero() {
		return errors.New("workspace agent updated_at is required")
	}
	return nil
}

func (b Branch) Validate() error {
	if b.ID == "" {
		return errors.New("workspace branch id is required")
	}
	if b.RepoID == "" {
		return errors.New("workspace branch repo id is required")
	}
	if b.Name == "" {
		return errors.New("workspace branch name is required")
	}
	if b.BaseBranch == "" {
		return errors.New("workspace branch base branch is required")
	}
	if b.BaseSHA == "" {
		return errors.New("workspace branch base sha is required")
	}
	switch b.Status {
	case BranchPlanned, BranchCreated, BranchPushed, BranchStale, BranchMerged, BranchDeleted:
	default:
		return fmt.Errorf("unsupported workspace branch status %q", b.Status)
	}
	if b.CreatedByAgentID == "" {
		return errors.New("workspace branch created_by_agent_id is required")
	}
	if b.CreatedAt.IsZero() {
		return errors.New("workspace branch created_at is required")
	}
	if b.UpdatedAt.IsZero() {
		return errors.New("workspace branch updated_at is required")
	}
	return nil
}

func (w *Workspace) BindRepository(repo RepositoryBinding, now time.Time) error {
	if w == nil {
		return errors.New("workspace is required")
	}
	if err := w.ensureMutable(); err != nil {
		return err
	}
	if now.IsZero() {
		return errors.New("repository binding time is required")
	}
	repo.CreatedAt = now
	if err := repo.Validate(); err != nil {
		return err
	}
	for _, existing := range w.Repositories {
		if existing.RepoID == repo.RepoID {
			return fmt.Errorf("repository %s is already bound to workspace", repo.RepoID)
		}
		if repo.Role == RepositoryPrimary && existing.Role == RepositoryPrimary {
			return errors.New("workspace can only have one primary repository")
		}
	}
	w.Repositories = append(w.Repositories, repo)
	w.UpdatedAt = now
	return nil
}

func (w *Workspace) AssignAgent(assignment AgentAssignment, now time.Time) error {
	if w == nil {
		return errors.New("workspace is required")
	}
	if err := w.ensureMutable(); err != nil {
		return err
	}
	if now.IsZero() {
		return errors.New("agent assignment time is required")
	}
	assignment.Status = AgentAssigned
	assignment.CreatedAt = now
	assignment.UpdatedAt = now
	if err := assignment.Validate(); err != nil {
		return err
	}
	for _, existing := range w.Agents {
		if existing.AgentID == assignment.AgentID && existing.Role == assignment.Role {
			return fmt.Errorf("agent %s is already assigned as %s", assignment.AgentID, assignment.Role)
		}
	}
	if assignment.Role == AgentCoordinator || assignment.Role == AgentImplementer {
		w.PrimaryAgentID = assignment.AgentID
	}
	w.Agents = append(w.Agents, assignment)
	w.UpdatedAt = now
	return nil
}

func (w *Workspace) PlanBranch(branch Branch, now time.Time) error {
	if w == nil {
		return errors.New("workspace is required")
	}
	if err := w.ensureMutable(); err != nil {
		return err
	}
	if now.IsZero() {
		return errors.New("branch planning time is required")
	}
	if !w.HasRepository(branch.RepoID) {
		return fmt.Errorf("repository %s is not bound to workspace", branch.RepoID)
	}
	branch.Status = BranchPlanned
	branch.CreatedAt = now
	branch.UpdatedAt = now
	if err := branch.Validate(); err != nil {
		return err
	}
	for _, existing := range w.Branches {
		if existing.RepoID == branch.RepoID && existing.Name == branch.Name {
			return fmt.Errorf("branch %s already planned for repo %s", branch.Name, branch.RepoID)
		}
	}
	w.Branches = append(w.Branches, branch)
	w.UpdatedAt = now
	return nil
}

func (w Workspace) HasRepository(repoID string) bool {
	for _, repo := range w.Repositories {
		if repo.RepoID == repoID {
			return true
		}
	}
	return false
}

func (w *Workspace) Transition(to Status, now time.Time) error {
	if w == nil {
		return errors.New("workspace is required")
	}
	if now.IsZero() {
		return errors.New("workspace transition time is required")
	}
	if !canTransition(w.Status, to) {
		return fmt.Errorf("cannot transition workspace from %s to %s", w.Status, to)
	}
	w.Status = to
	w.UpdatedAt = now
	if to == StatusCompleted || to == StatusCancelled {
		w.ClosedAt = &now
	}
	return nil
}

func (w *Workspace) SetApprovalState(state ApprovalState, now time.Time) error {
	if w == nil {
		return errors.New("workspace is required")
	}
	if err := w.ensureMutable(); err != nil {
		return err
	}
	if now.IsZero() {
		return errors.New("approval state time is required")
	}
	if err := validateApprovalState(state); err != nil {
		return err
	}
	w.ApprovalState = state
	w.UpdatedAt = now
	return nil
}

func (w Workspace) ensureMutable() error {
	if w.Status == StatusCompleted || w.Status == StatusCancelled {
		return fmt.Errorf("workspace %s is closed", w.ID)
	}
	return nil
}

func canTransition(from, to Status) bool {
	if from == to {
		return true
	}
	switch from {
	case StatusCreated:
		return to == StatusRunning || to == StatusWaitingApproval || to == StatusBlocked || to == StatusCancelled
	case StatusRunning:
		return to == StatusWaitingApproval || to == StatusBlocked || to == StatusCompleted || to == StatusCancelled
	case StatusWaitingApproval:
		return to == StatusRunning || to == StatusBlocked || to == StatusCancelled
	case StatusBlocked:
		return to == StatusRunning || to == StatusCancelled
	case StatusCompleted, StatusCancelled:
		return false
	default:
		return false
	}
}

func validateStatus(status Status) error {
	switch status {
	case StatusCreated, StatusRunning, StatusWaitingApproval, StatusBlocked, StatusCompleted, StatusCancelled:
		return nil
	default:
		return fmt.Errorf("unsupported workspace status %q", status)
	}
}

func validateApprovalState(state ApprovalState) error {
	switch state {
	case ApprovalNone, ApprovalPending, ApprovalApproved, ApprovalDenied, ApprovalExpired:
		return nil
	default:
		return fmt.Errorf("unsupported workspace approval state %q", state)
	}
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
