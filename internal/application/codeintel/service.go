package codeintel

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/capability"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/codeintel"
	authz "github.com/BruceLoveDecimal/AgentHub/internal/service/authorization"
)

type Service struct {
	Repository        RepositoryReader
	Symbols           SymbolIndex
	Ownership         OwnershipIndex
	WorkspaceScope    WorkspaceScope
	CapabilityGrants  CapabilityGrantStore
	ContextReferences ContextReferenceStore
	Audit             AuditRecorder
	IDs               IDGenerator
	Clock             Clock
}

type Command struct {
	Request    codeintel.Request
	Principal  capability.Principal
	ActorChain audit.ActorChain
}

func (s Service) GrepCode(ctx context.Context, cmd Command) (codeintel.Response, error) {
	cmd.Request.Primitive = codeintel.PrimitiveGrep
	if err := s.authorize(ctx, cmd); err != nil {
		return codeintel.Response{}, err
	}
	files, err := s.Repository.ListFiles(ctx, cmd.Request.RepoID, cmd.Request.Revision)
	if err != nil {
		return codeintel.Response{}, err
	}
	var results []codeintel.Result
	limit := normalizeLimit(cmd.Request.MaxResults)
	for _, file := range files {
		if len(results) >= limit {
			break
		}
		if cmd.Request.PathGlob != "" && !capability.MatchGlob(cmd.Request.PathGlob, file) {
			continue
		}
		content, err := s.Repository.ReadFile(ctx, cmd.Request.RepoID, cmd.Request.Revision, file)
		if err != nil {
			return codeintel.Response{}, err
		}
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if len(results) >= limit {
				break
			}
			if !strings.Contains(line, cmd.Request.Query) {
				continue
			}
			results = append(results, codeintel.Result{
				RefKind:     codeintel.RefFileRange,
				RepoID:      cmd.Request.RepoID,
				Revision:    cmd.Request.Revision,
				FilePath:    file,
				LineStart:   i + 1,
				LineEnd:     i + 1,
				Language:    codeintel.LanguageForPath(file),
				Content:     line,
				ContentHash: codeintel.HashContent(line),
				Source:      string(codeintel.PrimitiveGrep),
			})
		}
	}
	return s.finish(ctx, cmd, results)
}

func (s Service) ReadFile(ctx context.Context, cmd Command) (codeintel.Response, error) {
	cmd.Request.Primitive = codeintel.PrimitiveReadFile
	if err := s.authorize(ctx, cmd); err != nil {
		return codeintel.Response{}, err
	}
	content, err := s.Repository.ReadFile(ctx, cmd.Request.RepoID, cmd.Request.Revision, cmd.Request.Path)
	if err != nil {
		return codeintel.Response{}, err
	}
	start := cmd.Request.LineStart
	end := cmd.Request.LineEnd
	if start > 0 || end > 0 {
		content = sliceLines(content, start, end)
	}
	result := codeintel.Result{
		RefKind:     codeintel.RefFileRange,
		RepoID:      cmd.Request.RepoID,
		Revision:    cmd.Request.Revision,
		FilePath:    cmd.Request.Path,
		LineStart:   max(1, start),
		LineEnd:     end,
		Language:    codeintel.LanguageForPath(cmd.Request.Path),
		Content:     content,
		ContentHash: codeintel.HashContent(content),
		Source:      string(codeintel.PrimitiveReadFile),
	}
	if result.LineEnd == 0 {
		result.LineEnd = countLines(content)
	}
	return s.finish(ctx, cmd, []codeintel.Result{result})
}

func (s Service) FindSymbols(ctx context.Context, cmd Command) (codeintel.Response, error) {
	cmd.Request.Primitive = codeintel.PrimitiveSymbols
	if err := s.authorize(ctx, cmd); err != nil {
		return codeintel.Response{}, err
	}
	results, err := s.Symbols.FindSymbols(ctx, cmd.Request.RepoID, cmd.Request.Revision, cmd.Request.Query)
	if err != nil {
		return codeintel.Response{}, err
	}
	return s.finish(ctx, cmd, results)
}

func (s Service) FindReferences(ctx context.Context, cmd Command) (codeintel.Response, error) {
	cmd.Request.Primitive = codeintel.PrimitiveReferences
	if err := s.authorize(ctx, cmd); err != nil {
		return codeintel.Response{}, err
	}
	results, err := s.Symbols.FindReferences(ctx, cmd.Request.RepoID, cmd.Request.Revision, cmd.Request.Symbol)
	if err != nil {
		return codeintel.Response{}, err
	}
	return s.finish(ctx, cmd, results)
}

func (s Service) ResolveOwnership(ctx context.Context, cmd Command) (codeintel.Response, error) {
	cmd.Request.Primitive = codeintel.PrimitiveOwnership
	if err := s.authorize(ctx, cmd); err != nil {
		return codeintel.Response{}, err
	}
	result, err := s.Ownership.ResolveOwnership(ctx, cmd.Request.RepoID, cmd.Request.Path)
	if err != nil {
		return codeintel.Response{}, err
	}
	result.RepoID = cmd.Request.RepoID
	result.Revision = cmd.Request.Revision
	return s.finish(ctx, cmd, []codeintel.Result{result})
}

