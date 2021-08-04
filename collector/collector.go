package collector

import (
	"fmt"
	"strings"

	"github.com/mistifyio/go-zfs"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	defaultEnabled           = true
	defaultDisabled          = false
	namespace                = `zfs`
	helpDefaultStateEnabled  = `enabled`
	helpDefaultStateDisabled = `disabled`
)

var (
	collectorStates        = make(map[string]State)
	scrapeDurationDescName = prometheus.BuildFQName(namespace, `scrape`, `collector_duration_seconds`)
	scrapeDurationDesc     = prometheus.NewDesc(
		scrapeDurationDescName,
		`zfs_exporter: Duration of a collector scrape.`,
		[]string{`collector`},
		nil,
	)
	scrapeSuccessDescName = prometheus.BuildFQName(namespace, `scrape`, `collector_success`)
	scrapeSuccessDesc     = prometheus.NewDesc(
		scrapeSuccessDescName,
		`zfs_exporter: Whether a collector succeeded.`,
		[]string{`collector`},
		nil,
	)
)

type factoryFunc func() (Collector, error)

type State struct {
	Enabled *bool
	factory factoryFunc
}

type Collector interface {
	update(ch chan<- metric, pools []*zfs.Zpool, excludes regexpCollection) error
}

type metric struct {
	name       string
	prometheus prometheus.Metric
}

type desc struct {
	name       string
	prometheus *prometheus.Desc
}

func registerCollector(collector string, isDefaultEnabled bool, factory factoryFunc) {
	helpDefaultState := helpDefaultStateDisabled
	if isDefaultEnabled {
		helpDefaultState = helpDefaultStateEnabled
	}

	flagName := fmt.Sprintf("collector.%s", collector)
	flagHelp := fmt.Sprintf("Enable the %s collector (default: %s)", collector, helpDefaultState)
	defaultValue := fmt.Sprintf("%t", isDefaultEnabled)

	flag := kingpin.Flag(flagName, flagHelp).Default(defaultValue).Bool()

	collectorStates[collector] = State{
		Enabled: flag,
		factory: factory,
	}
}

func expandMetricName(prefix string, context ...string) string {
	return strings.Join(append(context, prefix), `-`)
}

func newDesc(subsystem string, metric_name string, help_text string, labels []string) desc {
	var name = prometheus.BuildFQName(namespace, subsystem, metric_name)
	return desc{
		name: name,
		prometheus: prometheus.NewDesc(
			name,
			help_text,
			labels,
			nil,
		),
	}
}

func newMetric(metric_desc *desc, value float64, labels []string) metric {
	return metric{
		name: expandMetricName(metric_desc.name, labels...),
		prometheus: prometheus.MustNewConstMetric(
			metric_desc.prometheus,
			prometheus.GaugeValue,
			value,
			labels...,
		),
	}
}
