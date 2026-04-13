package reader

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
)

// Reader abstracts file system reads and command execution for testability.
type Reader interface {
	ReadFile(path string) ([]byte, error)
	Glob(pattern string) ([]string, error)
	RunCommand(ctx context.Context, name string, args ...string) ([]byte, error)
}

// FSReader implements Reader using the real filesystem.
type FSReader struct {
	RootFS string
}

func NewFSReader(rootFS string) *FSReader {
	return &FSReader{RootFS: rootFS}
}

func (r *FSReader) resolvePath(path string) string {
	if r.RootFS == "" || r.RootFS == "/" {
		return path
	}
	return filepath.Join(r.RootFS, path)
}

func (r *FSReader) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(r.resolvePath(path))
}

func (r *FSReader) Glob(pattern string) ([]string, error) {
	resolved := r.resolvePath(pattern)
	matches, err := filepath.Glob(resolved)
	if err != nil {
		return nil, err
	}
	if r.RootFS != "" && r.RootFS != "/" {
		result := make([]string, len(matches))
		for i, m := range matches {
			rel, err := filepath.Rel(r.RootFS, m)
			if err != nil {
				result[i] = m
			} else {
				result[i] = "/" + rel
			}
		}
		return result, nil
	}
	return matches, nil
}

func (r *FSReader) RunCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}
