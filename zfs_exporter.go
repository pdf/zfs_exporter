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
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	"github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

func main() {
	var (
		metricsPath             = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		metricsExporterDisabled = kingpin.Flag(`web.disable-exporter-metrics`, `Exclude metrics about the exporter itself (promhttp_*, process_*, go_*).`).Default(`false`).Bool()
		deadline                = kingpin.Flag("deadline", "Maximum duration that a collection should run before returning cached data. Should be set to a value shorter than your scrape timeout duration. The current collection run will continue and update the cache when complete (default: 8s)").Default("8s").Duration()
		pools                   = kingpin.Flag("pool", "Name of the pool(s) to collect, repeat for multiple pools (default: all pools).").Strings()
		excludes                = kingpin.Flag("exclude", "Exclude datasets/snapshots/volumes that match the provided regex (e.g. '^rpool/docker/'), may be specified multiple times.").Strings()
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

	c, err := collector.NewZFS(collector.ZFSConfig{
		DisableMetrics: *metricsExporterDisabled,
		Deadline:       *deadline,
		Pools:          *pools,
		Excludes:       *excludes,
		Logger:         logger,
		ZFSClient:      zfs.New(),
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
