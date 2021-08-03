package collector

import (
	"fmt"

	"github.com/pdf/zfs_exporter/zfs"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector(`pool`, defaultEnabled, newPoolCollector)
}

type healthCode int

const (
	online healthCode = iota
	degraded
	faulted
	offline
	unavail
	removed
)

type poolCollector struct {
	health               desc
	allocatedBytes       desc
	sizeBytes            desc
	freeBytes            desc
	fragmentationPercent desc
	readOnly             desc
	freeingBytes         desc
	leakedBytes          desc
	dedupRatio           desc
}

func (c *poolCollector) update(ch chan<- metric, pools []*zfs.Zpool, excludes regexpCollection) error {
	for _, pool := range pools {
		if err := c.updatePoolMetrics(ch, pool); err != nil {
			return err
		}
	}

	return nil
}

func (c *poolCollector) updatePoolMetrics(ch chan<- metric, pool *zfs.Zpool) error {
	health, err := healthCodeFromString(pool.Health)
	if err != nil {
		return err
	}

	var readOnly float64
	if pool.ReadOnly {
		readOnly = 1
	}

	labels := []string{pool.Name}

	ch <- metric{
		name: expandMetricName(c.health.name, labels...),
		prometheus: prometheus.MustNewConstMetric(
			c.health.prometheus,
			prometheus.GaugeValue,
			float64(health),
			labels...,
		),
	}

	ch <- metric{
		name: expandMetricName(c.allocatedBytes.name, labels...),
		prometheus: prometheus.MustNewConstMetric(
			c.allocatedBytes.prometheus,
			prometheus.GaugeValue,
			float64(pool.Allocated),
			labels...,
		),
	}

	ch <- metric{
		name: expandMetricName(c.sizeBytes.name, labels...),
		prometheus: prometheus.MustNewConstMetric(
			c.sizeBytes.prometheus,
			prometheus.GaugeValue,
			float64(pool.Size),
			labels...,
		),
	}

	ch <- metric{
		name: expandMetricName(c.freeBytes.name, labels...),
		prometheus: prometheus.MustNewConstMetric(
			c.freeBytes.prometheus,
			prometheus.GaugeValue,
			float64(pool.Free),
			labels...,
		),
	}

	ch <- metric{
		name: expandMetricName(c.fragmentationPercent.name, labels...),
		prometheus: prometheus.MustNewConstMetric(
			c.fragmentationPercent.prometheus,
			prometheus.GaugeValue,
			float64(pool.Fragmentation),
			labels...,
		),
	}

	ch <- metric{
		name: expandMetricName(c.readOnly.name, labels...),
		prometheus: prometheus.MustNewConstMetric(
			c.readOnly.prometheus,
			prometheus.GaugeValue,
			readOnly,
			labels...,
		),
	}

	ch <- metric{
		name: expandMetricName(c.freeingBytes.name, labels...),
		prometheus: prometheus.MustNewConstMetric(
			c.freeingBytes.prometheus,
			prometheus.GaugeValue,
			float64(pool.Freeing),
			labels...,
		),
	}

	ch <- metric{
		name: expandMetricName(c.leakedBytes.name, labels...),
		prometheus: prometheus.MustNewConstMetric(
			c.leakedBytes.prometheus,
			prometheus.GaugeValue,
			float64(pool.Leaked),
			labels...,
		),
	}

	ch <- metric{
		name: expandMetricName(c.dedupRatio.name, labels...),
		prometheus: prometheus.MustNewConstMetric(
			c.dedupRatio.prometheus,
			prometheus.GaugeValue,
			pool.DedupRatio,
			labels...,
		),
	}

	return nil
}

func newPoolCollector() (Collector, error) {
	const subsystem = `pool`
	var (
		labels                   = []string{`pool`}
		healthName               = prometheus.BuildFQName(namespace, subsystem, `health`)
		allocatedBytesName       = prometheus.BuildFQName(namespace, subsystem, `allocated_bytes`)
		sizeBytesName            = prometheus.BuildFQName(namespace, subsystem, `size_bytes`)
		freeBytesName            = prometheus.BuildFQName(namespace, subsystem, `free_bytes`)
		fragmentationPercentName = prometheus.BuildFQName(namespace, subsystem, `fragmentation_percent`)
		readOnlyName             = prometheus.BuildFQName(namespace, subsystem, `readonly`)
		freeingBytesName         = prometheus.BuildFQName(namespace, subsystem, `freeing_bytes`)
		leakedBytesName          = prometheus.BuildFQName(namespace, subsystem, `leaked_bytes`)
		dedupRatioName           = prometheus.BuildFQName(namespace, subsystem, `deduplication_ratio`)
	)

	return &poolCollector{
		health: desc{
			name: healthName,
			prometheus: prometheus.NewDesc(
				healthName,
				fmt.Sprintf("Health status code for the pool [%d: %s, %d: %s, %d: %s, %d: %s, %d: %s, %d: %s].",
					online, zfs.ZpoolOnline, degraded, zfs.ZpoolDegraded, faulted, zfs.ZpoolFaulted, offline, zfs.ZpoolOffline, unavail, zfs.ZpoolUnavail, removed, zfs.ZpoolRemoved),
				labels,
				nil,
			),
		},
		allocatedBytes: desc{
			name: allocatedBytesName,
			prometheus: prometheus.NewDesc(
				allocatedBytesName,
				`Amount of storage space in bytes within the pool that has been physically allocated.`,
				labels,
				nil,
			),
		},
		sizeBytes: desc{
			name: sizeBytesName,
			prometheus: prometheus.NewDesc(
				sizeBytesName,
				`Total size in bytes of the storage pool.`,
				labels,
				nil,
			),
		},
		freeBytes: desc{
			name: freeBytesName,
			prometheus: prometheus.NewDesc(
				freeBytesName,
				`The amount of free space in bytes available in the pool.`,
				labels,
				nil,
			),
		},
		fragmentationPercent: desc{
			name: fragmentationPercentName,
			prometheus: prometheus.NewDesc(
				fragmentationPercentName,
				`Fragmentation percentage of the pool.`,
				labels,
				nil,
			),
		},
		readOnly: desc{
			name: readOnlyName,
			prometheus: prometheus.NewDesc(
				readOnlyName,
				`Read-only status of the pool [0: read-write, 1: read-only].`,
				labels,
				nil,
			),
		},
		freeingBytes: desc{
			name: freeingBytesName,
			prometheus: prometheus.NewDesc(
				freeingBytesName,
				`The amount of space in bytes remaining to be freed following the desctruction of a file system or snapshot.`,
				labels,
				nil,
			),
		},
		leakedBytes: desc{
			name: leakedBytesName,
			prometheus: prometheus.NewDesc(
				leakedBytesName,
				`Number of leaked bytes in the pool.`,
				labels,
				nil,
			),
		},
		dedupRatio: desc{
			name: dedupRatioName,
			prometheus: prometheus.NewDesc(
				dedupRatioName,
				`The deduplication ratio specified for the pool, expressed as a multiplier.`,
				labels,
				nil,
			),
		},
	}, nil
}

func healthCodeFromString(status string) (healthCode, error) {
	switch status {
	case zfs.ZpoolOnline:
		return online, nil
	case zfs.ZpoolDegraded:
		return degraded, nil
	case zfs.ZpoolFaulted:
		return faulted, nil
	case zfs.ZpoolOffline:
		return offline, nil
	case zfs.ZpoolUnavail:
		return unavail, nil
	case zfs.ZpoolRemoved:
		return removed, nil
	}

	return -1, fmt.Errorf(`unknown pool heath status: %s`, status)
}
