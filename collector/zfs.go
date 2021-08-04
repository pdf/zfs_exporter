package collector

import (
	"context"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/pdf/zfs_exporter/zfs"
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
	Deadline time.Duration
	Pools    []string
	Excludes []string
	Logger   log.Logger
}

// ZFS collector
type ZFS struct {
	Pools      []string
	Collectors map[string]State
	deadline   time.Duration
	cache      *metricCache
	ready      chan struct{}
	logger     log.Logger
	excludes   regexpCollection
}

// Describe implements the prometheus.Collector interface.
func (c *ZFS) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
}

// Collect implements the prometheus.Collector interface.
func (c *ZFS) Collect(ch chan<- prometheus.Metric) {
	select {
	case <-c.ready:
	default:
		c.sendCached(ch, make(map[string]struct{}))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.deadline)
	defer cancel()

	cache := newMetricCache()
	proxy := make(chan metric)
	// Synchronize on collector completion.
	wg := sync.WaitGroup{}
	wg.Add(len(c.Collectors))
	// Synchonize after timeout event, ensuring no writers are still active when we return control.
	timeout := make(chan struct{})
	done := make(chan struct{})
	timeoutMutex := sync.Mutex{}

	// Upon exceeding deadline, send cached data for any metrics that have not already been reported.
	go func() {
		<-ctx.Done()
		if err := ctx.Err(); err != nil && err != context.Canceled {
			timeoutMutex.Lock()
			c.cache.merge(cache)
			cacheIndex := cache.index()
			c.sendCached(ch, cacheIndex)
			close(timeout) // assert timeout for flow control in other goroutines
			timeoutMutex.Unlock()
		}
	}()

	// Close the proxy channel upon collector completion.
	go func() {
		wg.Wait()
		close(proxy)
	}()

	// Cache metrics as they come in via the proxy channel, and ship them out if we've not exceeded the deadline.
	go func() {
		for metric := range proxy {
			timeoutMutex.Lock()
			cache.add(metric)
			select {
			case <-timeout:
				timeoutMutex.Unlock()
				continue
			default:
				ch <- metric.prometheus
				timeoutMutex.Unlock()
			}
		}
		// Signal completion and update full cache.
		c.cache.replace(cache)
		close(done)
		// Notify next collection that we're ready to collect again
		c.ready <- struct{}{}
	}()

	pools, err := c.getPools(c.Pools)
	if err != nil {
		_ = level.Error(c.logger).Log("msg", "Error finding pools", "err", err)
		return
	}

	for name, state := range c.Collectors {
		if !*state.Enabled {
			wg.Done()
			continue
		}

		collector, err := state.factory()
		if err != nil {
			_ = level.Error(c.logger).Log("Error instantiating collector", "collector", name, "err", err)
			continue
		}
		go func(name string, collector Collector) {
			c.execute(ctx, name, collector, proxy, pools)
			wg.Done()
		}(name, collector)
	}

	// Wait for either timeout or completion.
	select {
	case <-timeout:
	case <-done:
	}
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

func (c *ZFS) getPools(pools []string) ([]*zfs.Zpool, error) {
	// Get all pools if not explicitly configured.
	if len(pools) == 0 {
		zpools, err := zfs.ListZpools()
		if err != nil {
			return nil, err
		}
		return zpools, nil
	}

	// Configured pools may not exist, so append available pools as they're found, rather than allocating up front.
	zpools := make([]*zfs.Zpool, 0)
	for _, name := range pools {
		pool, err := zfs.GetZpool(name)
		if err != nil {
			_ = level.Warn(c.logger).Log("msg", "Pool unavailable", "pool", name)
			continue
		}
		zpools = append(zpools, pool)
	}

	return zpools, nil
}

func (c *ZFS) execute(ctx context.Context, name string, collector Collector, ch chan<- metric, pools []*zfs.Zpool) {
	begin := time.Now()
	err := collector.update(ch, pools, c.excludes)
	duration := time.Since(begin)
	var success float64

	if err != nil {
		_ = level.Error(c.logger).Log("msg", "Executing collector", "status", "error", "collector", name, "durationSeconds", duration.Seconds(), "err", err)
		success = 0
	} else {
		select {
		case <-ctx.Done():
			err = ctx.Err()
		default:
			err = nil
		}
		if err != nil && err != context.Canceled {
			_ = level.Warn(c.logger).Log("msg", "Executing collector", "status", "delayed", "collector", name, "durationSeconds", duration.Seconds(), "err", ctx.Err())
			success = 0
		} else {
			_ = level.Debug(c.logger).Log("msg", "Executing collector", "status", "ok", "collector", name, "durationSeconds", duration.Seconds())
			success = 1
		}
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
		deadline:   config.Deadline,
		Pools:      config.Pools,
		Collectors: collectorStates,
		excludes:   excludes,
		cache:      newMetricCache(),
		ready:      ready,
		logger:     config.Logger,
	}, nil
}
