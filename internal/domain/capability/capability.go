package capability

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Name string

const (
	BranchCreate           Name = "branch.create"
	BranchPushAgentPattern Name = "branch.push.agent_pattern"
	PullRequestOpen        Name = "pull_request.open"
	IssueCommentEdit       Name = "issue.comment.edit"
	ReviewRequest          Name = "review.request"
	WorkflowTrigger        Name = "workflow.trigger"
	SecretRead             Name = "secret.read"
	NetworkAccess          Name = "network.access"
	ProtectedPathModify    Name = "protected_path.modify"
	CodeGrep               Name = "code.grep"
	CodeReadFile           Name = "code.read_file"
	CodeSymbols            Name = "code.symbols"
	CodeReferences         Name = "code.references"
	CodeOwnership          Name = "code.ownership"
	CodeASTQuery           Name = "code.ast_query"
	CodeDependencies       Name = "code.dependencies"
	CodeHistory            Name = "code.history"
	CodeDiffMap            Name = "code.diff_map"
	CodeTestDiscover       Name = "code.test_discover"
	CodeSemanticSearch     Name = "code.semantic_search"
)

type PrincipalKind string

const (
	PrincipalAgent PrincipalKind = "agent"
	PrincipalUser  PrincipalKind = "user"
	PrincipalTeam  PrincipalKind = "team"
)

type Principal struct {
	Kind PrincipalKind
	ID   string
}

func (p Principal) Validate() error {
	switch p.Kind {
	case PrincipalAgent, PrincipalUser, PrincipalTeam:
	default:
		return fmt.Errorf("unsupported principal kind %q", p.Kind)
	}
	if p.ID == "" {
		return errors.New("principal id is required")
	}
	return nil
}

type ResourceKind string

const (
	ResourceOrg       ResourceKind = "org"
	ResourceRepo      ResourceKind = "repo"
	ResourceWorkspace ResourceKind = "workspace"
	ResourcePath      ResourceKind = "path"
	ResourceBranch    ResourceKind = "branch"
)

type Resource struct {
	OrgID       string
	Kind        ResourceKind
	ID          string
	RepoID      string
	WorkspaceID string
	Path        string
	Branch      string
}

func (r Resource) Validate() error {
	if r.OrgID == "" {
		return errors.New("resource org id is required")
	}
	switch r.Kind {
	case ResourceOrg, ResourceRepo, ResourceWorkspace, ResourcePath, ResourceBranch:
	default:
		return fmt.Errorf("unsupported resource kind %q", r.Kind)
	}
	if r.Kind != ResourceOrg && r.ID == "" {
		return errors.New("resource id is required")
	}
	if (r.Kind == ResourceRepo || r.Kind == ResourcePath || r.Kind == ResourceBranch) && r.RepoID == "" {
		return errors.New("repo id is required for repo-scoped resources")
	}
	return nil
}

type Effect string

const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

type Conditions struct {
	RequiresApproval bool
	Metadata         map[string]string
}

type Grant struct {
	ID              string
	OrgID           string
	Principal       Principal
	Capability      Name
	Effect          Effect
	ResourceKind    ResourceKind
	ResourceID      string
	RepoID          string
	PathGlob        string
	BranchPattern   string
	Conditions      Conditions
	ExpiresAt       *time.Time
	CreatedByUserID string
	CreatedAt       time.Time
	RevokedAt       *time.Time
}

type GrantSpec struct {
	ID              string
	OrgID           string
	Principal       Principal
	Capability      Name
	Effect          Effect
	ResourceKind    ResourceKind
	ResourceID      string
	RepoID          string
	PathGlob        string
	BranchPattern   string
	Conditions      Conditions
	ExpiresAt       *time.Time
	CreatedByUserID string
	CreatedAt       time.Time
}

func NewGrant(spec GrantSpec) (*Grant, error) {
	grant := &Grant{
		ID:              spec.ID,
		OrgID:           spec.OrgID,
		Principal:       spec.Principal,
		Capability:      spec.Capability,
		Effect:          spec.Effect,
		ResourceKind:    spec.ResourceKind,
		ResourceID:      spec.ResourceID,
		RepoID:          spec.RepoID,
		PathGlob:        spec.PathGlob,
		BranchPattern:   spec.BranchPattern,
		Conditions:      cloneConditions(spec.Conditions),
		ExpiresAt:       spec.ExpiresAt,
		CreatedByUserID: spec.CreatedByUserID,
		CreatedAt:       spec.CreatedAt,
	}
	return grant, grant.Validate()
}

func (g Grant) Validate() error {
	if g.ID == "" {
		return errors.New("capability grant id is required")
	}
	if g.OrgID == "" {
		return errors.New("capability grant org id is required")
	}
	if err := g.Principal.Validate(); err != nil {
		return err
	}
	if g.Capability == "" {
		return errors.New("capability name is required")
	}
	switch g.Effect {
	case EffectAllow, EffectDeny:
	default:
		return fmt.Errorf("unsupported capability effect %q", g.Effect)
	}
	switch g.ResourceKind {
	case ResourceOrg, ResourceRepo, ResourceWorkspace, ResourcePath, ResourceBranch:
	default:
		return fmt.Errorf("unsupported grant resource kind %q", g.ResourceKind)
	}
	if g.CreatedByUserID == "" {
		return errors.New("capability grant creator is required")
	}
	if g.CreatedAt.IsZero() {
		return errors.New("capability grant created_at is required")
	}
	if g.ExpiresAt != nil && !g.ExpiresAt.After(g.CreatedAt) {
		return errors.New("capability grant expires_at must be after created_at")
	}
	return nil
}

func (g Grant) IsActive(now time.Time) bool {
	if g.RevokedAt != nil {
		return false
	}
	if g.ExpiresAt != nil && !now.Before(*g.ExpiresAt) {
		return false
	}
	return true
}

func (g Grant) Matches(principal Principal, name Name, resource Resource, now time.Time) bool {
	if !g.IsActive(now) {
		return false
	}
	if g.OrgID != resource.OrgID {
		return false
	}
	if g.Principal != principal {
		return false
	}
	if g.Capability != name {
		return false
	}
	if g.ResourceKind != resource.Kind && g.ResourceKind != ResourceOrg {
		return false
	}
	if g.ResourceID != "" && g.ResourceID != resource.ID {
		return false
	}
	if g.RepoID != "" && g.RepoID != resource.RepoID {
		return false
	}
	if g.PathGlob != "" && !MatchGlob(g.PathGlob, resource.Path) {
		return false
	}
	if g.BranchPattern != "" && !MatchGlob(g.BranchPattern, resource.Branch) {
		return false
	}
	return true
}

func MatchGlob(pattern, value string) bool {
	if pattern == "" {
		return value == ""
	}
	if pattern == "*" || pattern == "**" {
		return true
	}

	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i++
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	b.WriteString("$")

	matched, err := regexp.MatchString(b.String(), value)
	return err == nil && matched
}

func cloneConditions(conditions Conditions) Conditions {
	return Conditions{
		RequiresApproval: conditions.RequiresApproval,
		Metadata:         cloneMetadata(conditions.Metadata),
	}
}

func cloneMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return map[string]string{}
	}
	clone := make(map[string]string, len(metadata))
	for k, v := range metadata {
		clone[k] = v
	}
	return clone
}
