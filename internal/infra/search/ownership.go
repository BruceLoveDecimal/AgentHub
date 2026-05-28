package search

import (
	"context"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/capability"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/codeintel"
)

type OwnershipRule struct {
	RepoID           string
	PathGlob         string
	Owners           []string
	Protected        bool
	RequiresApproval bool
}

type OwnershipIndex struct {
	Rules []OwnershipRule
}

func (i OwnershipIndex) ResolveOwnership(_ context.Context, repoID, path string) (codeintel.Result, error) {
	result := codeintel.Result{
		RefKind:  codeintel.RefOwnership,
		RepoID:   repoID,
		FilePath: path,
		Source:   "ownership_rules",
	}
	for _, rule := range i.Rules {
		if rule.RepoID != "" && rule.RepoID != repoID {
			continue
		}
		if !capability.MatchGlob(rule.PathGlob, path) {
			continue
		}
		result.Owners = append(result.Owners, rule.Owners...)
		result.Protected = result.Protected || rule.Protected
		result.RequiresApproval = result.RequiresApproval || rule.RequiresApproval
	}
	result.ContentHash = codeintel.HashContent(repoID + ":" + path)
	return result, nil
}
