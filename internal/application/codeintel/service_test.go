package codeintel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/capability"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/codeintel"
)

func TestGrepCodeAuthorizesRecordsAuditAndContext(t *testing.T) {
	ctx := context.Background()
	svc, stores := newTestService()

	response, err := svc.GrepCode(ctx, baseCommand(codeintel.Request{
		Query:       "func Authorize",
		UsedByAgent: true,
	}))
	if err != nil {
		t.Fatalf("grep code: %v", err)
	}
	if len(response.Results) != 1 {
		t.Fatalf("expected one result, got %d", len(response.Results))
	}
	if response.Results[0].ContextReferenceID == "" {
		t.Fatal("expected context reference id on result")
	}
	if len(stores.context.refs) != 1 {
		t.Fatalf("expected context reference to be stored, got %d", len(stores.context.refs))
	}
	if len(stores.audit.events) != 2 {
		t.Fatalf("expected authorize and result audit events, got %d", len(stores.audit.events))
	}
	if stores.audit.events[0].Type != audit.EventCodeIntelQueried {
		t.Fatalf("unexpected audit event type: %s", stores.audit.events[0].Type)
	}
}

func TestReadFileRequiresBoundWorkspaceRepo(t *testing.T) {
	ctx := context.Background()
	svc, stores := newTestService()
	stores.scope.bound = false

	_, err := svc.ReadFile(ctx, baseCommand(codeintel.Request{
		Path: "internal/service/auth.go",
	}))
	if err == nil {
		t.Fatal("expected unbound repo error")
	}
}

