package collector

import (
	"fmt"
	"log"
	"strconv"

	"github.com/pdf/zfs_exporter/basic_zfs"
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

var datasetProperties = []string{
	"name", // NOTE: name should be first!
	// all datasets
	"logicalreferenced",
	"logicalused",
	"referenced",
	"used",
	"written",
	// volumes and filesystems only (i.e. no snapshots)
	"available",
	"usedbychildren",
	"usedbydataset",
	"usedbyrefreservation",
	"usedbysnapshots",
	// filesystems only
	"quota",
	// volumes only
	"volsize",
}

type datasetCollector struct {
	kind string
	// all datasets
	logicalReferencedBytes desc
	logicalUsedBytes       desc
	referencedBytes        desc
	usedBytes              desc
	writtenBytes           desc
	// volumes and filesystems only (i.e. no snapshots)
	availableBytes            desc
	usedByChildrenBytes       desc
	usedByDatasetBytes        desc
	usedByRefreservationBytes desc
	usedBySnapshotsBytes      desc
	// filesystems only
	quotaBytes desc
	// volumes only
	volumeSizeBytes desc
}

func (c *datasetCollector) toFloat64(dsPropValue string) float64 {
	var v float64
	if dsPropValue != "-" && dsPropValue != "none" {
		var err error
		v, err = strconv.ParseFloat(dsPropValue, 64)
		if err != nil {
			log.Fatalln(err)
			return 0
		}
	}
	return v
}

func (c *datasetCollector) update(ch chan<- metric, pools []string, excludes regexpCollection) error {
	for _, pool := range pools {
		if err := c.updatePoolMetrics(ch, pool, excludes); err != nil {
			return err
		}
	}

	return nil
}

func (c *datasetCollector) updatePoolMetrics(ch chan<- metric, pool string, excludes regexpCollection) error {
	var (
		datasetsWithProperties [][]string
		err                    error
	)
	switch c.kind {
	case basic_zfs.DatasetFilesystem:
		datasetsWithProperties, err = basic_zfs.FilesystemProperties(pool, datasetProperties)
	case basic_zfs.DatasetSnapshot:
		datasetsWithProperties, err = basic_zfs.SnapshotProperties(pool, datasetProperties)
	case basic_zfs.DatasetVolume:
		datasetsWithProperties, err = basic_zfs.VolumeProperties(pool, datasetProperties)
	}
	if err != nil {
		return err
	}

	for _, dsProps := range datasetsWithProperties {
		if excludes.MatchString(dsProps[0]) {
			continue
		}
		if err = c.updateDatasetMetrics(ch, pool, dsProps); err != nil {
			return err
		}
	}

	return nil
}

func (c *datasetCollector) updateDatasetMetrics(ch chan<- metric, pool string, dsProps []string) error {
	// match with datasetLabels
	labelValues := []string{dsProps[0], pool, c.kind}

	// NOTE: dsProps indices must match up with datasetProperties indices

	// Metrics shared by all dataset types.
	ch <- newGaugeMetric(
		c.logicalReferencedBytes,
		c.toFloat64(dsProps[1]),
		labelValues,
	)

	ch <- newGaugeMetric(
		c.logicalUsedBytes,
		c.toFloat64(dsProps[2]),
		labelValues,
	)

	ch <- newGaugeMetric(
		c.referencedBytes,
		c.toFloat64(dsProps[3]),
		labelValues,
	)

	ch <- newGaugeMetric(
		c.usedBytes,
		c.toFloat64(dsProps[4]),
		labelValues,
	)

	ch <- newGaugeMetric(
		c.writtenBytes,
		c.toFloat64(dsProps[5]),
		labelValues,
	)

	// Metrics shared by multiple dataset types.
	switch c.kind {
	case basic_zfs.DatasetFilesystem, basic_zfs.DatasetVolume:
		ch <- newGaugeMetric(
			c.availableBytes,
			c.toFloat64(dsProps[6]),
			labelValues,
		)

		ch <- newGaugeMetric(
			c.usedByChildrenBytes,
			c.toFloat64(dsProps[7]),
			labelValues,
		)

		ch <- newGaugeMetric(
			c.usedByDatasetBytes,
			c.toFloat64(dsProps[8]),
			labelValues,
		)

		ch <- newGaugeMetric(
			c.usedByRefreservationBytes,
			c.toFloat64(dsProps[9]),
			labelValues,
		)

		ch <- newGaugeMetric(
			c.usedBySnapshotsBytes,
			c.toFloat64(dsProps[10]),
			labelValues,
		)
	}

	// Metrics specific to individual dataset types.
	switch c.kind {
	case basic_zfs.DatasetFilesystem:
		ch <- newGaugeMetric(
			c.quotaBytes,
			c.toFloat64(dsProps[11]),
			labelValues,
		)

	case basic_zfs.DatasetVolume:
		ch <- newGaugeMetric(
			c.volumeSizeBytes,
			c.toFloat64(dsProps[12]),
			labelValues,
		)
	}

	return nil
}

func newDatasetCollector(kind string) (Collector, error) {
	switch kind {
	case basic_zfs.DatasetFilesystem, basic_zfs.DatasetSnapshot, basic_zfs.DatasetVolume:
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
		logicalReferencedBytes: newDesc(
			datasetSubsystem,
			`logical_referenced_bytes`,
			`The amount of space in bytes that is “logically” accessible by this dataset.`,
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
		usedByChildrenBytes: newDesc(
			datasetSubsystem,
			`used_by_children_bytes`,
			`The amount of space in bytes used by children of this dataset, which would be freed if all the dataset's children were destroyed.`,
			datasetLabels,
		),
		usedByDatasetBytes: newDesc(
			datasetSubsystem,
			`used_by_dataset_bytes`,
			`The amount of space in bytes used by this dataset itself, which would be freed if the dataset were destroyed`,
			datasetLabels,
		),
		usedByRefreservationBytes: newDesc(
			datasetSubsystem,
			`used_by_refreservation_bytes`,
			`The amount of space in bytes used by a refreservation set on this dataset, which would be freed if the refreservation was removed.`,
			datasetLabels,
		),
		usedBySnapshotsBytes: newDesc(
			datasetSubsystem,
			`used_by_snapshots_bytes`,
			`The amount of space in bytes consumed by snapshots of this dataset. In particular, it is the amount of space that would be freed if all of this dataset's snapshots were destroyed.`,
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
	return newDatasetCollector(basic_zfs.DatasetFilesystem)
}

func newSnapshotCollector() (Collector, error) {
	return newDatasetCollector(basic_zfs.DatasetSnapshot)
}

func newVolumeCollector() (Collector, error) {
	return newDatasetCollector(basic_zfs.DatasetVolume)
}
