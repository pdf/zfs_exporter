package collector

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mistifyio/go-zfs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	defaultEnabled           = true
	defaultDisabled          = false
	namespace                = `zfs`
	helpDefaultStateEnabled  = `enabled`
	helpDefaultStateDisabled = `disabled`
)

var (
	collectorStates        = make(map[string]State)
	scrapeDurationDescName = prometheus.BuildFQName(namespace, `scrape`, `collector_duration_seconds`)
	scrapeDurationDesc     = prometheus.NewDesc(
		scrapeDurationDescName,
		`zfs_exporter: Duration of a collector scrape.`,
		[]string{`collector`},
		nil,
	)
	scrapeSuccessDescName = prometheus.BuildFQName(namespace, `scrape`, `collector_success`)
	scrapeSuccessDesc     = prometheus.NewDesc(
		scrapeSuccessDescName,
		`zfs_exporter: Whether a collector succeeded.`,
		[]string{`collector`},
		nil,
	)
)

type factoryFunc func() (Collector, error)

type State struct {
	Enabled *bool
	factory factoryFunc
}

type Collector interface {
	update(ch chan<- metric, pools []*zfs.Zpool, ignore []*regexp.Regexp) error
}

type metric struct {
	name       string
	prometheus prometheus.Metric
}

type desc struct {
	name       string
	prometheus *prometheus.Desc
}

type ZFSCollector struct {
	Deadline   time.Duration
	Pools      []string
	Ignore     []*regexp.Regexp
	Collectors map[string]State
	cache      *metricCache
	ready      chan struct{}
}

// Describe implements the prometheus.Collector interface.
func (c *ZFSCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
}

// Collect implements the prometheus.Collector interface.
func (c *ZFSCollector) Collect(ch chan<- prometheus.Metric) {
	select {
	case <-c.ready:
	default:
		c.sendCached(ch, make(map[string]struct{}))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.Deadline)
	defer cancel()

	ignore := c.Ignore
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
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != nil && err != context.Canceled {
				timeoutMutex.Lock()
				c.cache.merge(cache)
				cacheIndex := cache.index()
				c.sendCached(ch, cacheIndex)
				close(timeout) // assert timeout for flow control in other goroutines
				timeoutMutex.Unlock()
			}
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

	pools, err := getPools(c.Pools)
	if err != nil {
		log.Errorf("Could not find pools: %s", err)
		return
	}

	for name, state := range c.Collectors {
		if !*state.Enabled {
			wg.Done()
			continue
		}

		collector, err := state.factory()
		if err != nil {
			log.Errorf("Could not instantiate collector (%s): %s", name, err)
			continue
		}
		go func(name string, collector Collector) {
			execute(ctx, name, collector, proxy, pools, ignore)
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
func (c *ZFSCollector) sendCached(ch chan<- prometheus.Metric, cacheIndex map[string]struct{}) {
	c.cache.RLock()
	defer c.cache.RUnlock()
	for name, metric := range c.cache.cache {
		if _, ok := cacheIndex[name]; ok {
			continue
		}
		ch <- metric
	}
}

func NewZFSCollector(deadline time.Duration, pools []string, ignore []*regexp.Regexp) (*ZFSCollector, error) {
	sort.Strings(pools)
	ready := make(chan struct{}, 1)
	ready <- struct{}{}
	return &ZFSCollector{
		Deadline:   deadline,
		Pools:      pools,
		Ignore:     ignore,
		Collectors: collectorStates,
		cache:      newMetricCache(),
		ready:      ready,
	}, nil
}

func registerCollector(collector string, isDefaultEnabled bool, factory factoryFunc) {
	helpDefaultState := helpDefaultStateDisabled
	if isDefaultEnabled {
		helpDefaultState = helpDefaultStateEnabled
	}

	flagName := fmt.Sprintf("collector.%s", collector)
	flagHelp := fmt.Sprintf("Enable the %s collector (default: %s)", collector, helpDefaultState)
	defaultValue := fmt.Sprintf("%t", isDefaultEnabled)

	flag := kingpin.Flag(flagName, flagHelp).Default(defaultValue).Bool()

	collectorStates[collector] = State{
		Enabled: flag,
		factory: factory,
	}
}

func getPools(pools []string) ([]*zfs.Zpool, error) {
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
			log.Warnln("Pool unavailable:", name)
			continue
		}
		zpools = append(zpools, pool)
	}

	return zpools, nil
}

func execute(ctx context.Context, name string, collector Collector, ch chan<- metric, pools []*zfs.Zpool, ignore []*regexp.Regexp) {
	begin := time.Now()
	err := collector.update(ch, pools, ignore)
	duration := time.Since(begin)
	var success float64

	if err != nil {
		log.Errorf("ERROR: %s collector failed after %fs: %s", name, duration.Seconds(), err)
		success = 0
	} else {
		select {
		case <-ctx.Done():
			err = ctx.Err()
		default:
			err = nil
		}
		if err != nil && err != context.Canceled {
			log.Warnf("DELAYED: %s collector completed after %fs: %s", name, duration.Seconds(), ctx.Err())
			success = 0
		} else {
			log.Debugf("OK: %s collector succeeded after %fs.", name, duration.Seconds())
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

func expandMetricName(prefix string, context ...string) string {
	return strings.Join(append(context, prefix), `-`)
}
