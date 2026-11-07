//go:build unix

package reader

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestFSReader_RunCommand(t *testing.T) {
	r := NewFSReader("")
	out, err := r.RunCommand(context.Background(), "sh", "-c", "printf 'net:\\n'")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "net:\n" {
		t.Errorf("got %q, want %q", out, "net:\n")
	}
}

// A wrapper script that is killed on timeout leaves its child running with
// the inherited stdout. RunCommand must still return soon after the timeout
// instead of waiting for that child, or a hung lnetctl behind a wrapper would
// block the scrape past every configured timeout.
func TestFSReader_RunCommandHonorsTimeoutWhenChildKeepsStdout(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	t.Cleanup(func() { killPIDFile(t, pidFile) })

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	r := NewFSReader("")
	start := time.Now()
	_, err := r.RunCommand(ctx, "sh", "-c", `sleep 30 & echo $! > "$1"; wait`, "sh", pidFile)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error from a command killed on timeout")
	}
	// The bound is the timeout plus commandWaitDelay with generous slack for
	// a loaded machine; without the wait delay this takes the full 30s.
	if limit := 10 * time.Second; elapsed > limit {
		t.Fatalf("RunCommand returned after %v, want at most %v", elapsed, limit)
	}
}

// killPIDFile kills the process whose PID a test script wrote to path, if any.
func killPIDFile(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Errorf("pid file %s: %v", path, err)
		return
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		t.Errorf("kill %d: %v", pid, err)
	}
}

// A read that blocks in the kernel (a hung Lustre proc file; here, a FIFO with
// no writer) cannot be interrupted. ReadFile must return when its context is
// done, and the goroutine it leaves behind must exit once the read completes.
// goleak in TestMain checks the second half.
func TestFSReader_ReadFileReturnsWhenContextDoneDuringBlockedRead(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "stats")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	r := NewFSReader("")
	_, err := r.ReadFile(ctx, fifo)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want %v", err, context.DeadlineExceeded)
	}

	// Unblock the abandoned read: opening the FIFO for writing lets its open
	// return, and closing it gives the reader EOF.
	w, err := os.OpenFile(fifo, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}
