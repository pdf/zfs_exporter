package collector

import (
	"fmt"
	"log/slog"

	"github.com/pdf/zfs_exporter/v2/zfs"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	vdevSizeDesc = prometheus.NewDesc(
		"zfs_pool_vdev_size_bytes",
		"Allocatable ZFS vdev capacity in bytes (not raw physical member capacity).",
		[]string{"pool", "vdev"}, nil,
	)
	vdevAllocatedDesc = prometheus.NewDesc(
		"zfs_pool_vdev_allocated_bytes",
		"Bytes allocated in the ZFS vdev.",
		[]string{"pool", "vdev"}, nil,
	)
	vdevFreeDesc = prometheus.NewDesc(
		"zfs_pool_vdev_free_bytes",
		"Bytes free in the ZFS vdev.",
		[]string{"pool", "vdev"}, nil,
	)
	poolDiskSizeDesc = prometheus.NewDesc(
		"zfs_pool_disk_size_bytes",
		"Raw size in bytes of a block device or partition belonging to a ZFS vdev; not ZFS allocated space.",
		[]string{"pool", "vdev", "device"}, nil,
	)
)

func init() {
	registerCollector("pool-topology", defaultDisabled, "", newPoolTopologyCollector)
}

type poolTopologyCollector struct {
	client zfs.Client
}

func newPoolTopologyCollector(_ *slog.Logger, client zfs.Client, _ []string) (Collector, error) {
	return &poolTopologyCollector{client: client}, nil
}

func (c *poolTopologyCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- vdevSizeDesc
	ch <- vdevAllocatedDesc
	ch <- vdevFreeDesc
	ch <- poolDiskSizeDesc
}

func (c *poolTopologyCollector) update(ch chan<- metric, pools []string, _ regexpCollection) error {
	for _, pool := range pools {
		topology, err := c.client.PoolTopology(pool)
		if err != nil {
			return fmt.Errorf("pool-topology %s: %w", pool, err)
		}
		for _, v := range topology.Vdevs {
			emitTopologyMetric(ch, vdevSizeDesc, "zfs_pool_vdev_size_bytes", float64(v.SizeBytes), pool, v.Name)
			emitTopologyMetric(ch, vdevAllocatedDesc, "zfs_pool_vdev_allocated_bytes", float64(v.AllocatedBytes), pool, v.Name)
			emitTopologyMetric(ch, vdevFreeDesc, "zfs_pool_vdev_free_bytes", float64(v.FreeBytes), pool, v.Name)
		}
		for _, d := range topology.Disks {
			emitTopologyMetric(ch, poolDiskSizeDesc, "zfs_pool_disk_size_bytes", float64(d.SizeBytes), pool, d.Vdev, d.Device)
		}
	}
	return nil
}

func emitTopologyMetric(ch chan<- metric, desc *prometheus.Desc, name string, value float64, labels ...string) {
	ch <- metric{
		name:       expandMetricName(name, labels...),
		prometheus: prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, labels...),
	}
}
