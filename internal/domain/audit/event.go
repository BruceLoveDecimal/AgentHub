package audit

import (
	"errors"
	"time"
)

type EventType string

const (
	EventAgentCreated      EventType = "agent.created"
	EventCapabilityGranted EventType = "capability.granted"
	EventCapabilityChecked EventType = "capability.checked"
	EventTaskTokenIssued   EventType = "agent.task_token.issued"
)

type Decision string

const (
	DecisionRecorded Decision = "recorded"
	DecisionAllowed  Decision = "allowed"
	DecisionDenied   Decision = "denied"
	DecisionFailed   Decision = "failed"
)

type Event struct {
	ID           string
	OrgID        string
	WorkspaceID  string
	ActorChainID string
	Type         EventType
	SubjectKind  string
	SubjectID    string
	Action       string
	Decision     Decision
	Payload      map[string]string
	OccurredAt   time.Time
}

type EventSpec struct {
	ID           string
	OrgID        string
	WorkspaceID  string
	ActorChainID string
	Type         EventType
	SubjectKind  string
	SubjectID    string
	Action       string
	Decision     Decision
	Payload      map[string]string
	OccurredAt   time.Time
}

func NewEvent(spec EventSpec) (*Event, error) {
	event := &Event{
		ID:           spec.ID,
		OrgID:        spec.OrgID,
		WorkspaceID:  spec.WorkspaceID,
		ActorChainID: spec.ActorChainID,
		Type:         spec.Type,
		SubjectKind:  spec.SubjectKind,
		SubjectID:    spec.SubjectID,
		Action:       spec.Action,
		Decision:     spec.Decision,
		Payload:      clonePayload(spec.Payload),
		OccurredAt:   spec.OccurredAt,
	}
	return event, event.Validate()
}

func (e Event) Validate() error {
	if e.ID == "" {
		return errors.New("audit event id is required")
	}
	if e.OrgID == "" {
		return errors.New("audit event org id is required")
	}
	if e.Type == "" {
		return errors.New("audit event type is required")
	}
	if e.SubjectKind == "" {
		return errors.New("audit event subject kind is required")
	}
	if e.SubjectID == "" {
		return errors.New("audit event subject id is required")
	}
	if e.Action == "" {
		return errors.New("audit event action is required")
	}
	if e.Decision == "" {
		return errors.New("audit event decision is required")
	}
	if e.OccurredAt.IsZero() {
		return errors.New("audit event occurred_at is required")
	}
	return nil
}

func clonePayload(payload map[string]string) map[string]string {
	if len(payload) == 0 {
		return map[string]string{}
	}
	clone := make(map[string]string, len(payload))
	for k, v := range payload {
		clone[k] = v
	}
	return clone
}
