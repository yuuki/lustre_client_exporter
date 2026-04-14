package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yuuki/lustre_exporter/collector"
	"github.com/yuuki/lustre_exporter/internal/discovery"
	"github.com/yuuki/lustre_exporter/internal/reader"
)

var version = "dev"

// Config holds all CLI flag values.
type Config struct {
	// Web
	ListenAddress string
	TelemetryPath string
	WebConfigFile string

	// Collectors
	CollectorClient  bool
	CollectorLNet    bool
	CollectorHealth  bool
	CollectorSptlrpc bool
	CollectorLpcc    bool

	// Tuning
	LNetSource    string
	ScrapeTimeout time.Duration
	SourceTimeout time.Duration
	Strict        bool

	// Paths
	RootFS  string
	ProcFS  string
	SysFS   string
	DebugFS string
	LNetCtl string
	LpccBin string

	// Logging
	LogLevel string

	// Meta
	ShowVersion bool
}

func parseFlags() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.ListenAddress, "web.listen-address", ":9169", "Address to listen on for web interface and telemetry.")
	flag.StringVar(&cfg.TelemetryPath, "web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	flag.StringVar(&cfg.WebConfigFile, "web.config.file", "", "Unsupported: exporter-toolkit TLS/auth configuration is not implemented.")

	flag.BoolVar(&cfg.CollectorClient, "collector.client", true, "Enable the client (llite/mdc/osc) collector.")
	flag.BoolVar(&cfg.CollectorLNet, "collector.lnet", true, "Enable the LNet collector.")
	flag.BoolVar(&cfg.CollectorHealth, "collector.health", true, "Enable the health collector.")
	flag.BoolVar(&cfg.CollectorSptlrpc, "collector.sptlrpc", true, "Enable the sptlrpc collector.")
	flag.BoolVar(&cfg.CollectorLpcc, "collector.lpcc", false, "Enable the LPCC (Lustre PCC) collector.")

	flag.StringVar(&cfg.LNetSource, "collector.lnet.source", "auto", "LNet data source: auto, debugfs, or lnetctl.")
	flag.DurationVar(&cfg.ScrapeTimeout, "collector.scrape-timeout", 30*time.Second, "Maximum duration of a scrape.")
	flag.DurationVar(&cfg.SourceTimeout, "collector.source-timeout", 10*time.Second, "Timeout for individual source reads.")
	flag.BoolVar(&cfg.Strict, "collector.strict", false, "Fail the scrape if any source is unavailable.")

	flag.StringVar(&cfg.RootFS, "path.rootfs", "/", "Root filesystem path prefix.")
	flag.StringVar(&cfg.ProcFS, "path.procfs", "/proc", "procfs mount point.")
	flag.StringVar(&cfg.SysFS, "path.sysfs", "/sys", "sysfs mount point.")
	flag.StringVar(&cfg.DebugFS, "path.debugfs", "/sys/kernel/debug", "debugfs mount point.")
	flag.StringVar(&cfg.LNetCtl, "path.lnetctl", "lnetctl", "Path to the lnetctl binary.")
	flag.StringVar(&cfg.LpccBin, "path.lpcc", "lpcc", "Path to the lpcc binary.")

	flag.StringVar(&cfg.LogLevel, "log.level", "info", "Log level: debug, info, warn, error.")

	flag.BoolVar(&cfg.ShowVersion, "version", false, "Print version and exit.")

	flag.Parse()
	return cfg
}

func setupLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}

func validateConfig(cfg *Config) error {
	if cfg.WebConfigFile != "" {
		return fmt.Errorf("-web.config.file is not supported by this build")
	}
	switch cfg.LNetSource {
	case "auto", "debugfs", "lnetctl":
		return nil
	default:
		return fmt.Errorf("invalid -collector.lnet.source %q: expected auto, debugfs, or lnetctl", cfg.LNetSource)
	}
}

func run() int {
	cfg := parseFlags()

	if cfg.ShowVersion {
		fmt.Printf("lustre_client_exporter %s\n", version)
		return 0
	}

	logger := setupLogger(cfg.LogLevel)
	if err := validateConfig(cfg); err != nil {
		logger.Error("invalid configuration", "error", err)
		return 1
	}

	pathCfg := discovery.PathConfig{
		ProcFS:  cfg.ProcFS,
		SysFS:   cfg.SysFS,
		DebugFS: cfg.DebugFS,
	}

	r := reader.NewFSReader(cfg.RootFS)

	var collectors []collector.Collector
	if cfg.CollectorHealth {
		collectors = append(collectors, collector.NewHealthCollector(r, pathCfg))
	}
	if cfg.CollectorSptlrpc {
		collectors = append(collectors, collector.NewSptlrpcCollector(r, pathCfg))
	}
	if cfg.CollectorLNet {
		lnetSource := discovery.LNetSourceAuto
		switch cfg.LNetSource {
		case "debugfs":
			lnetSource = discovery.LNetSourceDebugFS
		case "lnetctl":
			lnetSource = discovery.LNetSourceLNetCtl
		}
		collectors = append(collectors, collector.NewLNetCollector(r, pathCfg, lnetSource, cfg.LNetCtl, logger))
	}
	if cfg.CollectorClient {
		collectors = append(collectors, collector.NewClientCollectorWithStrict(r, pathCfg, logger, cfg.Strict))
	}
	if cfg.CollectorLpcc {
		collectors = append(collectors, collector.NewLpccCollector(r, cfg.LpccBin, logger))
	}

	reg := prometheus.NewRegistry()
	registry := collector.NewRegistryWithStrict(logger, cfg.ScrapeTimeout, cfg.SourceTimeout, cfg.Strict, collectors...)
	reg.MustRegister(registry)

	http.Handle(cfg.TelemetryPath, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<html><head><title>Lustre Client Exporter</title></head>`+
			`<body><h1>Lustre Client Exporter</h1>`+
			`<p><a href="%s">Metrics</a></p></body></html>`, cfg.TelemetryPath)
	})

	logger.Info("starting lustre_client_exporter", "address", cfg.ListenAddress, "version", version)
	if err := http.ListenAndServe(cfg.ListenAddress, nil); err != nil {
		logger.Error("failed to start server", "error", err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(run())
}
