package collector

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/pdf/zfs_exporter/v2/zfs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

var logger = slog.New(slog.NewTextHandler(io.Discard, nil))

func callCollector(ctx context.Context, collector prometheus.Collector, metricResults []byte, metricNames []string) error {
	result := make(chan error)
	go func() {
		result <- testutil.CollectAndCompare(collector, bytes.NewBuffer(metricResults), metricNames...)
	}()

	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func defaultConfig(z zfs.Client) ZFSConfig {
	return ZFSConfig{
		DisableMetrics: true,
		Deadline:       5 * time.Minute,
		Logger:         logger,
		ZFSClient:      z,
	}
}

func stringPointer(s string) *string {
	return &s
}

func boolPointer(b bool) *bool {
	return &b
}
