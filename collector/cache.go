package collector

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type metricCache struct {
	cache map[string]prometheus.Metric
	sync.RWMutex
}

func (c *metricCache) add(m metric) {
	c.Lock()
	defer c.Unlock()
	c.cache[m.name] = m.prometheus
}

func (c *metricCache) merge(other *metricCache) {
	if c == other {
		return
	}
	c.Lock()
	other.RLock()
	defer func() {
		other.RUnlock()
		c.Unlock()
	}()
	for name, value := range other.cache {
		c.cache[name] = value
	}
}

func (c *metricCache) replace(other *metricCache) {
	c.Lock()
	defer c.Unlock()
	c.cache = other.cache
}

func (c *metricCache) index() map[string]struct{} {
	c.RLock()
	defer c.RUnlock()
	index := make(map[string]struct{}, len(c.cache))
	for name := range c.cache {
		index[name] = struct{}{}
	}

	return index
}

func newMetricCache() *metricCache {
	return &metricCache{cache: make(map[string]prometheus.Metric)}
}
