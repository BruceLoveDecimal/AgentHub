package ast

import (
	"context"
	"testing"
)

func TestGoIndexFindsSymbolsAndReferences(t *testing.T) {
	repo := memoryRepo{files: map[string]string{
		"auth.go": "package auth\n\ntype Service struct{}\nfunc Authorize() {}\nfunc Call() { Authorize() }\n",
	}}
	index := GoIndex{Repository: repo}

	symbols, err := index.FindSymbols(context.Background(), "repo-1", "abc123", "Authorize")
	if err != nil {
		t.Fatalf("find symbols: %v", err)
	}
	if len(symbols) != 1 || symbols[0].SymbolName != "Authorize" {
		t.Fatalf("unexpected symbols: %+v", symbols)
	}

	refs, err := index.FindReferences(context.Background(), "repo-1", "abc123", "Authorize")
	if err != nil {
		t.Fatalf("find refs: %v", err)
	}
	if len(refs) < 2 {
		t.Fatalf("expected declaration and call references, got %+v", refs)
	}
}

type memoryRepo struct {
	files map[string]string
}

func (r memoryRepo) ListFiles(context.Context, string, string) ([]string, error) {
	files := make([]string, 0, len(r.files))
	for path := range r.files {
		files = append(files, path)
	}
	return files, nil
}

func (r memoryRepo) ReadFile(_ context.Context, _, _, path string) (string, error) {
	return r.files[path], nil
}
