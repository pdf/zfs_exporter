package collector

import (
	"fmt"

	"github.com/pdf/zfs_exporter/zfs"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector(`dataset-filesystem`, defaultEnabled, newFilesystemCollector)
	registerCollector(`dataset-snapshot`, defaultDisabled, newSnapshotCollector)
	registerCollector(`dataset-volume`, defaultEnabled, newVolumeCollector)
}

type datasetCollector struct {
	kind               string
	usedBytes          desc
	availableBytes     desc
	writtenBytes       desc
	volumeSizeBytes    desc
	logicalUsedBytes   desc
	usedByDatasetBytes desc
	quotaBytes         desc
	referencedBytes    desc
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
	labels := []string{dataset.Name, pool.Name, c.kind}

	// Metrics shared by all dataset types.
	ch <- metric{
		name: expandMetricName(c.usedBytes.name, labels...),
		prometheus: prometheus.MustNewConstMetric(
			c.usedBytes.prometheus,
			prometheus.GaugeValue,
			float64(dataset.Used),
			labels...,
		),
	}

	ch <- metric{
		name: expandMetricName(c.writtenBytes.name, labels...),
		prometheus: prometheus.MustNewConstMetric(
			c.writtenBytes.prometheus,
			prometheus.GaugeValue,
			float64(dataset.Written),
			labels...,
		),
	}

	ch <- metric{
		name: expandMetricName(c.logicalUsedBytes.name, labels...),
		prometheus: prometheus.MustNewConstMetric(
			c.logicalUsedBytes.prometheus,
			prometheus.GaugeValue,
			float64(dataset.Logicalused),
			labels...,
		),
	}

	ch <- metric{
		name: expandMetricName(c.referencedBytes.name, labels...),
		prometheus: prometheus.MustNewConstMetric(
			c.referencedBytes.prometheus,
			prometheus.GaugeValue,
			float64(dataset.Referenced),
			labels...,
		),
	}

	// Metrics shared by multiple dataset types.
	switch c.kind {
	case zfs.DatasetFilesystem, zfs.DatasetVolume:
		ch <- metric{
			name: expandMetricName(c.availableBytes.name, labels...),
			prometheus: prometheus.MustNewConstMetric(
				c.availableBytes.prometheus,
				prometheus.GaugeValue,
				float64(dataset.Avail),
				labels...,
			),
		}

		ch <- metric{
			name: expandMetricName(c.usedByDatasetBytes.name, labels...),
			prometheus: prometheus.MustNewConstMetric(
				c.usedByDatasetBytes.prometheus,
				prometheus.GaugeValue,
				float64(dataset.Usedbydataset),
				labels...,
			),
		}
	}

	// Metrics specific to individual dataset types.
	switch c.kind {
	case zfs.DatasetFilesystem:
		ch <- metric{
			name: expandMetricName(c.quotaBytes.name, labels...),
			prometheus: prometheus.MustNewConstMetric(
				c.quotaBytes.prometheus,
				prometheus.GaugeValue,
				float64(dataset.Quota),
				labels...,
			),
		}
	case zfs.DatasetVolume:
		ch <- metric{
			name: expandMetricName(c.volumeSizeBytes.name, labels...),
			prometheus: prometheus.MustNewConstMetric(
				c.volumeSizeBytes.prometheus,
				prometheus.GaugeValue,
				float64(dataset.Volsize),
				labels...,
			),
		}
	}

	return nil
}

func newDatasetCollector(kind string) (Collector, error) {
	switch kind {
	case zfs.DatasetFilesystem, zfs.DatasetSnapshot, zfs.DatasetVolume:
	default:
		return nil, fmt.Errorf("unknown dataset type: %s", kind)
	}

	const subsystem = `dataset`

	var (
		labels = []string{
			`name`,
			`pool`,
			`type`,
		}
		usedBytesName        = prometheus.BuildFQName(namespace, subsystem, `used_bytes`)
		availableBytesName   = prometheus.BuildFQName(namespace, subsystem, `available_bytes`)
		writtenBytesName     = prometheus.BuildFQName(namespace, subsystem, `written_bytes`)
		volumeSizeBytesName  = prometheus.BuildFQName(namespace, subsystem, `volume_size_bytes`)
		logicalUsedBytesName = prometheus.BuildFQName(namespace, subsystem, `logical_used_bytes`)
		usedByDatasetBytes   = prometheus.BuildFQName(namespace, subsystem, `used_by_dataset_bytes`)
		quotaBytesName       = prometheus.BuildFQName(namespace, subsystem, `quota_bytes`)
		referencedBytesName  = prometheus.BuildFQName(namespace, subsystem, `referenced_bytes`)
	)

	return &datasetCollector{
		kind: kind,
		usedBytes: desc{
			name: usedBytesName,
			prometheus: prometheus.NewDesc(
				usedBytesName,
				`The amount of space in bytes consumed by this dataset and all its descendents.`,
				labels,
				nil,
			),
		},
		availableBytes: desc{
			name: availableBytesName,
			prometheus: prometheus.NewDesc(
				availableBytesName,
				`The amount of space in bytes available to the dataset and all its children.`,
				labels,
				nil,
			),
		},
		writtenBytes: desc{
			name: writtenBytesName,
			prometheus: prometheus.NewDesc(
				writtenBytesName,
				`The amount of referenced space in bytes written to this dataset since the previous snapshot.`,
				labels,
				nil,
			),
		},
		volumeSizeBytes: desc{
			name: volumeSizeBytesName,
			prometheus: prometheus.NewDesc(
				volumeSizeBytesName,
				`The logical size of the volume in bytes.`,
				labels,
				nil,
			),
		},
		logicalUsedBytes: desc{
			name: logicalUsedBytesName,
			prometheus: prometheus.NewDesc(
				logicalUsedBytesName,
				`The amount of space in bytes that is "logically" consumed by this dataset and all its descendents.`,
				labels,
				nil,
			),
		},
		usedByDatasetBytes: desc{
			name: usedByDatasetBytes,
			prometheus: prometheus.NewDesc(
				usedByDatasetBytes,
				`The amount of space in bytes used by this dataset itself, which would be freed if the dataset were destroyed`,
				labels,
				nil,
			),
		},
		quotaBytes: desc{
			name: quotaBytesName,
			prometheus: prometheus.NewDesc(
				quotaBytesName,
				`The amount of space in bytes this dataset and its descendents can consume.`,
				labels,
				nil,
			),
		},
		referencedBytes: desc{
			name: referencedBytesName,
			prometheus: prometheus.NewDesc(
				referencedBytesName,
				`The amount of data in bytes that is accessible by this dataset, which may or may not be shared with other datasets in the pool.`,
				labels,
				nil,
			),
		},
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
