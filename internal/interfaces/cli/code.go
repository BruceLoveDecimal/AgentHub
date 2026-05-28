package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	app "github.com/BruceLoveDecimal/AgentHub/internal/application/codeintel"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/capability"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/codeintel"
)

type CodeCommand struct {
	Service app.Service
	Out     io.Writer
}

func (c CodeCommand) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("code subcommand is required")
	}
	subcommand := args[0]
	flags := flag.NewFlagSet("agenthub code "+subcommand, flag.ContinueOnError)
	workspaceID := flags.String("workspace", "", "workspace id")
	orgID := flags.String("org", "", "org id")
	repoID := flags.String("repo", "", "repo id")
	rev := flags.String("rev", "", "revision")
	query := flags.String("query", "", "query")
	path := flags.String("path", "", "path")
	symbol := flags.String("symbol", "", "symbol")
	agentID := flags.String("agent", "", "agent principal id")
	userID := flags.String("user", "", "root user id")
	used := flags.Bool("used-by-agent", true, "record context references")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	chain, err := audit.NewAgentActorChain("cli-chain", *orgID, *workspaceID, *userID, *agentID, c.Service.Clock.Now())
	if err != nil {
		return err
	}
	cmd := app.Command{
		Request: codeintel.Request{
			WorkspaceID: *workspaceID,
			OrgID:       *orgID,
			RepoID:      *repoID,
			Revision:    *rev,
			Query:       *query,
			Path:        *path,
			Symbol:      *symbol,
			UsedByAgent: *used,
		},
		Principal:  capability.Principal{Kind: capability.PrincipalAgent, ID: *agentID},
		ActorChain: chain,
	}
	var response codeintel.Response
	switch subcommand {
	case "grep":
		response, err = c.Service.GrepCode(ctx, cmd)
	case "read-file":
		response, err = c.Service.ReadFile(ctx, cmd)
	case "symbols":
		response, err = c.Service.FindSymbols(ctx, cmd)
	case "references":
		response, err = c.Service.FindReferences(ctx, cmd)
	case "ownership":
		response, err = c.Service.ResolveOwnership(ctx, cmd)
	default:
		return fmt.Errorf("unsupported code subcommand %q", subcommand)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(c.Out).Encode(response)
}
