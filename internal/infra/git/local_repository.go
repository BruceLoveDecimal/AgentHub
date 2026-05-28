package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type LocalRepository struct {
	Roots map[string]string
}

func NewLocalRepository(roots map[string]string) LocalRepository {
	clone := make(map[string]string, len(roots))
	for repoID, root := range roots {
		clone[repoID] = root
	}
	return LocalRepository{Roots: clone}
}

func (r LocalRepository) ListFiles(_ context.Context, repoID, _ string) ([]string, error) {
	root, err := r.root(repoID)
	if err != nil {
		return nil, err
	}
	var files []string
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	return files, err
}

func (r LocalRepository) ReadFile(_ context.Context, repoID, _, path string) (string, error) {
	root, err := r.root(repoID)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(path)
	if filepath.IsAbs(clean) || clean == "." || strings.HasPrefix(clean, "..") {
		return "", errors.New("unsafe repository path")
	}
	content, err := os.ReadFile(filepath.Join(root, clean))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (r LocalRepository) root(repoID string) (string, error) {
	root := r.Roots[repoID]
	if root == "" {
		return "", errors.New("repository root not configured")
	}
	return root, nil
}
