package provenance

import (
	"errors"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/change"
)

type Service struct{}

type BuildCommand struct {
	CommitID         string
	WorkspaceID      string
	AgentRunID       string
	ActorChain       audit.ActorChain
	PromptHash       string
	ContextHash      string
	DiffHash         string
	ToolLogRefs      []string
	TestEvidenceRefs []string
	CreatedAt        time.Time
	Payload          map[string]string
}

func (Service) Build(cmd BuildCommand) (*change.Provenance, error) {
	if err := cmd.ActorChain.Validate(); err != nil {
		return nil, err
	}
	if cmd.ActorChain.AgentID() == "" {
		return nil, errors.New("commit provenance requires an agent actor")
	}
	return change.NewProvenance(change.Provenance{
		CommitID:         cmd.CommitID,
		WorkspaceID:      cmd.WorkspaceID,
		AgentRunID:       cmd.AgentRunID,
		ActorChainID:     cmd.ActorChain.ID,
		PromptHash:       cmd.PromptHash,
		ContextHash:      cmd.ContextHash,
		DiffHash:         cmd.DiffHash,
		ToolLogRefs:      cmd.ToolLogRefs,
		TestEvidenceRefs: cmd.TestEvidenceRefs,
		CreatedAt:        cmd.CreatedAt,
		Payload:          cmd.Payload,
	})
}
