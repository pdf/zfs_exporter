package collector

import (
	"context"
	"fmt"
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
	update(ch chan<- metric, pools []*zfs.Zpool) error
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
	Collectors map[string]State
	cache      map[string]prometheus.Metric
	done       chan struct{}
	mu         sync.RWMutex
}

// Describe implements the prometheus.Collector interface.
func (c *ZFSCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
}

// Collect implements the prometheus.Collector interface.
func (c *ZFSCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	select {
	case <-c.done:
		c.mu.RUnlock()
	default:
		c.mu.RUnlock()
		c.sendCached(ch, make(map[string]struct{}))
		return
	}
	c.mu.Lock()
	c.done = make(chan struct{})
	c.mu.Unlock()
	mu := sync.Mutex{}
	ctx, cancel := context.WithTimeout(context.Background(), c.Deadline)
	defer cancel()

	proxy := make(chan metric)
	cache := make(map[string]prometheus.Metric)
	timeout := make(chan struct{})
	wg := sync.WaitGroup{}
	wg.Add(len(c.Collectors))

	// Upon exceeding deadline, send cached data for any metrics that have not already been reported.
	go func() {
		select {
		case <-ctx.Done():
			mu.Lock()
			cacheIndex := make(map[string]struct{}, len(cache))
			c.mu.Lock()
			for name, metric := range cache {
				c.cache[name] = metric
				cacheIndex[name] = struct{}{}
			}
			c.mu.Unlock()
			c.sendCached(ch, cacheIndex)
			close(timeout) // assert timeout for flow control in other goroutines
			mu.Unlock()
		case <-c.done:
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
			mu.Lock()
			cache[metric.name] = metric.prometheus
			select {
			case <-timeout:
				mu.Unlock()
				continue
			default:
				ch <- metric.prometheus
				mu.Unlock()
			}
		}
		// Signal completion.
		c.mu.Lock()
		c.cache = cache
		close(c.done)
		c.mu.Unlock()
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
			execute(ctx, name, collector, proxy, pools)
			wg.Done()
		}(name, collector)
	}

	// Wait for either timeout or completion.
	select {
	case <-timeout:
	case <-c.done:
	}
}

// sendCached values that do not appear in the current cacheIndex.
func (c *ZFSCollector) sendCached(ch chan<- prometheus.Metric, cacheIndex map[string]struct{}) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for name, metric := range c.cache {
		if _, ok := cacheIndex[name]; ok {
			continue
		}
		ch <- metric
	}
}

func NewZFSCollector(deadline time.Duration, pools []string) (*ZFSCollector, error) {
	sort.Strings(pools)
	done := make(chan struct{})
	close(done)
	return &ZFSCollector{
		Deadline:   deadline,
		Pools:      pools,
		Collectors: collectorStates,
		cache:      make(map[string]prometheus.Metric),
		done:       done,
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

func execute(ctx context.Context, name string, collector Collector, ch chan<- metric, pools []*zfs.Zpool) {
	begin := time.Now()
	err := collector.update(ch, pools)
	duration := time.Since(begin)
	var success float64

	if err != nil {
		log.Errorf("ERROR: %s collector failed after %fs: %s", name, duration.Seconds(), err)
		success = 0
	} else {
		select {
		case <-ctx.Done():
			log.Warnf("DELAYED: %s collector completed after %fs: %s", name, duration.Seconds(), ctx.Err())
			success = 0
		default:
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
