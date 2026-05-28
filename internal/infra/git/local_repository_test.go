package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalRepositoryReadsSafePaths(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "auth.go"), []byte("package auth\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	repo := NewLocalRepository(map[string]string{"repo-1": root})

	content, err := repo.ReadFile(context.Background(), "repo-1", "abc123", "internal/auth.go")
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if content != "package auth\n" {
		t.Fatalf("unexpected content: %q", content)
	}
	if _, err := repo.ReadFile(context.Background(), "repo-1", "abc123", "../secret"); err == nil {
		t.Fatal("expected unsafe path error")
	}
}
