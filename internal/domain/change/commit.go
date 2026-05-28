package change

import (
	"errors"
	"time"
)

type Commit struct {
	ID            string
	OrgID         string
	RepoID        string
	WorkspaceID   string
	BundleID      string
	SHA           string
	Message       string
	ParentSHAs    []string
	AuthorAgentID string
	AuthorUserID  string
	CommittedAt   time.Time
	CreatedAt     time.Time
}

type Provenance struct {
	CommitID         string
	WorkspaceID      string
	AgentRunID       string
	ActorChainID     string
	PromptHash       string
	ContextHash      string
	DiffHash         string
	ToolLogRefs      []string
	TestEvidenceRefs []string
	CreatedAt        time.Time
	Payload          map[string]string
}

func NewCommit(commit Commit) (*Commit, error) {
	commit.ParentSHAs = cloneSlice(commit.ParentSHAs)
	return &commit, commit.Validate()
}

func (c Commit) Validate() error {
	if c.ID == "" || c.OrgID == "" || c.RepoID == "" || c.SHA == "" {
		return errors.New("commit id, org id, repo id, and sha are required")
	}
	if c.WorkspaceID == "" {
		return errors.New("commit workspace id is required")
	}
	if c.Message == "" {
		return errors.New("commit message is required")
	}
	if c.AuthorAgentID == "" && c.AuthorUserID == "" {
		return errors.New("commit author is required")
	}
	if c.CommittedAt.IsZero() || c.CreatedAt.IsZero() {
		return errors.New("commit timestamps are required")
	}
	return nil
}

func NewProvenance(provenance Provenance) (*Provenance, error) {
	provenance.ToolLogRefs = cloneSlice(provenance.ToolLogRefs)
	provenance.TestEvidenceRefs = cloneSlice(provenance.TestEvidenceRefs)
	provenance.Payload = cloneMap(provenance.Payload)
	return &provenance, provenance.Validate()
}

func (p Provenance) Validate() error {
	if p.CommitID == "" || p.WorkspaceID == "" || p.ActorChainID == "" {
		return errors.New("provenance commit id, workspace id, and actor chain id are required")
	}
	if p.PromptHash == "" || p.ContextHash == "" || p.DiffHash == "" {
		return errors.New("provenance prompt, context, and diff hashes are required")
	}
	if len(p.ToolLogRefs) == 0 {
		return errors.New("provenance requires tool log refs")
	}
	if len(p.TestEvidenceRefs) == 0 {
		return errors.New("provenance requires test evidence refs")
	}
	if p.CreatedAt.IsZero() {
		return errors.New("provenance created_at is required")
	}
	return nil
}

func cloneSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	clone := make([]string, len(values))
	copy(clone, values)
	return clone
}
