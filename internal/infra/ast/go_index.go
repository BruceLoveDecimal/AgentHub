package ast

import (
	"context"
	goast "go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/codeintel"
)

type RepositoryReader interface {
	ListFiles(ctx context.Context, repoID, revision string) ([]string, error)
	ReadFile(ctx context.Context, repoID, revision, path string) (string, error)
}

type GoIndex struct {
	Repository RepositoryReader
}

func (i GoIndex) FindSymbols(ctx context.Context, repoID, revision, query string) ([]codeintel.Result, error) {
	files, err := i.Repository.ListFiles(ctx, repoID, revision)
	if err != nil {
		return nil, err
	}
	var results []codeintel.Result
	for _, file := range files {
		if filepath.Ext(file) != ".go" {
			continue
		}
		parsed, fset, err := i.parse(ctx, repoID, revision, file)
		if err != nil {
			return nil, err
		}
		goast.Inspect(parsed, func(node goast.Node) bool {
			switch n := node.(type) {
			case *goast.FuncDecl:
				if matches(n.Name.Name, query) {
					pos := fset.Position(n.Pos())
					end := fset.Position(n.End())
					results = append(results, symbolResult(repoID, revision, file, "function", n.Name.Name, pos.Line, end.Line))
				}
			case *goast.TypeSpec:
				if matches(n.Name.Name, query) {
					pos := fset.Position(n.Pos())
					end := fset.Position(n.End())
					results = append(results, symbolResult(repoID, revision, file, "type", n.Name.Name, pos.Line, end.Line))
				}
			}
			return true
		})
	}
	return results, nil
}

func (i GoIndex) FindReferences(ctx context.Context, repoID, revision, symbol string) ([]codeintel.Result, error) {
	files, err := i.Repository.ListFiles(ctx, repoID, revision)
	if err != nil {
		return nil, err
	}
	name := symbol
	if idx := strings.LastIndex(symbol, "."); idx >= 0 {
		name = symbol[idx+1:]
	}
	var results []codeintel.Result
	for _, file := range files {
		if filepath.Ext(file) != ".go" {
			continue
		}
		parsed, fset, err := i.parse(ctx, repoID, revision, file)
		if err != nil {
			return nil, err
		}
		goast.Inspect(parsed, func(node goast.Node) bool {
			ident, ok := node.(*goast.Ident)
			if !ok || ident.Name != name {
				return true
			}
			pos := fset.Position(ident.Pos())
			results = append(results, codeintel.Result{
				RefKind:       codeintel.RefSymbol,
				RepoID:        repoID,
				Revision:      revision,
				FilePath:      file,
				LineStart:     pos.Line,
				LineEnd:       pos.Line,
				Language:      "go",
				SymbolName:    ident.Name,
				QualifiedName: symbol,
				SymbolKind:    "reference",
				ContentHash:   codeintel.HashContent(file + ":" + symbol + ":" + pos.String()),
				Source:        "go_ast",
			})
			return true
		})
	}
	return results, nil
}

func (i GoIndex) parse(ctx context.Context, repoID, revision, path string) (*goast.File, *token.FileSet, error) {
	content, err := i.Repository.ReadFile(ctx, repoID, revision, path)
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	return parsed, fset, err
}

func symbolResult(repoID, revision, file, kind, name string, start, end int) codeintel.Result {
	return codeintel.Result{
		RefKind:       codeintel.RefSymbol,
		RepoID:        repoID,
		Revision:      revision,
		FilePath:      file,
		LineStart:     start,
		LineEnd:       end,
		Language:      "go",
		SymbolName:    name,
		QualifiedName: name,
		SymbolKind:    kind,
		ContentHash:   codeintel.HashContent(file + ":" + kind + ":" + name),
		Source:        "go_ast",
	}
}

func matches(name, query string) bool {
	return name == query || strings.Contains(strings.ToLower(name), strings.ToLower(query))
}
