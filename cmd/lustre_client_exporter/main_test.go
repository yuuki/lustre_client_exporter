package main

import (
	"flag"
	"testing"
)

func TestParseFlags_Defaults(t *testing.T) {
	// Reset flags for testing
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	cfg := &Config{}

	flag.StringVar(&cfg.ListenAddress, "web.listen-address", ":9169", "")
	flag.StringVar(&cfg.TelemetryPath, "web.telemetry-path", "/metrics", "")
	flag.BoolVar(&cfg.CollectorClient, "collector.client", true, "")
	flag.BoolVar(&cfg.CollectorLNet, "collector.lnet", true, "")
	flag.BoolVar(&cfg.CollectorHealth, "collector.health", true, "")
	flag.BoolVar(&cfg.CollectorSptlrpc, "collector.sptlrpc", true, "")
	flag.BoolVar(&cfg.CollectorLpcc, "collector.lpcc", false, "")
	flag.BoolVar(&cfg.Strict, "collector.strict", false, "")
	flag.StringVar(&cfg.LogLevel, "log.level", "info", "")

	if err := flag.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}

	if cfg.ListenAddress != ":9169" {
		t.Errorf("listen address = %q, want %q", cfg.ListenAddress, ":9169")
	}
	if cfg.TelemetryPath != "/metrics" {
		t.Errorf("telemetry path = %q, want %q", cfg.TelemetryPath, "/metrics")
	}
	if !cfg.CollectorHealth {
		t.Error("collector.health should be enabled by default")
	}
	if !cfg.CollectorClient {
		t.Error("collector.client should be enabled by default")
	}
	if cfg.Strict {
		t.Error("strict should be false by default")
	}
	if cfg.CollectorLpcc {
		t.Error("collector.lpcc should be disabled by default")
	}
}

func TestParseFlags_DisableCollector(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	cfg := &Config{}

	flag.BoolVar(&cfg.CollectorHealth, "collector.health", true, "")
	flag.BoolVar(&cfg.CollectorClient, "collector.client", true, "")

	if err := flag.CommandLine.Parse([]string{"-collector.health=false"}); err != nil {
		t.Fatal(err)
	}

	if cfg.CollectorHealth {
		t.Error("collector.health should be disabled")
	}
	if !cfg.CollectorClient {
		t.Error("collector.client should still be enabled")
	}
}

func TestParseFlags_UnknownFlag(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)

	err := flag.CommandLine.Parse([]string{"-collector.ost"})
	if err == nil {
		t.Error("expected error for unknown flag -collector.ost")
	}
}

func TestParseFlags_CustomAddress(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	cfg := &Config{}

	flag.StringVar(&cfg.ListenAddress, "web.listen-address", ":9169", "")

	if err := flag.CommandLine.Parse([]string{"-web.listen-address=:8080"}); err != nil {
		t.Fatal(err)
	}

	if cfg.ListenAddress != ":8080" {
		t.Errorf("listen address = %q, want %q", cfg.ListenAddress, ":8080")
	}
}

func TestValidateConfigRejectsUnsupportedWebConfigFile(t *testing.T) {
	cfg := &Config{WebConfigFile: "/etc/exporter/web.yml", LNetSource: "auto"}

	if err := validateConfig(cfg); err == nil {
		t.Fatal("expected error for unsupported web.config.file")
	}
}

func TestValidateConfigRejectsInvalidLNetSource(t *testing.T) {
	cfg := &Config{LNetSource: "bogus"}

	if err := validateConfig(cfg); err == nil {
		t.Fatal("expected error for invalid collector.lnet.source")
	}
}
