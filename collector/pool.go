package collector

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/pdf/zfs_exporter/v2/zfs"
	"github.com/prometheus/client_golang/prometheus"
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
				`The ratio of deduplicated size vs undeduplicated size for data in this pool.`,
				transformMultiplier,
				poolLabels...,
			),
			`capacity`: newProperty(
				subsystemPool,
				`capacity_ratio`,
				`Ratio of pool space used.`,
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
				`The amount of space in bytes remaining to be freed following the destruction of a file system or snapshot.`,
				transformNumeric,
				poolLabels...,
			),
			`health`: newProperty(
				subsystemPool,
				`health`,
				fmt.Sprintf("Health status code for the pool [%d: %s, %d: %s, %d: %s, %d: %s, %d: %s, %d: %s, %d: %s].",
					poolOnline, zfs.PoolOnline,
					poolDegraded, zfs.PoolDegraded,
					poolFaulted, zfs.PoolFaulted,
					poolOffline, zfs.PoolOffline,
					poolUnavail, zfs.PoolUnavail,
					poolRemoved, zfs.PoolRemoved,
					poolSuspended, zfs.PoolSuspended,
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
	log    *slog.Logger
	client zfs.Client
	props  []string
}

func (c *poolCollector) describe(ch chan<- *prometheus.Desc) {
	for _, k := range c.props {
		prop, err := poolProperties.find(k)
		if err != nil {
			c.log.Warn(propertyUnsupportedMsg, `help`, helpIssue, `collector`, `pool`, `property`, k, `err`, err)
			continue
		}
		ch <- prop.desc
	}
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
	p := c.client.Pool(pool)
	props, err := p.Properties(c.props...)
	if err != nil {
		return err
	}

	labelValues := []string{pool}
	for k, v := range props.Properties() {
		prop, err := poolProperties.find(k)
		if err != nil {
			c.log.Warn(propertyUnsupportedMsg, `help`, helpIssue, `collector`, `pool`, `property`, k, `err`, err)
		}
		if err = prop.push(ch, v, labelValues...); err != nil {
			return err
		}
	}

	return nil
}

func newPoolCollector(l *slog.Logger, c zfs.Client, props []string) (Collector, error) {
	return &poolCollector{log: l, client: c, props: props}, nil
}
