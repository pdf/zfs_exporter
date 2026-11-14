package collector

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/pdf/zfs_exporter/v2/zfs"
	"github.com/pdf/zfs_exporter/v2/zfs/mock_zfs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/mock/gomock"
)

// testDescs holds one Desc per test metric ID, so metrics can be identified by Desc and read with testutil.ToFloat64.
var testDescs = func() map[string]*prometheus.Desc {
	descs := make(map[string]*prometheus.Desc)
	for _, id := range []string{`fast`, `slow_early`, `slow_late`, `old`} {
		descs[id] = prometheus.NewDesc(`zfs_test_`+id, `Test metric.`, nil, nil)
	}

	return descs
}()

func testMetric(id string, value float64) metric {
	return metric{
		name:       `zfs_test_` + id,
		prometheus: prometheus.MustNewConstMetric(testDescs[id], prometheus.GaugeValue, value),
	}
}

// singleMetric adapts a prometheus.Metric to a prometheus.Collector for testutil.ToFloat64.
type singleMetric struct{ prometheus.Metric }

func (m singleMetric) Describe(ch chan<- *prometheus.Desc) { ch <- m.Desc() }
func (m singleMetric) Collect(ch chan<- prometheus.Metric) { ch <- m.Metric }

// testValues returns the values of test metrics by ID, and the number of other (exporter) metrics.
func testValues(metrics []prometheus.Metric) (map[string]float64, int) {
	values := make(map[string]float64)
	other := 0
	for _, m := range metrics {
		found := false
		for id, desc := range testDescs {
			if m.Desc() == desc {
				values[id] = testutil.ToFloat64(singleMetric{m})
				found = true
				break
			}
		}
		if !found {
			other++
		}
	}

	return values, other
}

func cachedMetrics(c *ZFS) []prometheus.Metric {
	c.cache.RLock()
	defer c.cache.RUnlock()
	metrics := make([]prometheus.Metric, 0, len(c.cache.cache))
	for _, m := range c.cache.cache {
		metrics = append(metrics, m)
	}

	return metrics
}

// stubCollector emits before, waits for release to be closed (if not nil), then emits after.
type stubCollector struct {
	before  []metric
	release <-chan struct{}
	after   []metric
}

func (s stubCollector) update(ch chan<- metric, _ []string, _ regexpCollection) error {
	for _, m := range s.before {
		ch <- m
	}
	if s.release != nil {
		<-s.release
	}
	for _, m := range s.after {
		ch <- m
	}

	return nil
}

func (stubCollector) describe(chan<- *prometheus.Desc) {}

// recorder drains a scrape channel, recording everything written to it, until closed.
type recorder struct {
	ch      chan prometheus.Metric
	stop    chan struct{}
	done    chan struct{}
	mu      sync.Mutex
	metrics []prometheus.Metric
}

