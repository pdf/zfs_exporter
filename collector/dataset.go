package collector

import (
	"fmt"

	zfs "github.com/mistifyio/go-zfs"
)

func init() {
	registerCollector(`dataset-filesystem`, defaultEnabled, newFilesystemCollector)
	registerCollector(`dataset-snapshot`, defaultDisabled, newSnapshotCollector)
	registerCollector(`dataset-volume`, defaultEnabled, newVolumeCollector)
}

const datasetSubsystem = `dataset`

var datasetLabels = []string{
	`name`,
	`pool`,
	`type`,
}

type datasetCollector struct {
	kind string
	// all datasets
	logicalUsedBytes desc
	referencedBytes  desc
	usedBytes        desc
	writtenBytes     desc
	// volumes and filesystems only (i.e. no snapshots)
	availableBytes     desc
	usedByDatasetBytes desc
	// filesystems only
	quotaBytes desc
	// volumes only
	volumeSizeBytes desc
}

func (c *datasetCollector) update(ch chan<- metric, pools []*zfs.Zpool, excludes regexpCollection) error {
	for _, pool := range pools {
		if err := c.updatePoolMetrics(ch, pool, excludes); err != nil {
			return err
		}
	}

	return nil
}

func (c *datasetCollector) updatePoolMetrics(ch chan<- metric, pool *zfs.Zpool, excludes regexpCollection) error {
	var (
		datasets []*zfs.Dataset
		err      error
	)
	switch c.kind {
	case zfs.DatasetFilesystem:
		datasets, err = zfs.Filesystems(pool.Name)
	case zfs.DatasetSnapshot:
		datasets, err = zfs.Snapshots(pool.Name)
	case zfs.DatasetVolume:
		datasets, err = zfs.Volumes(pool.Name)
	}
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

func (c *datasetCollector) updateDatasetMetrics(ch chan<- metric, pool *zfs.Zpool, dataset *zfs.Dataset) error {
	// match with datasetLabels
	labelValues := []string{dataset.Name, pool.Name, c.kind}

	// Metrics shared by all dataset types.
	ch <- newGaugeMetric(
		c.logicalUsedBytes,
		float64(dataset.Logicalused),
		labelValues,
	)

	ch <- newGaugeMetric(
		c.referencedBytes,
		float64(dataset.Referenced),
		labelValues,
	)

	ch <- newGaugeMetric(
		c.usedBytes,
		float64(dataset.Used),
		labelValues,
	)

	ch <- newGaugeMetric(
		c.writtenBytes,
		float64(dataset.Written),
		labelValues,
	)

	// Metrics shared by multiple dataset types.
	switch c.kind {
	case zfs.DatasetFilesystem, zfs.DatasetVolume:
		ch <- newGaugeMetric(
			c.availableBytes,
			float64(dataset.Avail),
			labelValues,
		)

		ch <- newGaugeMetric(
			c.usedByDatasetBytes,
			float64(dataset.Usedbydataset),
			labelValues,
		)
	}

	// Metrics specific to individual dataset types.
	switch c.kind {
	case zfs.DatasetFilesystem:
		ch <- newGaugeMetric(
			c.quotaBytes,
			float64(dataset.Quota),
			labelValues,
		)

	case zfs.DatasetVolume:
		ch <- newGaugeMetric(
			c.volumeSizeBytes,
			float64(dataset.Volsize),
			labelValues,
		)
	}

	return nil
}

func newDatasetCollector(kind string) (Collector, error) {
	switch kind {
	case zfs.DatasetFilesystem, zfs.DatasetSnapshot, zfs.DatasetVolume:
	default:
		return nil, fmt.Errorf("unknown dataset type: %s", kind)
	}

	return &datasetCollector{
		kind: kind,
		availableBytes: newDesc(
			datasetSubsystem,
			`available_bytes`,
			`The amount of space in bytes available to the dataset and all its children.`,
			datasetLabels,
		),
		logicalUsedBytes: newDesc(
			datasetSubsystem,
			`logical_used_bytes`,
			`The amount of space in bytes that is "logically" consumed by this dataset and all its descendents.`,
			datasetLabels,
		),
		quotaBytes: newDesc(
			datasetSubsystem,
			`quota_bytes`,
			`The amount of space in bytes this dataset and its descendents can consume.`,
			datasetLabels,
		),
		referencedBytes: newDesc(
			datasetSubsystem,
			`referenced_bytes`,
			`The amount of data in bytes that is accessible by this dataset, which may or may not be shared with other datasets in the pool.`,
			datasetLabels,
		),
		usedByDatasetBytes: newDesc(
			datasetSubsystem,
			`used_by_dataset_bytes`,
			`The amount of space in bytes used by this dataset itself, which would be freed if the dataset were destroyed`,
			datasetLabels,
		),
		usedBytes: newDesc(
			datasetSubsystem,
			`used_bytes`,
			`The amount of space in bytes consumed by this dataset and all its descendents.`,
			datasetLabels,
		),
		volumeSizeBytes: newDesc(
			datasetSubsystem,
			`volume_size_bytes`,
			`The logical size of the volume in bytes.`,
			datasetLabels,
		),
		writtenBytes: newDesc(
			datasetSubsystem,
			`written_bytes`,
			`The amount of referenced space in bytes written to this dataset since the previous snapshot.`,
			datasetLabels,
		),
	}, nil
}

func newFilesystemCollector() (Collector, error) {
	return newDatasetCollector(zfs.DatasetFilesystem)
}

func newSnapshotCollector() (Collector, error) {
	return newDatasetCollector(zfs.DatasetSnapshot)
}

func newVolumeCollector() (Collector, error) {
	return newDatasetCollector(zfs.DatasetVolume)
}
