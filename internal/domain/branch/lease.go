package branch

import (
	"errors"
	"fmt"
	"time"
)

type LeaseKind string

const (
	LeaseBranch     LeaseKind = "branch"
	LeaseFileArea   LeaseKind = "file_area"
	LeaseSymbolArea LeaseKind = "symbol_area"
)

type LeaseStatus string

const (
	LeaseActive   LeaseStatus = "active"
	LeaseReleased LeaseStatus = "released"
	LeaseExpired  LeaseStatus = "expired"
	LeaseStolen   LeaseStatus = "stolen"
)

type Lease struct {
	ID            string
	OrgID         string
	WorkspaceID   string
	RepoID        string
	BranchName    string
	Kind          LeaseKind
	HolderAgentID string
	FencingToken  int64
	Status        LeaseStatus
	AcquiredAt    time.Time
	ExpiresAt     time.Time
	ReleasedAt    *time.Time
	Metadata      map[string]string
}

type LeaseSpec struct {
	ID            string
	OrgID         string
	WorkspaceID   string
	RepoID        string
	BranchName    string
	Kind          LeaseKind
	HolderAgentID string
	FencingToken  int64
	TTL           time.Duration
	Now           time.Time
	Metadata      map[string]string
}

func NewLease(spec LeaseSpec) (*Lease, error) {
	if spec.Kind == "" {
		spec.Kind = LeaseBranch
	}
	lease := &Lease{
		ID:            spec.ID,
		OrgID:         spec.OrgID,
		WorkspaceID:   spec.WorkspaceID,
		RepoID:        spec.RepoID,
		BranchName:    spec.BranchName,
		Kind:          spec.Kind,
		HolderAgentID: spec.HolderAgentID,
		FencingToken:  spec.FencingToken,
		Status:        LeaseActive,
		AcquiredAt:    spec.Now,
		ExpiresAt:     spec.Now.Add(spec.TTL),
		Metadata:      cloneMap(spec.Metadata),
	}
	return lease, lease.Validate()
}

func (l Lease) Validate() error {
	if l.ID == "" {
		return errors.New("branch lease id is required")
	}
	if l.OrgID == "" {
		return errors.New("branch lease org id is required")
	}
	if l.WorkspaceID == "" {
		return errors.New("branch lease workspace id is required")
	}
	if l.RepoID == "" {
		return errors.New("branch lease repo id is required")
	}
	if l.BranchName == "" {
		return errors.New("branch lease branch name is required")
	}
	switch l.Kind {
	case LeaseBranch, LeaseFileArea, LeaseSymbolArea:
	default:
		return fmt.Errorf("unsupported branch lease kind %q", l.Kind)
	}
	if l.HolderAgentID == "" {
		return errors.New("branch lease holder agent id is required")
	}
	if l.FencingToken <= 0 {
		return errors.New("branch lease fencing token must be positive")
	}
	switch l.Status {
	case LeaseActive, LeaseReleased, LeaseExpired, LeaseStolen:
	default:
		return fmt.Errorf("unsupported branch lease status %q", l.Status)
	}
	if l.AcquiredAt.IsZero() {
		return errors.New("branch lease acquired_at is required")
	}
	if !l.ExpiresAt.After(l.AcquiredAt) {
		return errors.New("branch lease expires_at must be after acquired_at")
	}
	return nil
}

func (l Lease) IsActive(now time.Time) bool {
	return l.Status == LeaseActive && now.Before(l.ExpiresAt)
}

func (l *Lease) Release(now time.Time, holderAgentID string) error {
	if l == nil {
		return errors.New("branch lease is required")
	}
	if l.Status != LeaseActive {
		return fmt.Errorf("cannot release lease in status %s", l.Status)
	}
	if l.HolderAgentID != holderAgentID {
		return errors.New("only the lease holder can release the lease")
	}
	l.Status = LeaseReleased
	l.ReleasedAt = &now
	return nil
}

func (l *Lease) Expire(now time.Time) error {
	if l == nil {
		return errors.New("branch lease is required")
	}
	if l.Status != LeaseActive {
		return nil
	}
	if now.Before(l.ExpiresAt) {
		return errors.New("branch lease has not expired")
	}
	l.Status = LeaseExpired
	return nil
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
