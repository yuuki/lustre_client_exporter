package reader

import (
	"context"
	"fmt"
	"strings"
)

// FakeReader implements Reader for testing with in-memory data.
type FakeReader struct {
	Files    map[string][]byte
	Globs    map[string][]string
	Commands map[string][]byte
	Errors   map[string]error
}

func NewFakeReader() *FakeReader {
	return &FakeReader{
		Files:    make(map[string][]byte),
		Globs:    make(map[string][]string),
		Commands: make(map[string][]byte),
		Errors:   make(map[string]error),
	}
}

func (r *FakeReader) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err, ok := r.Errors[path]; ok {
		return nil, err
	}
	data, ok := r.Files[path]
	if !ok {
		return nil, fmt.Errorf("fake: file not found: %s", path)
	}
	return data, nil
}

func (r *FakeReader) Glob(ctx context.Context, pattern string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err, ok := r.Errors[pattern]; ok {
		return nil, err
	}
	matches, ok := r.Globs[pattern]
	if !ok {
		return nil, nil
	}
	return matches, nil
}

func (r *FakeReader) RunCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	if err, ok := r.Errors[key]; ok {
		return nil, err
	}
	data, ok := r.Commands[key]
	if !ok {
		return nil, fmt.Errorf("fake: command not found: %s", key)
	}
	return data, nil
}
