package reader

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFakeReader_ReadFile(t *testing.T) {
	r := NewFakeReader()
	r.Files["/proc/fs/lustre/health_check"] = []byte("healthy\n")

	data, err := r.ReadFile(context.Background(), "/proc/fs/lustre/health_check")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "healthy\n" {
		t.Errorf("got %q, want %q", string(data), "healthy\n")
	}
}

func TestFakeReader_ReadFileMissing(t *testing.T) {
	r := NewFakeReader()

	_, err := r.ReadFile(context.Background(), "/nonexistent")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestFakeReader_Glob(t *testing.T) {
	r := NewFakeReader()
	r.Globs["/proc/fs/lustre/llite/*/stats"] = []string{
		"/proc/fs/lustre/llite/scratch-ffff0001/stats",
		"/proc/fs/lustre/llite/home-ffff0002/stats",
	}

	matches, err := r.Glob(context.Background(), "/proc/fs/lustre/llite/*/stats")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}
}

func TestFakeReader_RunCommand(t *testing.T) {
	r := NewFakeReader()
	r.Commands["lnetctl stats show"] = []byte(`{"send_count": 100}`)

	data, err := r.RunCommand(context.Background(), "lnetctl", "stats", "show")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `{"send_count": 100}` {
		t.Errorf("got %q", string(data))
	}
}

func TestFSReader_ReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "health_check")
	if err := os.WriteFile(path, []byte("healthy\n"), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewFSReader("")
	data, err := r.ReadFile(context.Background(), path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "healthy\n" {
		t.Errorf("got %q, want %q", string(data), "healthy\n")
	}
}

func TestFSReader_WithRootFS(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sys", "fs", "lustre")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "health_check"), []byte("healthy\n"), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewFSReader(dir)
	data, err := r.ReadFile(context.Background(), "/sys/fs/lustre/health_check")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "healthy\n" {
		t.Errorf("got %q, want %q", string(data), "healthy\n")
	}
}

func TestFSReader_Glob(t *testing.T) {
	dir := t.TempDir()
	llite := filepath.Join(dir, "proc", "fs", "lustre", "llite", "scratch-ffff0001")
	if err := os.MkdirAll(llite, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(llite, "stats"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewFSReader(dir)
	matches, err := r.Glob(context.Background(), "/proc/fs/lustre/llite/*/stats")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(matches))
	}
	if matches[0] != "/proc/fs/lustre/llite/scratch-ffff0001/stats" {
		t.Errorf("got %q", matches[0])
	}
}

func TestFSReader_ReadFileHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := NewFSReader("")
	_, err := r.ReadFile(ctx, "/definitely/not/here")
	if err == nil {
		t.Fatal("expected canceled context error")
	}
}
