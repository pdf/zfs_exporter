package collector

import (
	"context"
	"strings"
	"testing"

	"github.com/pdf/zfs_exporter/v2/zfs/mock_zfs"
	"go.uber.org/mock/gomock"
)

func TestPoolMetrics(t *testing.T) {
	testCases := []struct {
		name           string
		pools          []string
		explicitPools  []string
		propsRequested []string
		metricNames    []string
		propsResults   map[string]map[string]string
		metricResults  string
	}{
		{
			name:           `all metrics`,
			pools:          []string{`testpool`},
			propsRequested: []string{`allocated`, `dedupratio`, `capacity`, `expandsize`, `fragmentation`, `free`, `freeing`, `health`, `leaked`, `readonly`, `size`},
			metricNames:    []string{`zfs_pool_allocated_bytes`, `zfs_pool_deduplication_ratio`, `zfs_pool_capacity_ratio`, `zfs_pool_expand_size_bytes`, `zfs_pool_fragmentation_ratio`, `zfs_pool_free_bytes`, `zfs_pool_freeing_bytes`, `zfs_pool_health`, `zfs_pool_leaked_bytes`, `zfs_pool_readonly`, `zfs_pool_size_bytes`},
			propsResults: map[string]map[string]string{
				`testpool`: {
					`allocated`:     `1024`,
					`dedupratio`:    `2.50`,
					`capacity`:      `50`,
					`expandsize`:    `2048`,
					`fragmentation`: `25`,
					`free`:          `1024`,
					`freeing`:       `0`,
					`health`:        `ONLINE`,
					`leaked`:        `1`,
					`readonly`:      `off`,
					`size`:          `2048`,
				},
			},
			metricResults: `# HELP zfs_pool_allocated_bytes Amount of storage in bytes used within the pool.
# TYPE zfs_pool_allocated_bytes gauge
zfs_pool_allocated_bytes{pool="testpool"} 1024
# HELP zfs_pool_capacity_ratio Ratio of pool space used.
# TYPE zfs_pool_capacity_ratio gauge
zfs_pool_capacity_ratio{pool="testpool"} 0.5
# HELP zfs_pool_deduplication_ratio The ratio of deduplicated size vs undeduplicated size for data in this pool.
# TYPE zfs_pool_deduplication_ratio gauge
zfs_pool_deduplication_ratio{pool="testpool"} 0.4
# HELP zfs_pool_expand_size_bytes Amount of uninitialized space within the pool or device that can be used to increase the total capacity of the pool.
# TYPE zfs_pool_expand_size_bytes gauge
zfs_pool_expand_size_bytes{pool="testpool"} 2048
# HELP zfs_pool_fragmentation_ratio The fragmentation ratio of the pool.
# TYPE zfs_pool_fragmentation_ratio gauge
zfs_pool_fragmentation_ratio{pool="testpool"} 0.25
# HELP zfs_pool_free_bytes The amount of free space in bytes available in the pool.
# TYPE zfs_pool_free_bytes gauge
zfs_pool_free_bytes{pool="testpool"} 1024
# HELP zfs_pool_freeing_bytes The amount of space in bytes remaining to be freed following the destruction of a file system or snapshot.
# TYPE zfs_pool_freeing_bytes gauge
zfs_pool_freeing_bytes{pool="testpool"} 0
# HELP zfs_pool_health Health status code for the pool [0: ONLINE, 1: DEGRADED, 2: FAULTED, 3: OFFLINE, 4: UNAVAIL, 5: REMOVED, 6: SUSPENDED].
# TYPE zfs_pool_health gauge
zfs_pool_health{pool="testpool"} 0
# HELP zfs_pool_leaked_bytes Number of leaked bytes in the pool.
# TYPE zfs_pool_leaked_bytes gauge
zfs_pool_leaked_bytes{pool="testpool"} 1
# HELP zfs_pool_readonly Read-only status of the pool [0: read-write, 1: read-only].
# TYPE zfs_pool_readonly gauge
zfs_pool_readonly{pool="testpool"} 0
# HELP zfs_pool_size_bytes Total size in bytes of the storage pool.
# TYPE zfs_pool_size_bytes gauge
zfs_pool_size_bytes{pool="testpool"} 2048
`,
		},
		{
			name:           `multiple pools`,
			pools:          []string{`testpool1`, `testpool2`},
			propsRequested: []string{`allocated`},
			metricNames:    []string{`zfs_pool_allocated_bytes`},
			propsResults: map[string]map[string]string{
				`testpool1`: {
					`allocated`: `1024`,
				},
				`testpool2`: {
					`allocated`: `2048`,
				},
			},
			metricResults: `# HELP zfs_pool_allocated_bytes Amount of storage in bytes used within the pool.
# TYPE zfs_pool_allocated_bytes gauge
zfs_pool_allocated_bytes{pool="testpool1"} 1024
zfs_pool_allocated_bytes{pool="testpool2"} 2048
`,
		},
		{
			name:           `explicit pools`,
			pools:          []string{`testpool1`, `testpool2`},
			explicitPools:  []string{`testpool1`},
			propsRequested: []string{`allocated`},
			metricNames:    []string{`zfs_pool_allocated_bytes`},
			propsResults: map[string]map[string]string{
				`testpool1`: {
					`allocated`: `1024`,
				},
				`testpool2`: {
					`allocated`: `2048`,
				},
			},
			metricResults: `# HELP zfs_pool_allocated_bytes Amount of storage in bytes used within the pool.
# TYPE zfs_pool_allocated_bytes gauge
zfs_pool_allocated_bytes{pool="testpool1"} 1024
`,
		},
		{
			name:           `health status`,
			pools:          []string{`onlinepool`, `degradedpool`, `faultedpool`, `offlinepool`, `unavailpool`, `removedpool`, `suspendedpool`},
			propsRequested: []string{`health`},
			metricNames:    []string{`zfs_pool_health`},
			propsResults: map[string]map[string]string{
				`onlinepool`: {
					`health`: `ONLINE`,
				},
				`degradedpool`: {
					`health`: `DEGRADED`,
				},
				`faultedpool`: {
					`health`: `FAULTED`,
				},
				`offlinepool`: {
					`health`: `OFFLINE`,
				},
				`unavailpool`: {
					`health`: `UNAVAIL`,
				},
				`removedpool`: {
					`health`: `REMOVED`,
				},
				`suspendedpool`: {
					`health`: `SUSPENDED`,
				},
			},
			metricResults: `# HELP zfs_pool_health Health status code for the pool [0: ONLINE, 1: DEGRADED, 2: FAULTED, 3: OFFLINE, 4: UNAVAIL, 5: REMOVED, 6: SUSPENDED].
# TYPE zfs_pool_health gauge
zfs_pool_health{pool="onlinepool"} 0
zfs_pool_health{pool="degradedpool"} 1
zfs_pool_health{pool="faultedpool"} 2
zfs_pool_health{pool="offlinepool"} 3
zfs_pool_health{pool="unavailpool"} 4
zfs_pool_health{pool="removedpool"} 5
zfs_pool_health{pool="suspendedpool"} 6
`,
		},
		{
			name:           `unsupported metric`,
			pools:          []string{`testpool`},
			propsRequested: []string{`unsupported`},
			metricNames:    []string{`zfs_pool_unsupported`},
			propsResults: map[string]map[string]string{
				`testpool`: {
					`unsupported`: `1024`,
				},
			},
			metricResults: `# HELP zfs_pool_unsupported !!! This property is unsupported, results are likely to be undesirable, please file an issue at https://github.com/pdf/zfs_exporter/issues to have this property supported !!!
# TYPE zfs_pool_unsupported gauge
zfs_pool_unsupported{pool="testpool"} 1024
`,
		},
		{
			name:           `legacy fragmentation/dedupratio`,
			pools:          []string{`testpool`},
			propsRequested: []string{`fragmentation`, `dedupratio`},
			metricNames:    []string{`zfs_pool_fragmentation_ratio`, `zfs_pool_deduplication_ratio`},
			propsResults: map[string]map[string]string{
				`testpool`: {
					`fragmentation`: `5%`,
					`dedupratio`:    `2.50x`,
				},
			},
			metricResults: `# HELP zfs_pool_fragmentation_ratio The fragmentation ratio of the pool.
# TYPE zfs_pool_fragmentation_ratio gauge
zfs_pool_fragmentation_ratio{pool="testpool"} 0.05
# HELP zfs_pool_deduplication_ratio The ratio of deduplicated size vs undeduplicated size for data in this pool.
# TYPE zfs_pool_deduplication_ratio gauge
zfs_pool_deduplication_ratio{pool="testpool"} 0.4
`,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl, ctx := gomock.WithContext(context.Background(), t)
			zfsClient := mock_zfs.NewMockClient(ctrl)
			config := defaultConfig(zfsClient)
			if tc.explicitPools != nil {
				config.Pools = tc.explicitPools
			}

			zfsClient.EXPECT().PoolNames().Return(tc.pools, nil).Times(1)
			for _, pool := range tc.pools {
				if tc.explicitPools != nil {
					wanted := false
					for _, explicit := range tc.explicitPools {
						if pool == explicit {
							wanted = true
						}
						break
					}
					if !wanted {
						continue
					}
				}
				zfsPoolProperties := mock_zfs.NewMockPoolProperties(ctrl)
				zfsPoolProperties.EXPECT().Properties().Return(tc.propsResults[pool]).Times(1)
				zfsPool := mock_zfs.NewMockPool(ctrl)
				zfsPool.EXPECT().Properties(tc.propsRequested).Return(zfsPoolProperties, nil).Times(1)
				zfsClient.EXPECT().Pool(pool).Return(zfsPool).Times(1)
			}

			collector, err := NewZFS(config)
			if err != nil {
				t.Fatal(err)
			}
			collector.Collectors = map[string]State{
				`pool`: {
					Name:       "pool",
					Enabled:    boolPointer(true),
					Properties: stringPointer(strings.Join(tc.propsRequested, `,`)),
					factory:    newPoolCollector,
				},
			}

			if err = callCollector(ctx, collector, []byte(tc.metricResults), tc.metricNames); err != nil {
				t.Fatal(err)
			}
		})
	}
}
