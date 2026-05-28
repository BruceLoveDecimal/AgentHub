package workflow

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/workspace"
)

type Service struct{}

func (Service) ValidateReadyToRun(w workspace.Workspace) error {
	if len(w.Repositories) == 0 {
		return fmt.Errorf("workspace %s has no bound repositories", w.ID)
	}
	if len(w.Agents) == 0 {
		return fmt.Errorf("workspace %s has no assigned agents", w.ID)
	}
	if w.PrimaryAgentID == "" {
		return fmt.Errorf("workspace %s has no primary agent", w.ID)
	}
	return nil
}

func (Service) DefaultTaskBranchName(w workspace.Workspace) string {
	key := strings.ToLower(w.Key)
	key = nonBranchChar.ReplaceAllString(key, "-")
	key = strings.Trim(key, "-")
	return "agent/" + key
}

var nonBranchChar = regexp.MustCompile(`[^a-z0-9._/-]+`)
