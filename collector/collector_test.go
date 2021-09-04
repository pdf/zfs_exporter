package collector

import (
	"bytes"
	"context"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pdf/zfs_exporter/zfs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

var (
	logger = log.NewNopLogger()
	//logger = log.NewLogfmtLogger(os.Stderr)
)

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

func defaultConfig(z zfs.Client) (ZFSConfig, error) {
	duration, err := time.ParseDuration(`5m`)
	if err != nil {
		return ZFSConfig{}, err
	}
	return ZFSConfig{
		DisableMetrics: true,
		Deadline:       duration,
		Logger:         logger,
		ZFSClient:      z,
	}, nil
}

func stringPointer(s string) *string {
	return &s
}

func boolPointer(b bool) *bool {
	return &b
}
