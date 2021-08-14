package collector

import (
	"fmt"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/pdf/zfs_exporter/zfs"
)

const (
	defaultFilesystemProps = `available,logicalused,quota,referenced,used,usedbydataset,written`
	defaultSnapshotProps   = `logicalused,referenced,used,written`
	defaultVolumeProps     = `available,logicalused,referenced,used,usedbydataset,volsize,written`
)

var (
	datasetLabels     = []string{`name`, `pool`, `type`}
	datasetProperties = propertyStore{
		defaultSubsystem: subsystemDataset,
		defaultLabels:    datasetLabels,
		store: map[string]property{
			`available`: newProperty(
				subsystemDataset,
				`available_bytes`,
				`The amount of space in bytes available to the dataset and all its children.`,
				transformNumeric,
				datasetLabels...,
			),
			`compressratio`: newProperty(
				subsystemDataset,
				`compression_ratio`,
				`The ratio of compressed size vs uncompressed size for this dataset.`,
				transformMultiplier,
				datasetLabels...,
			),
			`logicalused`: newProperty(
				subsystemDataset,
				`logical_used_bytes`,
				`The amount of space in bytes that is "logically" consumed by this dataset and all its descendents. See the "used_bytes" property.`,
				transformNumeric,
				datasetLabels...,
			),
			`logicalreferenced`: newProperty(
				subsystemDataset,
				`logical_referenced_bytes`,
				`The amount of space that is "logically" accessible by this dataset. See the "referenced_bytes" property.`,
				transformNumeric,
				datasetLabels...,
			),
			`quota`: newProperty(
				subsystemDataset,
				`quota_bytes`,
				`The maximum amount of space in bytes this dataset and its descendents can consume.`,
				transformNumeric,
				datasetLabels...,
			),
			`refcompressratio`: newProperty(
				subsystemDataset,
				`referenced_compression_ratio`,
				`The ratio of compressed size vs uncompressed size for the referenced space of this dataset. See also the "compression_ratio" property.`,
				transformMultiplier,
				datasetLabels...,
			),
			`referenced`: newProperty(
				subsystemDataset,
				`referenced_bytes`,
				`The amount of data in bytes that is accessible by this dataset, which may or may not be shared with other datasets in the pool.`,
				transformNumeric,
				datasetLabels...,
			),
			`refquota`: newProperty(
				subsystemDataset,
				`referenced_quota_bytes`,
				`The maximum amount of space in bytes this dataset can consume.`,
				transformNumeric,
				datasetLabels...,
			),
			`refreservation`: newProperty(
				subsystemDataset,
				`referenced_reservation_bytes`,
				`The minimum amount of space in bytes guaranteed to this dataset.`,
				transformNumeric,
				datasetLabels...,
			),
			`reservation`: newProperty(
				subsystemDataset,
				`reservation_bytes`,
				`The minimum amount of space in bytes guaranteed to a dataset and its descendants.`,
				transformNumeric,
				datasetLabels...,
			),
			`snapshot_count`: newProperty(
				subsystemDataset,
				`snapshot_count_total`,
				`The total number of snapshots that exist under this location in the dataset tree. This value is only available when a snapshot_limit has been set somewhere in the tree under which the dataset resides.`,
				transformNumeric,
				datasetLabels...,
			),
			`snapshot_limit`: newProperty(
				subsystemDataset,
				`snapshot_limit_total`,
				`The total limit on the number of snapshots that can be created on a dataset and its descendents.`,
				transformNumeric,
				datasetLabels...,
			),
			`used`: newProperty(
				subsystemDataset,
				`used_bytes`,
				`The amount of space in bytes consumed by this dataset and all its descendents.`,
				transformNumeric,
				datasetLabels...,
			),
			`usedbychildren`: newProperty(
				subsystemDataset,
				`used_by_children_bytes`,
				`The amount of space in bytes used by children of this dataset, which would be freed if all the dataset's children were destroyed.`,
				transformNumeric,
				datasetLabels...,
			),
			`usedbydataset`: newProperty(
				subsystemDataset,
				`used_by_dataset_bytes`,
				`The amount of space in bytes used by this dataset itself, which would be freed if the dataset were destroyed.`,
				transformNumeric,
				datasetLabels...,
			),
			`usedbyrefreservation`: newProperty(
				subsystemDataset,
				`used_by_reference_reservation_bytes`,
				`The amount of space in bytes used by a refreservation set on this dataset, which would be freed if the refreservation was removed.`,
				transformNumeric,
				datasetLabels...,
			),
			`usedbysnapshots`: newProperty(
				subsystemDataset,
				`used_by_snapshot_bytes`,
				`The amount of space in bytes consumed by snapshots of this dataset.`,
				transformNumeric,
				datasetLabels...,
			),
			`volsize`: newProperty(
				subsystemDataset,
				`volume_size_bytes`,
				`The logical size in bytes of this volume.`,
				transformNumeric,
				datasetLabels...,
			),
			`written`: newProperty(
				subsystemDataset,
				`written_bytes`,
				`The amount of referenced space in bytes written to this dataset since the previous snapshot.`,
				transformNumeric,
				datasetLabels...,
			),
		},
	}
)

