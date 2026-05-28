package codeintel

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/capability"
)

type Primitive string

const (
	PrimitiveGrep       Primitive = "code.grep"
	PrimitiveReadFile   Primitive = "code.read_file"
	PrimitiveSymbols    Primitive = "code.symbols"
	PrimitiveReferences Primitive = "code.references"
	PrimitiveOwnership  Primitive = "code.ownership"
)

type RefKind string

const (
	RefFileRange RefKind = "file_range"
	RefSymbol    RefKind = "symbol"
	RefOwnership RefKind = "ownership"
)

type Request struct {
	ID           string
	WorkspaceID  string
	OrgID        string
	RepoID       string
	Revision     string
	Primitive    Primitive
	Query        string
	Path         string
	PathGlob     string
	Symbol       string
	LineStart    int
	LineEnd      int
	MaxResults   int
	UsedByAgent  bool
	ActorChainID string
}

func (r Request) Validate() error {
	if r.WorkspaceID == "" {
		return errors.New("workspace id is required")
	}
	if r.OrgID == "" {
		return errors.New("org id is required")
	}
	if r.RepoID == "" {
		return errors.New("repo id is required")
	}
	if r.Revision == "" {
		return errors.New("revision is required")
	}
	switch r.Primitive {
	case PrimitiveGrep:
		if strings.TrimSpace(r.Query) == "" {
			return errors.New("grep query is required")
		}
	case PrimitiveReadFile:
		if strings.TrimSpace(r.Path) == "" {
			return errors.New("file path is required")
		}
	case PrimitiveSymbols:
		if strings.TrimSpace(r.Query) == "" {
			return errors.New("symbol query is required")
		}
	case PrimitiveReferences:
		if strings.TrimSpace(r.Symbol) == "" {
			return errors.New("symbol is required")
		}
	case PrimitiveOwnership:
		if strings.TrimSpace(r.Path) == "" {
			return errors.New("ownership path is required")
		}
	default:
		return fmt.Errorf("unsupported code intelligence primitive %q", r.Primitive)
	}
	if r.LineStart < 0 || r.LineEnd < 0 {
		return errors.New("line numbers cannot be negative")
	}
	if r.LineStart > 0 && r.LineEnd > 0 && r.LineEnd < r.LineStart {
		return errors.New("line_end cannot be before line_start")
	}
	if r.Path != "" && !SafePath(r.Path) {
		return fmt.Errorf("unsafe file path %q", r.Path)
	}
	if r.PathGlob != "" && strings.Contains(r.PathGlob, "..") {
		return fmt.Errorf("unsafe path glob %q", r.PathGlob)
	}
	return nil
}

func (r Request) CapabilityName() capability.Name {
	switch r.Primitive {
	case PrimitiveGrep:
		return capability.CodeGrep
	case PrimitiveReadFile:
		return capability.CodeReadFile
	case PrimitiveSymbols:
		return capability.CodeSymbols
	case PrimitiveReferences:
		return capability.CodeReferences
	case PrimitiveOwnership:
		return capability.CodeOwnership
	default:
		return capability.Name(r.Primitive)
	}
}

func (r Request) Resource() capability.Resource {
	kind := capability.ResourceRepo
	id := r.RepoID
	if r.Path != "" || r.PathGlob != "" {
		kind = capability.ResourcePath
		id = r.Path
		if id == "" {
			id = r.PathGlob
		}
	}
	return capability.Resource{
		OrgID:       r.OrgID,
		Kind:        kind,
		ID:          id,
		RepoID:      r.RepoID,
		WorkspaceID: r.WorkspaceID,
		Path:        r.Path,
	}
}

type Result struct {
	RefKind            RefKind  `json:"ref_kind"`
	RepoID             string   `json:"repo_id"`
	Revision           string   `json:"revision"`
	FilePath           string   `json:"file_path,omitempty"`
	LineStart          int      `json:"line_start,omitempty"`
	LineEnd            int      `json:"line_end,omitempty"`
	Language           string   `json:"language,omitempty"`
	SymbolName         string   `json:"symbol_name,omitempty"`
	QualifiedName      string   `json:"qualified_name,omitempty"`
	SymbolKind         string   `json:"symbol_kind,omitempty"`
	Owners             []string `json:"owners,omitempty"`
	Protected          bool     `json:"protected,omitempty"`
	RequiresApproval   bool     `json:"requires_approval,omitempty"`
	Content            string   `json:"content,omitempty"`
	ContentHash        string   `json:"content_hash,omitempty"`
	Source             string   `json:"source"`
	ContextReferenceID string   `json:"context_reference_id,omitempty"`
}

func (r Result) WithContentHash() Result {
	if r.ContentHash == "" && r.Content != "" {
		r.ContentHash = HashContent(r.Content)
	}
	return r
}

type Response struct {
	WorkspaceID string    `json:"workspace_id"`
	RepoID      string    `json:"repo_id"`
	Revision    string    `json:"revision"`
	Primitive   Primitive `json:"primitive"`
	Results     []Result  `json:"results"`
}

type ContextReference struct {
	ID          string
	WorkspaceID string
	AgentRunID  string
	RefKind     RefKind
	RefID       string
	RepoID      string
	FilePath    string
	LineStart   int
	LineEnd     int
	ContentHash string
	Rank        int
	Metadata    map[string]string
}

func HashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func SafePath(path string) bool {
	if path == "" || filepath.IsAbs(path) {
		return false
	}
	clean := filepath.Clean(path)
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, "../") && !strings.Contains(clean, "/../")
}

func LanguageForPath(path string) string {
	switch filepath.Ext(path) {
	case ".go":
		return "go"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".py":
		return "python"
	case ".md":
		return "markdown"
	default:
		return strings.TrimPrefix(filepath.Ext(path), ".")
	}
}
