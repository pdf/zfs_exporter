package collector

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/pdf/zfs_exporter/v2/zfs/mock_zfs"
	"go.uber.org/mock/gomock"
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

func TestNormalizeDescendants(t *testing.T) {
	testCases := []struct {
		name    string
		paths   []string
		want    []string
		wantErr bool
	}{
		{name: `none`, want: []string{}},
		{name: `sorted`, paths: []string{`tank/b`, `rpool/a`}, want: []string{`rpool/a`, `tank/b`}},
		{name: `duplicates`, paths: []string{`rpool/a`, `rpool/a`}, want: []string{`rpool/a`}},
		{name: `shallower path wins`, paths: []string{`rpool/a/b`, `rpool/a`, `rpool/a/b/c`}, want: []string{`rpool/a`}},
		{name: `shared prefix is not nesting`, paths: []string{`rpool/a`, `rpool/ab`}, want: []string{`rpool/a`, `rpool/ab`}},
		{name: `pool`, paths: []string{`rpool`, `rpool/a`, `rpool2/a`}, want: []string{`rpool`, `rpool2/a`}},
		{name: `empty`, paths: []string{``}, wantErr: true},
		{name: `snapshot`, paths: []string{`rpool/a@snap`}, wantErr: true},
		{name: `bookmark`, paths: []string{`rpool/a#mark`}, wantErr: true},
		{name: `trailing slash`, paths: []string{`rpool/a/`}, wantErr: true},
		{name: `leading slash`, paths: []string{`/rpool/a`}, wantErr: true},
		{name: `double slash`, paths: []string{`rpool//a`}, wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeDescendants(tc.paths)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("normalizeDescendants(%q) = %q, want an error", tc.paths, got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("normalizeDescendants(%q) = %q, want %q", tc.paths, got, tc.want)
			}
		})
	}
}

func TestNewZFSRejectsInvalidDescendants(t *testing.T) {
	ctrl := gomock.NewController(t)
	config := defaultConfig(mock_zfs.NewMockClient(ctrl))
	config.ExcludeDescendants = []string{`rpool/docker/`}
	if _, err := NewZFS(config); err == nil {
		t.Fatal("NewZFS() succeeded, want an error")
	}
}
