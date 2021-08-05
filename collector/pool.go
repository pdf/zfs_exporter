package collector

import (
	"fmt"

	"github.com/mistifyio/go-zfs"
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
	allocatedBytes       desc
	dedupRatio           desc
	fragmentationPercent desc
	freeBytes            desc
	freeingBytes         desc
	health               desc
	leakedBytes          desc
	readOnly             desc
	sizeBytes            desc
}

const poolSubsystem = `pool`

var poolLabels = []string{`pool`}

func (c *poolCollector) update(ch chan<- metric, pools []*zfs.Zpool, excludes regexpCollection) error {
	for _, pool := range pools {
		if err := c.updatePoolMetrics(ch, pool); err != nil {
			return err
		}
	}

	return nil
}

func (c *poolCollector) updatePoolMetrics(ch chan<- metric, pool *zfs.Zpool) error {
	// match with poolLabels
	labelValues := []string{pool.Name}

	health, err := healthCodeFromString(pool.Health)
	if err != nil {
		return err
	}

	var readOnly float64
	if pool.ReadOnly {
		readOnly = 1
	}

	ch <- newGaugeMetric(
		c.allocatedBytes,
		float64(pool.Allocated),
		labelValues,
	)

	ch <- newGaugeMetric(
		c.dedupRatio,
		pool.DedupRatio,
		labelValues,
	)

	ch <- newGaugeMetric(
		c.fragmentationPercent,
		float64(pool.Fragmentation),
		labelValues,
	)

	ch <- newGaugeMetric(
		c.freeBytes,
		float64(pool.Free),
		labelValues,
	)

	ch <- newGaugeMetric(
		c.freeingBytes,
		float64(pool.Freeing),
		labelValues,
	)

	ch <- newGaugeMetric(
		c.health,
		float64(health),
		labelValues,
	)

	ch <- newGaugeMetric(
		c.leakedBytes,
		float64(pool.Leaked),
		labelValues,
	)

	ch <- newGaugeMetric(
		c.readOnly,
		readOnly,
		labelValues,
	)

	ch <- newGaugeMetric(
		c.sizeBytes,
		float64(pool.Size),
		labelValues,
	)

	return nil
}

func newPoolCollector() (Collector, error) {
	return &poolCollector{
		allocatedBytes: newDesc(
			poolSubsystem,
			`allocated_bytes`,
			`Amount of storage space in bytes within the pool that has been physically allocated.`,
			poolLabels,
		),

		dedupRatio: newDesc(
			poolSubsystem,
			`deduplication_ratio`,
			`The deduplication ratio specified for the pool, expressed as a multiplier.`,
			poolLabels,
		),

		fragmentationPercent: newDesc(
			poolSubsystem,
			`fragmentation_percent`,
			`Fragmentation percentage of the pool.`,
			poolLabels,
		),

		freeBytes: newDesc(
			poolSubsystem,
			`free_bytes`,
			`The amount of free space in bytes available in the pool.`,
			poolLabels,
		),

		freeingBytes: newDesc(
			poolSubsystem,
			`freeing_bytes`,
			`The amount of space in bytes remaining to be freed following the desctruction of a file system or snapshot.`,
			poolLabels,
		),

		health: newDesc(
			poolSubsystem,
			`health`,
			fmt.Sprintf("Health status code for the pool [%d: %s, %d: %s, %d: %s, %d: %s, %d: %s, %d: %s].",
				online, zfs.ZpoolOnline, degraded, zfs.ZpoolDegraded, faulted, zfs.ZpoolFaulted, offline, zfs.ZpoolOffline, unavail, zfs.ZpoolUnavail, removed, zfs.ZpoolRemoved),
			poolLabels,
		),

		leakedBytes: newDesc(
			poolSubsystem,
			`leaked_bytes`,
			`Number of leaked bytes in the pool.`,
			poolLabels,
		),

		readOnly: newDesc(
			poolSubsystem,
			`readonly`,
			`Read-only status of the pool [0: read-write, 1: read-only].`,
			poolLabels,
		),

		sizeBytes: newDesc(
			poolSubsystem,
			`size_bytes`,
			`Total size in bytes of the storage pool.`,
			poolLabels,
		),
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
