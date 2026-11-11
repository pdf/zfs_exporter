package collector

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/pdf/zfs_exporter/v2/zfs"
	"github.com/pdf/zfs_exporter/v2/zfs/mock_zfs"
	"github.com/prometheus/client_golang/prometheus"
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

// stubCollector sends its metrics, then blocks until release is closed.
type stubCollector struct {
	metrics []metric
	release chan struct{}
}

func (s stubCollector) update(ch chan<- metric, _ []string, _ regexpCollection) error {
	for _, m := range s.metrics {
		ch <- m
	}
	if s.release != nil {
		<-s.release
	}
	return nil
}

func (s stubCollector) describe(_ chan<- *prometheus.Desc) {}

func stubState(name string, c Collector) State {
	return State{
		Name:       name,
		Enabled:    boolPointer(true),
		Properties: stringPointer(``),
		factory: func(_ *slog.Logger, _ zfs.Client, _ []string) (Collector, error) {
			return c, nil
		},
	}
}

func TestZFSCollectReturnsAtDeadline(t *testing.T) {
	ctrl := gomock.NewController(t)
	zfsClient := mock_zfs.NewMockClient(ctrl)
	zfsClient.EXPECT().PoolNames().Return([]string{`testpool`}, nil).Times(1)

	config := defaultConfig(zfsClient)
	config.Deadline = 200 * time.Millisecond
	collector, err := NewZFS(config)
	if err != nil {
		t.Fatal(err)
	}

	desc := prometheus.NewDesc(`zfs_test_fast`, `A metric sent before the deadline.`, nil, nil)
	release := make(chan struct{})
	collector.Collectors = map[string]State{
		`fast`: stubState(`fast`, stubCollector{metrics: []metric{{
			name:       `zfs_test_fast`,
			prometheus: prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, 1),
		}}}),
		// The slow collector sends nothing after the deadline, so nothing
		// arrives to unblock Collect but the deadline itself.
		`slow`: stubState(`slow`, stubCollector{release: release}),
	}

	ch := make(chan prometheus.Metric)
	done := make(chan struct{})
	go func() {
		collector.Collect(ch)
		close(done)
	}()

	var received []prometheus.Metric
	timeout := time.After(5 * time.Second)
	for collecting := true; collecting; {
		select {
		case m := <-ch:
			received = append(received, m)
		case <-done:
			collecting = false
		case <-timeout:
			close(release)
			t.Fatal("Collect() did not return after the deadline")
		}
	}

	if len(received) != 1 || received[0].Desc() != desc {
		t.Errorf("received %d metrics, want only zfs_test_fast", len(received))
	}

	// Let the background collection finish, and check it no longer writes to ch.
	close(release)
	select {
	case <-collector.ready:
	case <-time.After(5 * time.Second):
		t.Fatal("background collection did not finish")
	}
	select {
	case m := <-ch:
		t.Errorf("received %v after Collect() returned", m.Desc())
	default:
	}
}