func (s Service) authorize(ctx context.Context, cmd Command) error {
	if err := s.requirePorts(cmd.Request.Primitive); err != nil {
		return err
	}
	if err := cmd.Request.Validate(); err != nil {
		return err
	}
	if err := cmd.ActorChain.Validate(); err != nil {
		return err
	}
	bound, err := s.WorkspaceScope.IsRepoBound(ctx, cmd.Request.WorkspaceID, cmd.Request.RepoID)
	if err != nil {
		return err
	}
	if !bound {
		return fmt.Errorf("repo %s is not bound to workspace %s", cmd.Request.RepoID, cmd.Request.WorkspaceID)
	}
	grants, err := s.CapabilityGrants.ListForPrincipal(ctx, cmd.Principal)
	if err != nil {
		return err
	}
	decision := authz.Service{}.Authorize(authz.Request{
		ActorChain: cmd.ActorChain,
		Principal:  cmd.Principal,
		Capability: cmd.Request.CapabilityName(),
		Resource:   cmd.Request.Resource(),
		At:         s.Clock.Now().UTC(),
	}, grants)
	auditDecision := audit.DecisionDenied
	if decision.Allowed {
		auditDecision = audit.DecisionAllowed
	}
	if err := s.recordAudit(ctx, cmd, auditDecision, map[string]string{
		"phase":            "authorize",
		"reason":           decision.Reason,
		"matched_grant_id": decision.MatchedGrantID,
	}); err != nil {
		return err
	}
	if !decision.Allowed {
		return fmt.Errorf("code intelligence request denied: %s", decision.Reason)
	}
	return nil
}

func (s Service) finish(ctx context.Context, cmd Command, results []codeintel.Result) (codeintel.Response, error) {
	for i := range results {
		results[i] = results[i].WithContentHash()
		if !cmd.Request.UsedByAgent {
			continue
		}
		ref := &codeintel.ContextReference{
			ID:          s.IDs.NewID(),
			WorkspaceID: cmd.Request.WorkspaceID,
			RefKind:     results[i].RefKind,
			RefID:       refID(results[i]),
			RepoID:      cmd.Request.RepoID,
			FilePath:    results[i].FilePath,
			LineStart:   results[i].LineStart,
			LineEnd:     results[i].LineEnd,
			ContentHash: results[i].ContentHash,
			Rank:        i + 1,
			Metadata: map[string]string{
				"primitive": string(cmd.Request.Primitive),
				"revision":  cmd.Request.Revision,
			},
		}
		if err := s.ContextReferences.Save(ctx, ref); err != nil {
			return codeintel.Response{}, err
		}
		results[i].ContextReferenceID = ref.ID
	}
	if err := s.recordAudit(ctx, cmd, audit.DecisionRecorded, map[string]string{
		"phase":        "result",
		"result_count": fmt.Sprintf("%d", len(results)),
	}); err != nil {
		return codeintel.Response{}, err
	}
	return codeintel.Response{
		WorkspaceID: cmd.Request.WorkspaceID,
		RepoID:      cmd.Request.RepoID,
		Revision:    cmd.Request.Revision,
		Primitive:   cmd.Request.Primitive,
		Results:     results,
	}, nil
}

func (s Service) requirePorts(primitive codeintel.Primitive) error {
	if s.WorkspaceScope == nil {
		return errors.New("workspace scope is required")
	}
	if s.CapabilityGrants == nil {
		return errors.New("capability grants store is required")
	}
	if s.ContextReferences == nil {
		return errors.New("context reference store is required")
	}
	if s.Audit == nil {
		return errors.New("audit recorder is required")
	}
	if s.IDs == nil {
		return errors.New("id generator is required")
	}
	if s.Clock == nil {
		return errors.New("clock is required")
	}
	switch primitive {
	case codeintel.PrimitiveGrep, codeintel.PrimitiveReadFile:
		if s.Repository == nil {
			return errors.New("repository reader is required")
		}
	case codeintel.PrimitiveSymbols, codeintel.PrimitiveReferences:
		if s.Symbols == nil {
			return errors.New("symbol index is required")
		}
	case codeintel.PrimitiveOwnership:
		if s.Ownership == nil {
			return errors.New("ownership index is required")
		}
	}
	return nil
}

func (s Service) recordAudit(ctx context.Context, cmd Command, decision audit.Decision, payload map[string]string) error {
	event, err := audit.NewEvent(audit.EventSpec{
		ID:           s.IDs.NewID(),
		OrgID:        cmd.Request.OrgID,
		WorkspaceID:  cmd.Request.WorkspaceID,
		ActorChainID: cmd.ActorChain.ID,
		Type:         audit.EventCodeIntelQueried,
		SubjectKind:  "code_intelligence",
		SubjectID:    string(cmd.Request.Primitive),
		Action:       string(cmd.Request.Primitive),
		Decision:     decision,
		Payload:      payload,
		OccurredAt:   s.Clock.Now().UTC(),
	})
	if err != nil {
		return err
	}
	return s.Audit.Record(ctx, event)
}

func normalizeLimit(limit int) int {
	if limit <= 0 || limit > 100 {
		return 100
	}
	return limit
}

func sliceLines(content string, start, end int) string {
	lines := strings.Split(content, "\n")
	if start <= 0 {
		start = 1
	}
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	if start > len(lines) {
		return ""
	}
	return strings.Join(lines[start-1:end], "\n")
}

func countLines(content string) int {
	if content == "" {
		return 0
	}
	return len(strings.Split(content, "\n"))
}

func refID(result codeintel.Result) string {
	if result.QualifiedName != "" {
		return result.QualifiedName
	}
	if result.FilePath != "" {
		return fmt.Sprintf("%s:%d-%d", result.FilePath, result.LineStart, result.LineEnd)
	}
	return result.Source
}
