package audit

import (
	"errors"
	"fmt"
	"time"
)

type ActorKind string

const (
	ActorKindHuman ActorKind = "human"
	ActorKindAgent ActorKind = "agent"
	ActorKindTool  ActorKind = "tool"
)

type ActorStep struct {
	Kind         ActorKind
	ID           string
	DisplayName  string
	InvocationID string
}

type ActorChain struct {
	ID          string
	OrgID       string
	WorkspaceID string
	Steps       []ActorStep
	CreatedAt   time.Time
}

func NewHumanActorChain(id, orgID, workspaceID, userID string, createdAt time.Time) (ActorChain, error) {
	chain := ActorChain{
		ID:          id,
		OrgID:       orgID,
		WorkspaceID: workspaceID,
		Steps: []ActorStep{
			{Kind: ActorKindHuman, ID: userID},
		},
		CreatedAt: createdAt,
	}
	return chain, chain.Validate()
}

func NewAgentActorChain(id, orgID, workspaceID, userID, agentID string, createdAt time.Time) (ActorChain, error) {
	chain := ActorChain{
		ID:          id,
		OrgID:       orgID,
		WorkspaceID: workspaceID,
		Steps: []ActorStep{
			{Kind: ActorKindHuman, ID: userID},
			{Kind: ActorKindAgent, ID: agentID},
		},
		CreatedAt: createdAt,
	}
	return chain, chain.Validate()
}

func (c ActorChain) WithTool(invocationID, toolName string) (ActorChain, error) {
	c.Steps = append(c.Steps, ActorStep{
		Kind:         ActorKindTool,
		ID:           toolName,
		DisplayName:  toolName,
		InvocationID: invocationID,
	})
	return c, c.Validate()
}

func (c ActorChain) Validate() error {
	if c.ID == "" {
		return errors.New("actor chain id is required")
	}
	if c.OrgID == "" {
		return errors.New("actor chain org id is required")
	}
	if c.CreatedAt.IsZero() {
		return errors.New("actor chain created_at is required")
	}
	if len(c.Steps) == 0 {
		return errors.New("actor chain must include at least one actor")
	}
	if c.Steps[0].Kind != ActorKindHuman {
		return errors.New("actor chain must start with a human")
	}

	seenAgent := false
	for i, step := range c.Steps {
		if step.ID == "" {
			return fmt.Errorf("actor chain step %d id is required", i)
		}
		switch step.Kind {
		case ActorKindHuman:
			if i != 0 {
				return errors.New("human actor must be the first actor in the chain")
			}
		case ActorKindAgent:
			if i != 1 {
				return errors.New("agent actor must immediately follow the human actor")
			}
			seenAgent = true
		case ActorKindTool:
			if !seenAgent {
				return errors.New("tool invocation must follow an agent actor")
			}
			if step.InvocationID == "" {
				return errors.New("tool invocation id is required")
			}
		default:
			return fmt.Errorf("unsupported actor kind %q", step.Kind)
		}
	}

	return nil
}

func (c ActorChain) RootUserID() string {
	if len(c.Steps) == 0 || c.Steps[0].Kind != ActorKindHuman {
		return ""
	}
	return c.Steps[0].ID
}

func (c ActorChain) AgentID() string {
	if len(c.Steps) < 2 || c.Steps[1].Kind != ActorKindAgent {
		return ""
	}
	return c.Steps[1].ID
}
