package agent

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Status string

const (
	StatusActive  Status = "active"
	StatusPaused  Status = "paused"
	StatusRevoked Status = "revoked"
)

const DefaultMaxTokenTTL = 15 * time.Minute

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)

type Agent struct {
	ID                  string
	OrgID               string
	Slug                string
	DisplayName         string
	Purpose             string
	OwnerUserID         string
	RecoveryOwnerUserID string
	Status              Status
	DefaultModel        string
	MaxTokenTTL         time.Duration
	CreatedByUserID     string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Metadata            map[string]string
}

type Profile struct {
	ID                  string
	OrgID               string
	Slug                string
	DisplayName         string
	Purpose             string
	OwnerUserID         string
	RecoveryOwnerUserID string
	Status              Status
	DefaultModel        string
	MaxTokenTTL         time.Duration
	CreatedByUserID     string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Metadata            map[string]string
}

func New(profile Profile) (*Agent, error) {
	status := profile.Status
	if status == "" {
		status = StatusActive
	}
	maxTokenTTL := profile.MaxTokenTTL
	if maxTokenTTL <= 0 {
		maxTokenTTL = DefaultMaxTokenTTL
	}
	agent := &Agent{
		ID:                  strings.TrimSpace(profile.ID),
		OrgID:               strings.TrimSpace(profile.OrgID),
		Slug:                strings.TrimSpace(profile.Slug),
		DisplayName:         strings.TrimSpace(profile.DisplayName),
		Purpose:             strings.TrimSpace(profile.Purpose),
		OwnerUserID:         strings.TrimSpace(profile.OwnerUserID),
		RecoveryOwnerUserID: strings.TrimSpace(profile.RecoveryOwnerUserID),
		Status:              status,
		DefaultModel:        strings.TrimSpace(profile.DefaultModel),
		MaxTokenTTL:         maxTokenTTL,
		CreatedByUserID:     strings.TrimSpace(profile.CreatedByUserID),
		CreatedAt:           profile.CreatedAt,
		UpdatedAt:           profile.UpdatedAt,
		Metadata:            cloneMetadata(profile.Metadata),
	}
	return agent, agent.Validate()
}

func (a Agent) Validate() error {
	if a.ID == "" {
		return errors.New("agent id is required")
	}
	if a.OrgID == "" {
		return errors.New("agent org id is required")
	}
	if !slugPattern.MatchString(a.Slug) {
		return fmt.Errorf("agent slug %q must be lowercase kebab-case and 3-64 characters", a.Slug)
	}
	if a.DisplayName == "" {
		return errors.New("agent display name is required")
	}
	if a.Purpose == "" {
		return errors.New("agent purpose is required")
	}
	if a.OwnerUserID == "" {
		return errors.New("agent owner user id is required")
	}
	if a.RecoveryOwnerUserID == "" {
		return errors.New("agent recovery owner user id is required")
	}
	switch a.Status {
	case StatusActive, StatusPaused, StatusRevoked:
	default:
		return fmt.Errorf("unsupported agent status %q", a.Status)
	}
	if a.MaxTokenTTL <= 0 {
		return errors.New("agent max token ttl must be positive")
	}
	if a.CreatedByUserID == "" {
		return errors.New("agent creator user id is required")
	}
	if a.CreatedAt.IsZero() {
		return errors.New("agent created_at is required")
	}
	if a.UpdatedAt.IsZero() {
		return errors.New("agent updated_at is required")
	}
	if a.UpdatedAt.Before(a.CreatedAt) {
		return errors.New("agent updated_at cannot be before created_at")
	}
	return nil
}

func (a Agent) CanIssueTaskToken() error {
	if a.Status != StatusActive {
		return fmt.Errorf("agent %s is not active", a.ID)
	}
	return nil
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