func newRecorder() *recorder {
	r := &recorder{
		ch:   make(chan prometheus.Metric),
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go func() {
		defer close(r.done)
		for {
			select {
			case m := <-r.ch:
				r.mu.Lock()
				r.metrics = append(r.metrics, m)
				r.mu.Unlock()
			case <-r.stop:
				return
			}
		}
	}()

	return r
}

// take returns and clears the metrics recorded so far. Call synctest.Wait first so that no receive is half-recorded.
func (r *recorder) take() []prometheus.Metric {
	r.mu.Lock()
	defer r.mu.Unlock()
	metrics := r.metrics
	r.metrics = nil

	return metrics
}

func (r *recorder) close() {
	close(r.stop)
	<-r.done
}

const testDeadline = 10 * time.Second

func newTestZFS(t *testing.T, disableMetrics bool, poolNames func() ([]string, error), stubs map[string]Collector) *ZFS {
	t.Helper()
	ctrl := gomock.NewController(t)
	zfsClient := mock_zfs.NewMockClient(ctrl)
	zfsClient.EXPECT().PoolNames().DoAndReturn(poolNames).AnyTimes()

	config := defaultConfig(zfsClient)
	config.DisableMetrics = disableMetrics
	config.Deadline = testDeadline
	c, err := NewZFS(config)
	if err != nil {
		t.Fatal(err)
	}
	c.Collectors = make(map[string]State, len(stubs))
	for name, stub := range stubs {
		c.Collectors[name] = State{
			Name:       name,
			Enabled:    boolPointer(true),
			Properties: stringPointer(``),
			factory: func(*slog.Logger, zfs.Client, []string) (Collector, error) {
				return stub, nil
			},
		}
	}

	return c
}

// collect runs c.Collect and returns how long it took on the bubble's fake clock. If Collect has not returned well
// after the deadline, it reports an error and returns false; the caller's cleanup must then unblock it.
func collect(t *testing.T, c *ZFS, r *recorder) (time.Duration, bool) {
	t.Helper()
	returned := make(chan struct{})
	start := time.Now()
	go func() {
		defer close(returned)
		c.Collect(r.ch)
	}()
	select {
	case <-returned:
		synctest.Wait()
		return time.Since(start), true
	case <-time.After(3 * testDeadline):
		t.Errorf("Collect did not return within %v of a %v deadline", 3*testDeadline, testDeadline)
		return 0, false
	}
}

// slowFixture returns a collector with a fast collector, and a slow one that emits one metric and then blocks until
// release is called. The cache holds stale values for all of them, as if from an earlier run.
func slowFixture(t *testing.T, disableMetrics bool) (*ZFS, func()) {
	t.Helper()
	release := make(chan struct{})
	c := newTestZFS(t, disableMetrics, func() ([]string, error) {
		return []string{`testpool`}, nil
	}, map[string]Collector{
		`fast`: stubCollector{before: []metric{testMetric(`fast`, 1)}},
		`slow`: stubCollector{
			before:  []metric{testMetric(`slow_early`, 1)},
			release: release,
			after:   []metric{testMetric(`slow_late`, 1)},
		},
	})
	for _, id := range []string{`fast`, `slow_early`, `slow_late`} {
		c.cache.add(testMetric(id, 0))
	}

	return c, sync.OnceFunc(func() { close(release) })
}

func TestZFSCollectInvalidPools(t *testing.T) {
	const result = `# HELP zfs_scrape_collector_duration_seconds zfs_exporter: Duration of a collector scrape.
# TYPE zfs_scrape_collector_duration_seconds gauge
zfs_scrape_collector_duration_seconds{collector="pool"} 0
# HELP zfs_scrape_collector_success zfs_exporter: Whether a collector succeeded.
# TYPE zfs_scrape_collector_success gauge
zfs_scrape_collector_success{collector="pool"} 0
`

	synctest.Test(t, func(t *testing.T) {
		// The pool listing fails before any collector is instantiated, so a stub stands in for the pool collector.
		c := newTestZFS(t, false, func() ([]string, error) {
			return nil, errors.New(`Error returned from PoolNames()`)
		}, map[string]Collector{
			`pool`: stubCollector{},
		})

		if err := testutil.CollectAndCompare(c, strings.NewReader(result), `zfs_scrape_collector_duration_seconds`, `zfs_scrape_collector_success`); err != nil {
			t.Error(err)
		}
		if len(c.ready) != 1 {
			t.Error("ready not signalled after the collection completed")
		}
	})
}

func TestZFSCollectDeadline(t *testing.T) {
	for _, tc := range []struct {
		name           string
		disableMetrics bool
		wantOther      int
	}{
		{name: `exporter metrics disabled`, disableMetrics: true, wantOther: 0},
		// The fast collector's duration and success metrics.
		{name: `exporter metrics enabled`, disableMetrics: false, wantOther: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				r := newRecorder()
				defer r.close()
				c, release := slowFixture(t, tc.disableMetrics)
				defer synctest.Wait()
				defer release()

				elapsed, ok := collect(t, c, r)
				if !ok {
					return
				}
				if elapsed != testDeadline {
					t.Errorf("Collect returned after %v, want %v", elapsed, testDeadline)
				}
				values, other := testValues(r.take())
				// Fresh values for what was collected in time, the cached value for the rest.
				want := map[string]float64{`fast`: 1, `slow_early`: 1, `slow_late`: 0}
				if !maps.Equal(values, want) {
					t.Errorf("Collect sent %v, want %v", values, want)
				}
				if other != tc.wantOther {
					t.Errorf("Collect sent %d exporter metrics, want %d", other, tc.wantOther)
				}
			})
		})
	}
}

func TestZFSCollectNoWritesAfterReturn(t *testing.T) {
	for _, disableMetrics := range []bool{true, false} {
		t.Run(fmt.Sprintf(`disableMetrics=%v`, disableMetrics), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				r := newRecorder()
				defer r.close()
				c, release := slowFixture(t, disableMetrics)
				defer synctest.Wait()
				defer release()

				if _, ok := collect(t, c, r); !ok {
					return
				}
				r.take()

				// Let the background collection finish; it must not write to the returned scrape's channel.
				release()
				synctest.Wait()
				if late := r.take(); len(late) != 0 {
					values, other := testValues(late)
					t.Errorf("%d metrics written after Collect returned: %v and %d exporter metrics", len(late), values, other)
				}
			})
		})
	}
}

