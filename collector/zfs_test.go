package collector

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pdf/zfs_exporter/v2/zfs/mock_zfs"
)

func TestZFSCollectInvalidPools(t *testing.T) {
	const result = `# HELP zfs_scrape_collector_duration_seconds zfs_exporter: Duration of a collector scrape.
# TYPE zfs_scrape_collector_duration_seconds gauge
zfs_scrape_collector_duration_seconds{collector="pool"} 0
# HELP zfs_scrape_collector_success zfs_exporter: Whether a collector succeeded.
# TYPE zfs_scrape_collector_success gauge
zfs_scrape_collector_success{collector="pool"} 0
`

	ctrl, ctx := gomock.WithContext(context.Background(), t)
	zfsClient := mock_zfs.NewMockClient(ctrl)
	zfsClient.EXPECT().PoolNames().Return(nil, errors.New(`Error returned from PoolNames()`)).Times(1)

	config := defaultConfig(zfsClient)
	config.DisableMetrics = false
	collector, err := NewZFS(config)
	collector.Collectors = map[string]State{
		`pool`: {
			Name:       "pool",
			Enabled:    boolPointer(true),
			Properties: stringPointer(``),
			factory:    newPoolCollector,
		},
	}
	if err != nil {
		t.Fatal(err)
	}

	if err = callCollector(ctx, collector, []byte(result), []string{`zfs_scrape_collector_duration_seconds`, `zfs_scrape_collector_success`}); err != nil {
		t.Fatal(err)
	}
}
