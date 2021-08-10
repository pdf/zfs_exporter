package collector

import (
	"fmt"

	"github.com/pdf/zfs_exporter/basic_zfs"
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

var poolLabels = []string{`pool`}

var poolProperties = []string{
	"name", // NOTE: name should be first!
	"allocated",
	"dedupratio",
	"fragmentation",
	"free",
	"freeing",
	"health",
	"leaked",
	"readonly",
	"size",
}

const poolSubsystem = `pool`

func (c *poolCollector) update(ch chan<- metric, pools []string, excludes regexpCollection) error {
	poolsWithProperties, err := basic_zfs.PoolProperties(pools, poolProperties)
	if err != nil {
		return err
	}
	for _, poolProps := range poolsWithProperties {
		if err := c.updatePoolMetrics(ch, poolProps); err != nil {
			return err
		}
	}

	return nil
}

func (c *poolCollector) updatePoolMetrics(ch chan<- metric, poolProps []string) error {
	// match with poolLabels
	labelValues := []string{poolProps[0]}

	ch <- newGaugeMetric(
		c.allocatedBytes,
		float64FromNumProp(poolProps[1]),
		labelValues,
	)

	ch <- newGaugeMetric(
		c.dedupRatio,
		float64FromNumProp(poolProps[2]),
		labelValues,
	)

	ch <- newGaugeMetric(
		c.fragmentationPercent,
		float64FromNumProp(poolProps[3]),
		labelValues,
	)

	ch <- newGaugeMetric(
		c.freeBytes,
		float64FromNumProp(poolProps[4]),
		labelValues,
	)

	ch <- newGaugeMetric(
		c.freeingBytes,
		float64FromNumProp(poolProps[5]),
		labelValues,
	)

	health, err := healthCodeFromString(poolProps[6])
	if err != nil {
		return err
	}
	ch <- newGaugeMetric(
		c.health,
		float64(health),
		labelValues,
	)

	ch <- newGaugeMetric(
		c.leakedBytes,
		float64FromNumProp(poolProps[7]),
		labelValues,
	)

	readOnly, err := float64FromBoolProp(poolProps[8])
	if err != nil {
		return err
	}
	ch <- newGaugeMetric(
		c.readOnly,
		readOnly,
		labelValues,
	)

	ch <- newGaugeMetric(
		c.sizeBytes,
		float64FromNumProp(poolProps[9]),
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
				online, basic_zfs.ZpoolOnline,
				degraded, basic_zfs.ZpoolDegraded,
				faulted, basic_zfs.ZpoolFaulted,
				offline, basic_zfs.ZpoolOffline,
				unavail, basic_zfs.ZpoolUnavail,
				removed, basic_zfs.ZpoolRemoved,
			),
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
