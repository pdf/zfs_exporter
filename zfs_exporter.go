package main

import (
	"net/http"
	"os"
	"strings"

	"github.com/pdf/zfs_exporter/v2/collector"
	"github.com/pdf/zfs_exporter/v2/zfs"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/exporter-toolkit/web"
	"github.com/prometheus/exporter-toolkit/web/kingpinflag"

	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/common/version"
)

func main() {
	var (
		metricsPath             = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		metricsExporterDisabled = kingpin.Flag(`web.disable-exporter-metrics`, `Exclude metrics about the exporter itself (promhttp_*, process_*, go_*).`).Default(`false`).Bool()
		deadline                = kingpin.Flag("deadline", "Maximum duration that a collection should run before returning cached data. Should be set to a value shorter than your scrape timeout duration. The current collection run will continue and update the cache when complete (default: 8s)").Default("8s").Duration()
		pools                   = kingpin.Flag("pool", "Name of the pool(s) to collect, repeat for multiple pools (default: all pools).").Strings()
		excludes                = kingpin.Flag("exclude", "Exclude datasets/snapshots/volumes that match the provided regex (e.g. '^rpool/docker/'), may be specified multiple times.").Strings()
		hostRoot                = kingpin.Flag(`host-root`, `For containers: run zpool and zfs chrooted into this directory, where the host's root filesystem is mounted (the image sets /host). Unset (the default), they run from the exporter's own PATH as its own user. Linux only; needs CAP_SYS_CHROOT, CAP_SETUID and CAP_SETGID.`).String()
		hostRootPath            = kingpin.Flag(`host-root.path`, `With --host-root: colon-separated directories inside the host root searched for zpool and zfs.`).Default(strings.Join(zfs.DefaultHostPath, `:`)).String()
		hostRootUID             = kingpin.Flag(`host-root.uid`, `With --host-root: user ID that zpool and zfs run as.`).Default(`65534`).Uint32()
		hostRootGID             = kingpin.Flag(`host-root.gid`, `With --host-root: group ID that zpool and zfs run as.`).Default(`65534`).Uint32()
		toolkitFlags            = kingpinflag.AddFlags(kingpin.CommandLine, ":9134")
	)

	promslogConfig := &promslog.Config{}
	flag.AddFlags(kingpin.CommandLine, promslogConfig)
	kingpin.Version(version.Print("zfs_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promslog.New(promslogConfig)

	logger.Info("Starting zfs_exporter", "version", version.Info())
	logger.Info("Build context", "context", version.BuildContext())

	client := zfs.New()
	if *hostRoot != `` {
		var info zfs.HostRootInfo
		var err error
		client, info, err = zfs.NewHostRoot(zfs.HostRootConfig{
			Root: *hostRoot,
			Path: strings.Split(*hostRootPath, `:`),
			UID:  *hostRootUID,
			GID:  *hostRootGID,
		})
		if err != nil {
			logger.Error("Error using the host root", "err", err)
			os.Exit(1)
		}
		logger.Info("Using the host's ZFS", "root", *hostRoot, "zpool", info.Zpool, "zfs", info.ZFS, "version", info.Version)
	}

	c, err := collector.NewZFS(collector.ZFSConfig{
		DisableMetrics: *metricsExporterDisabled,
		Deadline:       *deadline,
		Pools:          *pools,
		Excludes:       *excludes,
		Logger:         logger,
		ZFSClient:      client,
	})
	if err != nil {
		logger.Error("Error creating an exporter", "err", err)
		os.Exit(1)
	}

	if *metricsExporterDisabled {
		r := prometheus.NewRegistry()
		prometheus.DefaultRegisterer = r
		prometheus.DefaultGatherer = r
	}
	prometheus.MustRegister(c)
	prometheus.MustRegister(versioncollector.NewCollector("zfs_exporter"))

	if len(c.Pools) > 0 {
		logger.Info("Enabling pools", "pools", strings.Join(c.Pools, ", "))
	} else {
		logger.Info("Enabling pools", "pools", "(all)")
	}

	collectorNames := make([]string, 0, len(c.Collectors))
	for n, c := range c.Collectors {
		if *c.Enabled {
			collectorNames = append(collectorNames, n)
		}
	}
	logger.Info("Enabling collectors", "collectors", strings.Join(collectorNames, ", "))

	http.Handle(*metricsPath, promhttp.Handler())
	if *metricsPath != "/" {
		landingConfig := web.LandingConfig{
			Name:        "ZFS Exporter",
			Description: "Prometheus ZFS Exporter",
			Version:     version.Info(),
			Links: []web.LandingLinks{
				{
					Address: *metricsPath,
					Text:    "Metrics",
				},
			},
		}
		landingPage, err := web.NewLandingPage(landingConfig)
		if err != nil {
			logger.Error("Error creating landing page", "err", err)
			os.Exit(1)
		}
		http.Handle("/", landingPage)
	}

	server := &http.Server{}
	err = web.ListenAndServe(server, toolkitFlags, logger)
	if err != nil {
		logger.Error("Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}
