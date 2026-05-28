package change

import (
	"errors"
	"fmt"
	"time"
)

type BundleStatus string

const (
	BundleDraft          BundleStatus = "draft"
	BundleReadyForReview BundleStatus = "ready_for_review"
	BundleApproved       BundleStatus = "approved"
	BundleMerged         BundleStatus = "merged"
	BundleReverted       BundleStatus = "reverted"
	BundleCancelled      BundleStatus = "cancelled"
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type ItemStatus string

const (
	ItemDraft    ItemStatus = "draft"
	ItemReady    ItemStatus = "ready"
	ItemBlocked  ItemStatus = "blocked"
	ItemMerged   ItemStatus = "merged"
	ItemReverted ItemStatus = "reverted"
)

type Bundle struct {
	ID               string
	OrgID            string
	WorkspaceID      string
	Title            string
	Status           BundleStatus
	RiskLevel        RiskLevel
	CreatedByAgentID string
	RollbackPlan     map[string]string
	Items            []BundleItem
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Metadata         map[string]string
}

type BundleItem struct {
	ID                string
	BundleID          string
	RepoID            string
	WorkspaceBranchID string
	PullRequestID     string
	MergeOrder        int
	Status            ItemStatus
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type BundleSpec struct {
	ID               string
	OrgID            string
	WorkspaceID      string
	Title            string
	RiskLevel        RiskLevel
	CreatedByAgentID string
	CreatedAt        time.Time
	Metadata         map[string]string
}

func NewBundle(spec BundleSpec) (*Bundle, error) {
	if spec.RiskLevel == "" {
		spec.RiskLevel = RiskLow
	}
	bundle := &Bundle{
		ID:               spec.ID,
		OrgID:            spec.OrgID,
		WorkspaceID:      spec.WorkspaceID,
		Title:            spec.Title,
		Status:           BundleDraft,
		RiskLevel:        spec.RiskLevel,
		CreatedByAgentID: spec.CreatedByAgentID,
		CreatedAt:        spec.CreatedAt,
		UpdatedAt:        spec.CreatedAt,
		Metadata:         cloneMap(spec.Metadata),
	}
	return bundle, bundle.Validate()
}

func (b Bundle) Validate() error {
	if b.ID == "" {
		return errors.New("change bundle id is required")
	}
	if b.OrgID == "" {
		return errors.New("change bundle org id is required")
	}
	if b.WorkspaceID == "" {
		return errors.New("change bundle workspace id is required")
	}
	if b.Title == "" {
		return errors.New("change bundle title is required")
	}
	switch b.Status {
	case BundleDraft, BundleReadyForReview, BundleApproved, BundleMerged, BundleReverted, BundleCancelled:
	default:
		return fmt.Errorf("unsupported bundle status %q", b.Status)
	}
	switch b.RiskLevel {
	case RiskLow, RiskMedium, RiskHigh, RiskCritical:
	default:
		return fmt.Errorf("unsupported risk level %q", b.RiskLevel)
	}
	if b.CreatedByAgentID == "" {
		return errors.New("change bundle created_by_agent_id is required")
	}
	if b.CreatedAt.IsZero() || b.UpdatedAt.IsZero() {
		return errors.New("change bundle timestamps are required")
	}
	for _, item := range b.Items {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (b *Bundle) AddItem(item BundleItem, now time.Time) error {
	if b == nil {
		return errors.New("change bundle is required")
	}
	if b.Status != BundleDraft {
		return fmt.Errorf("cannot add item to bundle in status %s", b.Status)
	}
	item.BundleID = b.ID
	item.Status = ItemDraft
	item.CreatedAt = now
	item.UpdatedAt = now
	if item.MergeOrder == 0 {
		item.MergeOrder = len(b.Items) + 1
	}
	if err := item.Validate(); err != nil {
		return err
	}
	for _, existing := range b.Items {
		if existing.RepoID == item.RepoID && existing.WorkspaceBranchID == item.WorkspaceBranchID {
			return errors.New("change bundle already contains this repo branch")
		}
	}
	b.Items = append(b.Items, item)
	b.UpdatedAt = now
	return nil
}

func (i BundleItem) Validate() error {
	if i.ID == "" {
		return errors.New("change bundle item id is required")
	}
	if i.BundleID == "" {
		return errors.New("change bundle item bundle id is required")
	}
	if i.RepoID == "" {
		return errors.New("change bundle item repo id is required")
	}
	if i.WorkspaceBranchID == "" {
		return errors.New("change bundle item workspace branch id is required")
	}
	if i.MergeOrder <= 0 {
		return errors.New("change bundle item merge order must be positive")
	}
	switch i.Status {
	case ItemDraft, ItemReady, ItemBlocked, ItemMerged, ItemReverted:
	default:
		return fmt.Errorf("unsupported item status %q", i.Status)
	}
	if i.CreatedAt.IsZero() || i.UpdatedAt.IsZero() {
		return errors.New("change bundle item timestamps are required")
	}
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
