package collector

import (
	"context"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pdf/zfs_exporter/v2/zfs"
	"github.com/prometheus/client_golang/prometheus"
)

type regexpCollection []*regexp.Regexp

func (c regexpCollection) MatchString(input string) bool {
	for _, r := range c {
		if r.MatchString(input) {
			return true
		}
	}

	return false
}

// ZFSConfig configures a ZFS collector
type ZFSConfig struct {
	DisableMetrics bool
	Deadline       time.Duration
	Pools          []string
	Excludes       []string
	Logger         *slog.Logger
	ZFSClient      zfs.Client
}

// ZFS collector
type ZFS struct {
	Pools          []string
	Collectors     map[string]State
	client         zfs.Client
	disableMetrics bool
	deadline       time.Duration
	cache          *metricCache
	ready          chan struct{}
	logger         *slog.Logger
	excludes       regexpCollection
}

// Describe implements the prometheus.Collector interface.
func (c *ZFS) Describe(ch chan<- *prometheus.Desc) {
	if !c.disableMetrics {
		ch <- scrapeDurationDesc
		ch <- scrapeSuccessDesc
	}

	for _, state := range c.Collectors {
		if !*state.Enabled {
			continue
		}

		collector, err := state.factory(c.logger, c.client, strings.Split(*state.Properties, `,`))
		if err != nil {
			continue
		}
		collector.describe(ch)
	}
}

// Collect implements the prometheus.Collector interface.
//
// Only the calling goroutine writes to ch, so nothing is written to it after Collect returns. If the collection
// exceeds the deadline, the cache is sent for any metrics not yet collected, and the collection continues in the
// background to update the cache. Collections never run concurrently: while one is running, the cache is sent.
func (c *ZFS) Collect(ch chan<- prometheus.Metric) {
	select {
	case <-c.ready:
	default:
		c.sendCached(ch, make(map[string]struct{}))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.deadline)
	defer cancel()

	proxy := make(chan metric)
	go func() {
		defer close(proxy)
		c.run(ctx, proxy)
	}()

	cache := newMetricCache()
	finish := func() {
		c.cache.replace(cache)
		c.ready <- struct{}{}
	}

	// forward sends a metric from proxy, or finishes the collection when proxy is closed and reports true.
	forward := func(m metric, ok bool) bool {
		if !ok {
			finish()
			return true
		}
		cache.add(m)
		ch <- m.prometheus
		return false
	}

	for {
		select {
		case metric, ok := <-proxy:
			if forward(metric, ok) {
				return
			}
		case <-ctx.Done():
			// The collection may have completed at the same moment as the deadline, so check proxy once before
			// falling back to the cache. If so, return the fresh data instead.
			select {
			case metric, ok := <-proxy:
				if forward(metric, ok) {
					return
				}
			default:
			}

			// Send cached data for any metrics that have not already been reported, then hand the rest of the
			// collection off to the background.
			c.cache.merge(cache)
			c.sendCached(ch, cache.index())
			go func() {
				for metric := range proxy {
					cache.add(metric)
				}
				finish()
			}()

			return
		}
	}
}

// run executes the enabled collectors, sending their metrics to ch, and returns once they have all completed.
func (c *ZFS) run(ctx context.Context, ch chan<- metric) {
	pools, poolErr := c.getPools(c.Pools)

	var wg sync.WaitGroup
	for name, state := range c.Collectors {
		if !*state.Enabled {
			continue
		}

		if poolErr != nil {
			c.publishCollectorMetrics(ctx, name, poolErr, 0, ch)
			continue
		}

		collector, err := state.factory(c.logger, c.client, strings.Split(*state.Properties, `,`))
		if err != nil {
			c.logger.Error("Error instantiating collector", "collector", name, "err", err)
			continue
		}

		wg.Go(func() {
			c.execute(ctx, name, collector, ch, pools)
		})
	}
	wg.Wait()
}

// sendCached values that do not appear in the current cacheIndex.
func (c *ZFS) sendCached(ch chan<- prometheus.Metric, cacheIndex map[string]struct{}) {
	c.cache.RLock()
	defer c.cache.RUnlock()
	for name, metric := range c.cache.cache {
		if _, ok := cacheIndex[name]; ok {
			continue
		}
		ch <- metric
	}
}

func (c *ZFS) getPools(pools []string) ([]string, error) {
	poolNames, err := c.client.PoolNames()
	if err != nil {
		return nil, err
	}
	// Return all pools if not explicitly configured.
	if len(pools) == 0 {
		return poolNames, nil
	}

	// Configured pools may not exist, so append available pools as they're found, rather than allocating up front.
	result := make([]string, 0)
	for _, want := range pools {
		found := false
		for _, avail := range poolNames {
			if want == avail {
				result = append(result, want)
				found = true
				break
			}
		}
		if !found {
			c.logger.Warn("Pool unavailable", "pool", want)
		}
	}

	return result, nil
}

func (c *ZFS) execute(ctx context.Context, name string, collector Collector, ch chan<- metric, pools []string) {
	begin := time.Now()
	err := collector.update(ch, pools, c.excludes)
	duration := time.Since(begin)

	c.publishCollectorMetrics(ctx, name, err, duration, ch)
}

func (c *ZFS) publishCollectorMetrics(ctx context.Context, name string, err error, duration time.Duration, ch chan<- metric) {
	var success float64

	if err != nil {
		c.logger.Error("Executing collector", "status", "error", "collector", name, "durationSeconds", duration.Seconds(), "err", err)
		success = 0
	} else {
		select {
		case <-ctx.Done():
			err = ctx.Err()
		default:
			err = nil
		}
		if err != nil && err != context.Canceled {
			c.logger.Warn("Executing collector", "status", "delayed", "collector", name, "durationSeconds", duration.Seconds(), "err", ctx.Err())
			success = 0
		} else {
			c.logger.Debug("Executing collector", "status", "ok", "collector", name, "durationSeconds", duration.Seconds())
			success = 1
		}
	}

	if c.disableMetrics {
		return
	}
	ch <- metric{
		name:       scrapeDurationDescName,
		prometheus: prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, duration.Seconds(), name),
	}
	ch <- metric{
		name:       scrapeSuccessDescName,
		prometheus: prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, success, name),
	}
}

// NewZFS instantiates a ZFS collector with the provided ZFSConfig
func NewZFS(config ZFSConfig) (*ZFS, error) {
	sort.Strings(config.Pools)
	sort.Strings(config.Excludes)
	excludes := make(regexpCollection, len(config.Excludes))
	for i, v := range config.Excludes {
		excludes[i] = regexp.MustCompile(v)
	}
	ready := make(chan struct{}, 1)
	ready <- struct{}{}
	return &ZFS{
		disableMetrics: config.DisableMetrics,
		client:         config.ZFSClient,
		deadline:       config.Deadline,
		Pools:          config.Pools,
		Collectors:     collectorStates,
		excludes:       excludes,
		cache:          newMetricCache(),
		ready:          ready,
		logger:         config.Logger,
	}, nil
}
