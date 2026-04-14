package reader

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Reader abstracts file system reads and command execution for testability.
type Reader interface {
	ReadFile(ctx context.Context, path string) ([]byte, error)
	Glob(ctx context.Context, pattern string) ([]string, error)
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

func (r *FSReader) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	type result struct {
		data []byte
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		data, err := os.ReadFile(r.resolvePath(path))
		ch <- result{data: data, err: err}
	}()

	select {
	case res := <-ch:
		return res.data, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *FSReader) Glob(ctx context.Context, pattern string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	type result struct {
		matches []string
		err     error
	}
	ch := make(chan result, 1)
	go func() {
		matches, err := filepath.Glob(r.resolvePath(pattern))
		ch <- result{matches: matches, err: err}
	}()

	var matches []string
	select {
	case res := <-ch:
		if res.err != nil {
			return nil, res.err
		}
		matches = res.matches
	case <-ctx.Done():
		return nil, ctx.Err()
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

// ReadFirstAvailable tries each path in order, returning data from the first successful read.
func ReadFirstAvailable(ctx context.Context, r Reader, paths []string) ([]byte, string, error) {
	if len(paths) == 0 {
		return nil, "", fmt.Errorf("no paths provided")
	}
	var lastErr error
	for _, p := range paths {
		data, err := r.ReadFile(ctx, p)
		if err != nil {
			lastErr = err
			continue
		}
		return data, p, nil
	}
	return nil, "", lastErr
}