func TestZFSCollectBackgroundCompletion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRecorder()
		defer r.close()
		c, release := slowFixture(t, false)
		defer synctest.Wait()
		defer release()

		if _, ok := collect(t, c, r); !ok {
			return
		}
		r.take()
		if len(c.ready) == 1 {
			t.Fatal("ready signalled while the collection is still running")
		}

		// A scrape during the background collection returns the cache straight away, updated with what the
		// current run has collected so far.
		elapsed, ok := collect(t, c, r)
		if !ok {
			return
		}
		if elapsed != 0 {
			t.Errorf("Collect during a background collection took %v, want 0", elapsed)
		}
		values, _ := testValues(r.take())
		want := map[string]float64{`fast`: 1, `slow_early`: 1, `slow_late`: 0}
		if !maps.Equal(values, want) {
			t.Errorf("Collect during a background collection sent %v, want %v", values, want)
		}

		release()
		synctest.Wait()
		if len(c.ready) != 1 {
			t.Fatal("ready not signalled after the background collection completed")
		}
		values, other := testValues(cachedMetrics(c))
		want = map[string]float64{`fast`: 1, `slow_early`: 1, `slow_late`: 1}
		if !maps.Equal(values, want) {
			t.Errorf("cache holds %v after the background collection, want %v", values, want)
		}
		// One duration and one success metric (the cache keys them by name).
		if other != 2 {
			t.Errorf("cache holds %d exporter metrics, want 2", other)
		}

		// The next scrape runs a full collection again.
		elapsed, ok = collect(t, c, r)
		if !ok {
			return
		}
		if elapsed != 0 {
			t.Errorf("Collect after the background collection took %v, want 0", elapsed)
		}
		values, other = testValues(r.take())
		if !maps.Equal(values, want) {
			t.Errorf("Collect after the background collection sent %v, want %v", values, want)
		}
		if other != 4 {
			t.Errorf("Collect after the background collection sent %d exporter metrics, want 4", other)
		}
		if len(c.ready) != 1 {
			t.Error("ready not signalled after a complete collection")
		}
	})
}

func TestZFSCollectSlowPoolError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRecorder()
		defer r.close()
		poolRelease := make(chan struct{})
		release := sync.OnceFunc(func() { close(poolRelease) })
		c := newTestZFS(t, false, func() ([]string, error) {
			<-poolRelease
			return nil, errors.New(`Error returned from PoolNames()`)
		}, map[string]Collector{
			`stub`: stubCollector{before: []metric{testMetric(`fast`, 1)}},
		})
		c.cache.add(testMetric(`old`, 0))
		defer synctest.Wait()
		defer release()

		// Listing pools outlives the deadline: return the cache.
		elapsed, ok := collect(t, c, r)
		if !ok {
			return
		}
		if elapsed != testDeadline {
			t.Errorf("Collect returned after %v, want %v", elapsed, testDeadline)
		}
		values, other := testValues(r.take())
		if want := map[string]float64{`old`: 0}; !maps.Equal(values, want) || other != 0 {
			t.Errorf("Collect sent %v and %d exporter metrics, want %v and none", values, other, want)
		}

		release()
		synctest.Wait()
		if late := r.take(); len(late) != 0 {
			t.Errorf("%d metrics written after Collect returned", len(late))
		}
		if len(c.ready) != 1 {
			t.Fatal("ready not signalled after the background collection completed")
		}
		// The failed run replaces the cache with its failure metrics.
		cached := cachedMetrics(c)
		values, other = testValues(cached)
		if len(values) != 0 || other != 2 {
			t.Errorf("cache holds %v and %d exporter metrics, want only the 2 exporter metrics", values, other)
		}
		for _, m := range cached {
			if m.Desc() == scrapeSuccessDesc {
				if v := testutil.ToFloat64(singleMetric{m}); v != 0 {
					t.Errorf("cached collector success is %v, want 0", v)
				}
			}
		}
	})
}

// TestZFSCollectDeadlineTie covers a collection that completes while Collect is blocked sending to a slow consumer,
// and the deadline passes before the consumer reads. Once unblocked, both the closed proxy channel and the deadline are
// ready; Collect must finish the collection rather than send stale cache entries that this run no longer produces.
//
// With the tie-break this passes on every iteration. Without it, select picks at random, so each iteration fails with
// probability 1/2 and the test misses the bug with probability 2^-64.
func TestZFSCollectDeadlineTie(t *testing.T) {
	for i := range 64 {
		synctest.Test(t, func(t *testing.T) {
			c := newTestZFS(t, true, func() ([]string, error) {
				return []string{`testpool`}, nil
			}, map[string]Collector{
				`fast`: stubCollector{before: []metric{testMetric(`fast`, 1)}},
			})
			c.cache.add(testMetric(`old`, 0))

			ch := make(chan prometheus.Metric)
			returned := make(chan struct{})
			go func() {
				defer close(returned)
				c.Collect(ch)
			}()

			// Collect is now blocked sending the only metric, and the collection has completed.
			time.Sleep(2 * testDeadline)
			var got []prometheus.Metric
			for done := false; !done; {
				select {
				case m := <-ch:
					got = append(got, m)
				case <-returned:
					done = true
				}
			}

			values, _ := testValues(got)
			if want := map[string]float64{`fast`: 1}; !maps.Equal(values, want) {
				t.Errorf("iteration %d: Collect sent %v, want %v", i, values, want)
			}
			if len(c.ready) != 1 {
				t.Error("ready not signalled after the collection completed")
			}
			values, _ = testValues(cachedMetrics(c))
			if want := map[string]float64{`fast`: 1}; !maps.Equal(values, want) {
				t.Errorf("cache holds %v, want %v", values, want)
			}
		})
	}
}
