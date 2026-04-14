package collector

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/yuuki/lustre_client_exporter/internal/discovery"
	"github.com/yuuki/lustre_client_exporter/internal/reader"
)

func TestHealthCollector_MissingFile_ReturnsError(t *testing.T) {
	r := reader.NewFakeReader()
	c := NewHealthCollector(r, discovery.DefaultPathConfig())
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for missing health_check file")
	}
}

func TestSptlrpcCollector_MissingFile_ReturnsError(t *testing.T) {
	r := reader.NewFakeReader()
	c := NewSptlrpcCollector(r, discovery.DefaultPathConfig())
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for missing encrypt_page_pools")
	}
}

func TestLNetCollector_NoSources_ReturnsError(t *testing.T) {
	r := reader.NewFakeReader()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := NewLNetCollector(r, discovery.DefaultPathConfig(), discovery.LNetSourceDebugFS, "lnetctl", logger)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error when no LNet sources available")
	}
}

func TestClientCollector_EmptyFS_ReturnsEmpty(t *testing.T) {
	r := reader.NewFakeReader()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := NewClientCollector(r, discovery.DefaultPathConfig(), logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 0 {
		t.Errorf("got %d metrics for empty FS, want 0", len(metrics))
	}
}

func TestSptlrpcCollector_MalformedValue(t *testing.T) {
	r := reader.NewFakeReader()
	r.Files["/sys/kernel/debug/lustre/sptlrpc/encrypt_page_pools"] = []byte("physical pages: not_a_number\n")

	c := NewSptlrpcCollector(r, discovery.DefaultPathConfig())
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for malformed number")
	}
}

func TestLNetCollector_Auto_FallsBackToLNetCtl(t *testing.T) {
	r := reader.NewFakeReader()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	r.Commands["lnetctl stats show"] = []byte(`{"statistics":{"msgs_alloc":0,"msgs_max":1,"errors":0,"send_count":10,"recv_count":5,"route_count":0,"drop_count":0,"send_length":100,"recv_length":50,"route_length":0,"drop_length":0}}`)

	c := NewLNetCollector(r, discovery.DefaultPathConfig(), discovery.LNetSourceAuto, "lnetctl", logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 11 {
		t.Errorf("got %d metrics, want 11 (stats only, no params)", len(metrics))
	}
}