func TestReadFileSlicesLineRange(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService()

	response, err := svc.ReadFile(ctx, baseCommand(codeintel.Request{
		Path:      "internal/service/auth.go",
		LineStart: 2,
		LineEnd:   2,
	}))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if got := response.Results[0].Content; got != "func Authorize() {}" {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestDeniedGrantBlocksCodeIntel(t *testing.T) {
	ctx := context.Background()
	svc, stores := newTestService()
	stores.grants.grants = []capability.Grant{
		mustGrant(t, capability.GrantSpec{
			ID:              "deny-1",
			OrgID:           "org-1",
			Principal:       capability.Principal{Kind: capability.PrincipalAgent, ID: "agent-1"},
			Capability:      capability.CodeGrep,
			Effect:          capability.EffectDeny,
			ResourceKind:    capability.ResourceRepo,
			ResourceID:      "repo-1",
			RepoID:          "repo-1",
			CreatedByUserID: "user-1",
			CreatedAt:       stores.clock.now,
		}),
	}

	_, err := svc.GrepCode(ctx, baseCommand(codeintel.Request{Query: "Authorize"}))
	if err == nil {
		t.Fatal("expected denied request")
	}
	if len(stores.audit.events) != 1 {
		t.Fatalf("expected denied audit event, got %d", len(stores.audit.events))
	}
	if stores.audit.events[0].Decision != audit.DecisionDenied {
		t.Fatalf("expected denied audit decision, got %s", stores.audit.events[0].Decision)
	}
}

func TestFindSymbolsReferencesAndOwnership(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService()

	symbols, err := svc.FindSymbols(ctx, baseCommand(codeintel.Request{Query: "Authorize"}))
	if err != nil {
		t.Fatalf("find symbols: %v", err)
	}
	if len(symbols.Results) != 1 || symbols.Results[0].SymbolName != "Authorize" {
		t.Fatalf("unexpected symbol results: %+v", symbols.Results)
	}

	refs, err := svc.FindReferences(ctx, baseCommand(codeintel.Request{Symbol: "Authorize"}))
	if err != nil {
		t.Fatalf("find references: %v", err)
	}
	if len(refs.Results) == 0 {
		t.Fatal("expected references")
	}

	ownership, err := svc.ResolveOwnership(ctx, baseCommand(codeintel.Request{Path: "internal/service/auth.go"}))
	if err != nil {
		t.Fatalf("resolve ownership: %v", err)
	}
	if !ownership.Results[0].Protected || !ownership.Results[0].RequiresApproval {
		t.Fatalf("expected protected ownership result: %+v", ownership.Results[0])
	}
}

type testStores struct {
	repo    *memoryRepo
	symbols *memorySymbols
	owners  *memoryOwners
	scope   *memoryScope
	grants  *memoryGrants
	context *memoryContext
	audit   *memoryAudit
	clock   *fixedClock
}

func newTestService() (Service, testStores) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	stores := testStores{
		repo: &memoryRepo{files: map[string]string{
			"internal/service/auth.go": "package service\nfunc Authorize() {}\nfunc Call() { Authorize() }",
		}},
		symbols: &memorySymbols{},
		owners:  &memoryOwners{},
		scope:   &memoryScope{bound: true},
		grants:  &memoryGrants{},
		context: &memoryContext{},
		audit:   &memoryAudit{},
		clock:   &fixedClock{now: now},
	}
	stores.symbols.repo = stores.repo
	stores.owners.result = codeintel.Result{
		RefKind:          codeintel.RefOwnership,
		FilePath:         "internal/service/auth.go",
		Owners:           []string{"team/security"},
		Protected:        true,
		RequiresApproval: true,
		Source:           "test",
	}
	for _, capName := range []capability.Name{
		capability.CodeGrep,
		capability.CodeReadFile,
		capability.CodeSymbols,
		capability.CodeReferences,
		capability.CodeOwnership,
	} {
		stores.grants.grants = append(stores.grants.grants, mustGrant(nil, capability.GrantSpec{
			ID:              string(capName),
			OrgID:           "org-1",
			Principal:       capability.Principal{Kind: capability.PrincipalAgent, ID: "agent-1"},
			Capability:      capName,
			Effect:          capability.EffectAllow,
			ResourceKind:    capability.ResourceRepo,
			ResourceID:      "repo-1",
			RepoID:          "repo-1",
			CreatedByUserID: "user-1",
			CreatedAt:       now,
		}))
	}
	return Service{
		Repository:        stores.repo,
		Symbols:           stores.symbols,
		Ownership:         stores.owners,
		WorkspaceScope:    stores.scope,
		CapabilityGrants:  stores.grants,
		ContextReferences: stores.context,
		Audit:             stores.audit,
		IDs:               &sequenceIDs{},
		Clock:             stores.clock,
	}, stores
}

func baseCommand(req codeintel.Request) Command {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	chain, err := audit.NewAgentActorChain("chain-1", "org-1", "workspace-1", "user-1", "agent-1", now)
	if err != nil {
		panic(err)
	}
	req.WorkspaceID = "workspace-1"
	req.OrgID = "org-1"
	req.RepoID = "repo-1"
	req.Revision = "abc123"
	return Command{
		Request:    req,
		Principal:  capability.Principal{Kind: capability.PrincipalAgent, ID: "agent-1"},
		ActorChain: chain,
	}
}

type memoryRepo struct {
	files map[string]string
}

func (r *memoryRepo) ListFiles(context.Context, string, string) ([]string, error) {
	files := make([]string, 0, len(r.files))
	for path := range r.files {
		files = append(files, path)
	}
	return files, nil
}

func (r *memoryRepo) ReadFile(_ context.Context, _, _, path string) (string, error) {
	content, ok := r.files[path]
	if !ok {
		return "", errors.New("file not found")
	}
	return content, nil
}

type memorySymbols struct {
	repo *memoryRepo
}

func (s *memorySymbols) FindSymbols(context.Context, string, string, string) ([]codeintel.Result, error) {
	return []codeintel.Result{{
		RefKind:       codeintel.RefSymbol,
		FilePath:      "internal/service/auth.go",
		LineStart:     2,
		LineEnd:       2,
		Language:      "go",
		SymbolName:    "Authorize",
		QualifiedName: "Authorize",
		SymbolKind:    "function",
		ContentHash:   codeintel.HashContent("Authorize"),
		Source:        "test",
	}}, nil
}

func (s *memorySymbols) FindReferences(context.Context, string, string, string) ([]codeintel.Result, error) {
	return []codeintel.Result{{
		RefKind:       codeintel.RefSymbol,
		FilePath:      "internal/service/auth.go",
		LineStart:     3,
		LineEnd:       3,
		Language:      "go",
		SymbolName:    "Authorize",
		QualifiedName: "Authorize",
		SymbolKind:    "reference",
		ContentHash:   codeintel.HashContent("Authorize-ref"),
		Source:        "test",
	}}, nil
}

type memoryOwners struct {
	result codeintel.Result
}

func (o *memoryOwners) ResolveOwnership(context.Context, string, string) (codeintel.Result, error) {
	return o.result, nil
}

type memoryScope struct {
	bound bool
}

func (s *memoryScope) IsRepoBound(context.Context, string, string) (bool, error) {
	return s.bound, nil
}

type memoryGrants struct {
	grants []capability.Grant
}

func (s *memoryGrants) ListForPrincipal(_ context.Context, principal capability.Principal) ([]capability.Grant, error) {
	var grants []capability.Grant
	for _, grant := range s.grants {
		if grant.Principal == principal {
			grants = append(grants, grant)
		}
	}
	return grants, nil
}

type memoryContext struct {
	refs []*codeintel.ContextReference
}

func (s *memoryContext) Save(_ context.Context, ref *codeintel.ContextReference) error {
	s.refs = append(s.refs, ref)
	return nil
}

type memoryAudit struct {
	events []*audit.Event
}

func (r *memoryAudit) Record(_ context.Context, event *audit.Event) error {
	r.events = append(r.events, event)
	return nil
}

type sequenceIDs struct {
	next int
}

func (g *sequenceIDs) NewID() string {
	g.next++
	return "id-" + string(rune('0'+g.next))
}

type fixedClock struct {
	now time.Time
}

func (c *fixedClock) Now() time.Time {
	return c.now
}

func mustGrant(t *testing.T, spec capability.GrantSpec) capability.Grant {
	if t != nil {
		t.Helper()
	}
	grant, err := capability.NewGrant(spec)
	if err != nil {
		if t == nil {
			panic(err)
		}
		t.Fatalf("new grant: %v", err)
	}
	return *grant
}
