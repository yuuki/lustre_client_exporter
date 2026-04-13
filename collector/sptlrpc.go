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

type SptlrpcCollector struct {
	reader  reader.Reader
	pathCfg discovery.PathConfig
}

func NewSptlrpcCollector(r reader.Reader, cfg discovery.PathConfig) *SptlrpcCollector {
	return &SptlrpcCollector{reader: r, pathCfg: cfg}
}

func (c *SptlrpcCollector) Name() string { return "sptlrpc" }

func (c *SptlrpcCollector) Collect(ctx context.Context) ([]prometheus.Metric, error) {
	path := discovery.SptlrpcPath(c.pathCfg)

	data, err := c.reader.ReadFile(path)
	if err != nil {
		return nil, err
	}

	observations, err := parser.ParseEncryptPagePools(data, path)
	if err != nil {
		return nil, err
	}

	mapped, err := mapper.Map(observations)
	if err != nil {
		return nil, err
	}

	return emitter.Emit(mapped), nil
}
