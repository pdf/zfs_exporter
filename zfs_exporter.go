package main

import (
	"fmt"
	"net/http"

	"github.com/pdf/zfs_exporter/collector"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

func init() {
	prometheus.MustRegister(version.NewCollector("zfs_exporter"))
}

func handler(c *collector.ZFSCollector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		registry := prometheus.NewRegistry()
		if err := registry.Register(c); err != nil {
			serr := fmt.Sprintf("Couldn't register collector: %s", err)
			log.Errorln(serr)
			w.WriteHeader(http.StatusInternalServerError)
			if _, err = w.Write([]byte(serr)); err != nil {
				log.Warnln(`Couldn't write response:`, err)
			}
			return
		}

		gatherers := prometheus.Gatherers{
			prometheus.DefaultGatherer,
			registry,
		}

		h := promhttp.InstrumentMetricHandler(
			registry,
			promhttp.HandlerFor(gatherers,
				promhttp.HandlerOpts{
					ErrorLog:      log.NewErrorLogger(),
					ErrorHandling: promhttp.ContinueOnError,
				}),
		)
		h.ServeHTTP(w, r)
	}
}

func main() {
	var (
		listenAddress = kingpin.Flag("web.listen-address", "Address on which to expose metrics and web interface.").Default(":9134").String()
		metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		deadline      = kingpin.Flag("deadline", "Maximum duration that a collection should run before returning cached data. Should be set to a value shorter than your scrape timeout duration. The current collection run will continue and update the cache when complete (default: 8s)").Default("8s").Duration()
		pools         = kingpin.Flag("pool", "Name of the pool(s) to collect, repeat for multiple pools (default: all pools).").Strings()
	)

	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("zfs_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting zfs_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	c, err := collector.NewZFSCollector(*deadline, *pools)
	if err != nil {
		log.Fatalf("Couldn't create collector: %s", err)
	}

	log.Infof("Enabling pools:")
	for _, p := range c.Pools {
		log.Infof(" - %s", p)
	}
	if len(c.Pools) == 0 {
		log.Infof(" - (all)")
	}

	log.Infof("Enabling collectors:")
	for n, c := range c.Collectors {
		if *c.Enabled {
			log.Infof(" - %s", n)
		}
	}

	http.HandleFunc(*metricsPath, handler(c))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err = w.Write([]byte(`<html>
			<head><title>ZFS Exporter</title></head>
			<body>
			<h1>ZFS Exporter</h1>
			<p><a href="` + *metricsPath + `">Metrics</a></p>
			</body>
			</html>`))
		if err != nil {
			log.Errorln(err)
		}
	})

	log.Infoln("Listening on", *listenAddress)
	err = http.ListenAndServe(*listenAddress, nil)
	if err != nil {
		log.Fatal(err)
	}
}