func init() {
	registerCollector(`dataset-filesystem`, defaultEnabled, defaultFilesystemProps, newFilesystemCollector)
	registerCollector(`dataset-snapshot`, defaultDisabled, defaultSnapshotProps, newSnapshotCollector)
	registerCollector(`dataset-volume`, defaultEnabled, defaultVolumeProps, newVolumeCollector)
}

type datasetCollector struct {
	kind  zfs.DatasetKind
	log   log.Logger
	props []string
}

func (c *datasetCollector) update(ch chan<- metric, pools []string, excludes regexpCollection) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(pools))
	for _, pool := range pools {
		wg.Add(1)
		go func(pool string) {
			if err := c.updatePoolMetrics(ch, pool, excludes); err != nil {
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

func (c *datasetCollector) updatePoolMetrics(ch chan<- metric, pool string, excludes regexpCollection) error {
	datasets, err := zfs.DatasetProperties(pool, c.kind, c.props...)
	if err != nil {
		return err
	}

	for _, dataset := range datasets {
		if excludes.MatchString(dataset.Name) {
			continue
		}
		if err = c.updateDatasetMetrics(ch, pool, dataset); err != nil {
			return err
		}
	}

	return nil
}

func (c *datasetCollector) updateDatasetMetrics(ch chan<- metric, pool string, dataset zfs.Dataset) error {
	labelValues := []string{dataset.Name, pool, string(c.kind)}

	for k, v := range dataset.Properties {
		prop, err := datasetProperties.find(k)
		if err != nil {
			_ = level.Warn(c.log).Log(`msg`, propertyUnsupportedMsg, `help`, helpIssue, `property`, k, `err`, err)
		}
		if err = prop.push(ch, v, labelValues...); err != nil {
			return err
		}
	}

	return nil
}

func newDatasetCollector(kind zfs.DatasetKind, l log.Logger, props []string) (Collector, error) {
	switch kind {
	case zfs.DatasetFilesystem, zfs.DatasetSnapshot, zfs.DatasetVolume:
	default:
		return nil, fmt.Errorf("unknown dataset type: %s", kind)
	}

	return &datasetCollector{kind: kind, log: l, props: props}, nil
}

func newFilesystemCollector(l log.Logger, props []string) (Collector, error) {
	return newDatasetCollector(zfs.DatasetFilesystem, l, props)
}

func newSnapshotCollector(l log.Logger, props []string) (Collector, error) {
	return newDatasetCollector(zfs.DatasetSnapshot, l, props)
}

func newVolumeCollector(l log.Logger, props []string) (Collector, error) {
	return newDatasetCollector(zfs.DatasetVolume, l, props)
}
