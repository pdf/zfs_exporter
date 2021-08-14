package collector

import (
	"fmt"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/pdf/zfs_exporter/zfs"
)

const (
	defaultPoolProps = `allocated,dedupratio,fragmentation,free,freeing,health,leaked,readonly,size`
)

var (
	poolLabels     = []string{`pool`}
	poolProperties = propertyStore{
		defaultSubsystem: subsystemPool,
		defaultLabels:    poolLabels,
		store: map[string]property{
			`allocated`: newProperty(
				subsystemPool,
				`allocated_bytes`,
				`Amount of storage in bytes used within the pool.`,
				transformNumeric,
				poolLabels...,
			),
			`dedupratio`: newProperty(
				subsystemPool,
				`deduplication_ratio`,
				`The deduplication ratio specified for the pool, expressed as a multiplier.`,
				transformMultiplier,
				poolLabels...,
			),
			`capacity`: newProperty(
				subsystemPool,
				`capacity_ratio`,
				`Percentage of pool space used.`,
				transformPercentage,
				poolLabels...,
			),
			`expandsize`: newProperty(
				subsystemPool,
				`expand_size_bytes`,
				`Amount of uninitialized space within the pool or device that can be used to increase the total capacity of the pool.`,
				transformNumeric,
				poolLabels...,
			),
			`fragmentation`: newProperty(
				subsystemPool,
				`fragmentation_ratio`,
				`The fragmentation ratio of the pool.`,
				transformPercentage,
				poolLabels...,
			),
			`free`: newProperty(
				subsystemPool,
				`free_bytes`,
				`The amount of free space in bytes available in the pool.`,
				transformNumeric,
				poolLabels...,
			),
			`freeing`: newProperty(
				subsystemPool,
				`freeing_bytes`,
				`The amount of space in bytes remaining to be freed following the desctruction of a file system or snapshot.`,
				transformNumeric,
				poolLabels...,
			),
			`health`: newProperty(
				subsystemPool,
				`health`,
				fmt.Sprintf("Health status code for the pool [%d: %s, %d: %s, %d: %s, %d: %s, %d: %s, %d: %s].",
					poolOnline, zfs.PoolOnline,
					poolDegraded, zfs.PoolDegraded,
					poolFaulted, zfs.PoolFaulted,
					poolOffline, zfs.PoolOffline,
					poolUnavail, zfs.PoolUnavail,
					poolRemoved, zfs.PoolRemoved,
				),
				transformHealthCode,
				poolLabels...,
			),
			`leaked`: newProperty(
				subsystemPool,
				`leaked_bytes`,
				`Number of leaked bytes in the pool.`,
				transformNumeric,
				poolLabels...,
			),
			`readonly`: newProperty(
				subsystemPool,
				`readonly`,
				`Read-only status of the pool [0: read-write, 1: read-only].`,
				transformBool,
				poolLabels...,
			),
			`size`: newProperty(
				subsystemPool,
				`size_bytes`,
				`Total size in bytes of the storage pool.`,
				transformNumeric,
				poolLabels...,
			),
		},
	}
)

func init() {
	registerCollector(`pool`, defaultEnabled, defaultPoolProps, newPoolCollector)
}

type poolCollector struct {
	log   log.Logger
	props []string
}

func (c *poolCollector) update(ch chan<- metric, pools []string, excludes regexpCollection) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(pools))
	for _, pool := range pools {
		wg.Add(1)
		go func(pool string) {
			if err := c.updatePoolMetrics(ch, pool); err != nil {
				errChan <- err
			}
			wg.Done()
		}(pool)
	}
	wg.Wait()

	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

func (c *poolCollector) updatePoolMetrics(ch chan<- metric, pool string) error {
	p, err := zfs.PoolProperties(pool, c.props...)
	if err != nil {
		return err
	}

	labelValues := []string{pool}
	for k, v := range p.Properties {
		prop, err := poolProperties.find(k)
		if err != nil {
			_ = level.Warn(c.log).Log(`msg`, propertyUnsupportedMsg, `help`, helpIssue, `property`, k, `err`, err)
		}
		if err = prop.push(ch, v, labelValues...); err != nil {
			return err
		}
	}

	return nil
}

func newPoolCollector(l log.Logger, props []string) (Collector, error) {
	return &poolCollector{log: l, props: props}, nil
}
