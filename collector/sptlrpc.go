package collector

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/yuuki/lustre_exporter/internal/discovery"
	"github.com/yuuki/lustre_exporter/internal/emitter"
	"github.com/yuuki/lustre_exporter/internal/mapper"
	"github.com/yuuki/lustre_exporter/internal/parser"
	"github.com/yuuki/lustre_exporter/internal/reader"
)

// SptlrpcCollector reads sptlrpc/encrypt_page_pools and emits 15 metrics.
type SptlrpcCollector struct {
	reader  reader.Reader
	pathCfg discovery.PathConfig
}

func NewSptlrpcCollector(r reader.Reader, cfg discovery.PathConfig) *SptlrpcCollector {
	return &SptlrpcCollector{reader: r, pathCfg: cfg}
}

func (c *SptlrpcCollector) Name() string { return "sptlrpc" }

func (c *SptlrpcCollector) Collect(ctx context.Context) ([]prometheus.Metric, error) {
	target, err := discovery.DiscoverSptlrpc(c.reader, c.pathCfg)
	if err != nil {
		return nil, err
	}

	data, err := c.reader.ReadFile(target.Path)
	if err != nil {
		return nil, err
	}

	observations, err := parser.ParseEncryptPagePools(data, target.Path)
	if err != nil {
		return nil, err
	}

	mapped, err := mapper.Map(observations)
	if err != nil {
		return nil, err
	}

	return emitter.Emit(mapped), nil
}
