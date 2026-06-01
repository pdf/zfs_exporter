package collector

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pdf/zfs_exporter/v2/zfs"
	"github.com/pdf/zfs_exporter/v2/zfs/mock_zfs"
	"go.uber.org/mock/gomock"
)

func TestScrubMetrics(t *testing.T) {
	completed := time.Date(2024, 5, 12, 2, 42, 37, 0, time.Local)
	completedUnix := completed.Unix()

	testCases := []struct {
		name           string
		pools          []string
		propsRequested []string
		metricNames    []string
		scanResults    map[string]zfs.PoolScan
		metricResults  string
	}{
		{
			name:           `none requested`,
			pools:          []string{`testpool`},
			propsRequested: []string{`state`, `errors`, `repaired_bytes`, `last_completed`},
			metricNames:    []string{`zfs_pool_scrub_state`, `zfs_pool_scrub_errors`, `zfs_pool_scrub_repaired_bytes`, `zfs_pool_scrub_last_completed_timestamp_seconds`},
			scanResults: map[string]zfs.PoolScan{
				`testpool`: {Function: zfs.ScanFunctionNone, State: zfs.ScanStateNone},
			},
			metricResults: `# HELP zfs_pool_scrub_errors Number of errors detected during the most recent scrub or resilver.
# TYPE zfs_pool_scrub_errors gauge
zfs_pool_scrub_errors{pool="testpool"} 0
# HELP zfs_pool_scrub_last_completed_timestamp_seconds Unix timestamp of the most recently completed scrub or resilver, or 0 if none.
# TYPE zfs_pool_scrub_last_completed_timestamp_seconds gauge
zfs_pool_scrub_last_completed_timestamp_seconds{pool="testpool"} 0
# HELP zfs_pool_scrub_repaired_bytes Bytes repaired during the most recent scrub or resilver.
# TYPE zfs_pool_scrub_repaired_bytes gauge
zfs_pool_scrub_repaired_bytes{pool="testpool"} 0
# HELP zfs_pool_scrub_state State code for the most recent scrub or resilver [0: none, 1: scrub_in_progress, 2: scrub_finished, 3: scrub_canceled, 4: scrub_paused, 5: resilver_in_progress, 6: resilver_finished].
# TYPE zfs_pool_scrub_state gauge
zfs_pool_scrub_state{pool="testpool"} 0
`,
		},
		{
			name:           `scrub finished`,
			pools:          []string{`testpool`},
			propsRequested: []string{`state`, `errors`, `repaired_bytes`, `last_completed`},
			metricNames:    []string{`zfs_pool_scrub_state`, `zfs_pool_scrub_errors`, `zfs_pool_scrub_repaired_bytes`, `zfs_pool_scrub_last_completed_timestamp_seconds`},
			scanResults: map[string]zfs.PoolScan{
				`testpool`: {
					Function:    zfs.ScanFunctionScrub,
					State:       zfs.ScanStateFinished,
					Errors:      3,
					Repaired:    4096,
					CompletedAt: completed,
				},
			},
			metricResults: completedScrubMetricResults(`testpool`, 2, 3, 4096, completedUnix),
		},
		{
			name:           `scrub in progress`,
			pools:          []string{`testpool`},
			propsRequested: []string{`state`},
			metricNames:    []string{`zfs_pool_scrub_state`},
			scanResults: map[string]zfs.PoolScan{
				`testpool`: {Function: zfs.ScanFunctionScrub, State: zfs.ScanStateInProgress},
			},
			metricResults: `# HELP zfs_pool_scrub_state State code for the most recent scrub or resilver [0: none, 1: scrub_in_progress, 2: scrub_finished, 3: scrub_canceled, 4: scrub_paused, 5: resilver_in_progress, 6: resilver_finished].
# TYPE zfs_pool_scrub_state gauge
zfs_pool_scrub_state{pool="testpool"} 1
`,
		},
		{
			name:           `resilver in progress`,
			pools:          []string{`testpool`},
			propsRequested: []string{`state`},
			metricNames:    []string{`zfs_pool_scrub_state`},
			scanResults: map[string]zfs.PoolScan{
				`testpool`: {Function: zfs.ScanFunctionResilver, State: zfs.ScanStateInProgress},
			},
			metricResults: `# HELP zfs_pool_scrub_state State code for the most recent scrub or resilver [0: none, 1: scrub_in_progress, 2: scrub_finished, 3: scrub_canceled, 4: scrub_paused, 5: resilver_in_progress, 6: resilver_finished].
# TYPE zfs_pool_scrub_state gauge
zfs_pool_scrub_state{pool="testpool"} 5
`,
		},
		{
			name:           `multiple pools`,
			pools:          []string{`testpool1`, `testpool2`},
			propsRequested: []string{`state`},
			metricNames:    []string{`zfs_pool_scrub_state`},
			scanResults: map[string]zfs.PoolScan{
				`testpool1`: {Function: zfs.ScanFunctionScrub, State: zfs.ScanStateInProgress},
				`testpool2`: {Function: zfs.ScanFunctionScrub, State: zfs.ScanStatePaused},
			},
			metricResults: `# HELP zfs_pool_scrub_state State code for the most recent scrub or resilver [0: none, 1: scrub_in_progress, 2: scrub_finished, 3: scrub_canceled, 4: scrub_paused, 5: resilver_in_progress, 6: resilver_finished].
# TYPE zfs_pool_scrub_state gauge
zfs_pool_scrub_state{pool="testpool1"} 1
zfs_pool_scrub_state{pool="testpool2"} 4
`,
		},
		{
			name:           `scrub in progress with metrics`,
			pools:          []string{`testpool`},
			propsRequested: []string{`total_bytes`, `scanned_bytes`, `issued_bytes`, `rate_bytes_per_second`},
			metricNames:    []string{`zfs_pool_scrub_total_bytes`, `zfs_pool_scrub_scanned_bytes`, `zfs_pool_scrub_issued_bytes`, `zfs_pool_scrub_rate_bytes_per_second`},
			scanResults: map[string]zfs.PoolScan{
				`testpool`: {
					Function: zfs.ScanFunctionScrub,
					State:    zfs.ScanStateInProgress,
					Total:    33200000000000,
					Scanned:  11428000000000,
					Issued:   7600000000000,
					Rate:     864000000,
				},
			},
			metricResults: `# HELP zfs_pool_scrub_issued_bytes Bytes verified so far by the in-progress scrub or resilver, 0 if no scan is running.
# TYPE zfs_pool_scrub_issued_bytes gauge
zfs_pool_scrub_issued_bytes{pool="testpool"} 7.6e+12
# HELP zfs_pool_scrub_rate_bytes_per_second Current scan rate of the in-progress scrub or resilver, 0 if no scan is running.
# TYPE zfs_pool_scrub_rate_bytes_per_second gauge
zfs_pool_scrub_rate_bytes_per_second{pool="testpool"} 8.64e+08
# HELP zfs_pool_scrub_scanned_bytes Bytes read so far by the in-progress scrub or resilver, 0 if no scan is running.
# TYPE zfs_pool_scrub_scanned_bytes gauge
zfs_pool_scrub_scanned_bytes{pool="testpool"} 1.1428e+13
# HELP zfs_pool_scrub_total_bytes Total bytes to scan in the in-progress scrub or resilver, 0 if no scan is running.
# TYPE zfs_pool_scrub_total_bytes gauge
zfs_pool_scrub_total_bytes{pool="testpool"} 3.32e+13
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

			zfsClient.EXPECT().PoolNames().Return(tc.pools, nil).Times(1)
			for _, pool := range tc.pools {
				zfsPool := mock_zfs.NewMockPool(ctrl)
				zfsPool.EXPECT().Scan().Return(tc.scanResults[pool], nil).Times(1)
				zfsClient.EXPECT().Pool(pool).Return(zfsPool).Times(1)
			}

			collector, err := NewZFS(config)
			if err != nil {
				t.Fatal(err)
			}
			collector.Collectors = map[string]State{
				`pool-scrub`: {
					Name:       "pool-scrub",
					Enabled:    boolPointer(true),
					Properties: stringPointer(strings.Join(tc.propsRequested, `,`)),
					factory:    newScrubCollector,
				},
			}

			if err = callCollector(ctx, collector, []byte(tc.metricResults), tc.metricNames); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func completedScrubMetricResults(pool string, state, errors, repaired int, completedUnix int64) string {
	return fmt.Sprintf(`# HELP zfs_pool_scrub_errors Number of errors detected during the most recent scrub or resilver.
# TYPE zfs_pool_scrub_errors gauge
zfs_pool_scrub_errors{pool=%q} %d
# HELP zfs_pool_scrub_last_completed_timestamp_seconds Unix timestamp of the most recently completed scrub or resilver, or 0 if none.
# TYPE zfs_pool_scrub_last_completed_timestamp_seconds gauge
zfs_pool_scrub_last_completed_timestamp_seconds{pool=%q} %d
# HELP zfs_pool_scrub_repaired_bytes Bytes repaired during the most recent scrub or resilver.
# TYPE zfs_pool_scrub_repaired_bytes gauge
zfs_pool_scrub_repaired_bytes{pool=%q} %d
# HELP zfs_pool_scrub_state State code for the most recent scrub or resilver [0: none, 1: scrub_in_progress, 2: scrub_finished, 3: scrub_canceled, 4: scrub_paused, 5: resilver_in_progress, 6: resilver_finished].
# TYPE zfs_pool_scrub_state gauge
zfs_pool_scrub_state{pool=%q} %d
`, pool, errors, pool, completedUnix, pool, repaired, pool, state)
}
