package collector

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	defaultEnabled           = true
	defaultDisabled          = false
	namespace                = `zfs`
	helpDefaultStateEnabled  = `enabled`
	helpDefaultStateDisabled = `disabled`

	subsystemDataset = `dataset`
	subsystemPool    = `pool`

	propertyUnsupportedDesc = `!!! This property is unsupported, results are likely to be undesirable, please file an issue at https://github.com/pdf/zfs_exporter/issues to have this property supported !!!`
	propertyUnsupportedMsg  = `Unsupported dataset property, results are likely to be undesirable`
	helpIssue               = `Please file an issue at https://github.com/pdf/zfs_exporter/issues`
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

	errUnsupportedProperty = errors.New(`unsupported property`)
)

type factoryFunc func(l log.Logger, properties []string) (Collector, error)

type transformFunc func(string) (float64, error)

// State holds metadata for managing collector status
type State struct {
	Name       string
	Enabled    *bool
	Properties *string
	factory    factoryFunc
}

// Collector defines the minimum functionality for registering a collector
type Collector interface {
	update(ch chan<- metric, pools []string, excludes regexpCollection) error
}

type metric struct {
	name       string
	prometheus prometheus.Metric
}

type property struct {
	name      string
	desc      *prometheus.Desc
	transform transformFunc
}

func (p property) push(ch chan<- metric, value string, labelValues ...string) error {
	v, err := p.transform(value)
	if err != nil {
		return err
	}
	ch <- metric{
		name: expandMetricName(p.name, labelValues...),
		prometheus: prometheus.MustNewConstMetric(
			p.desc,
			prometheus.GaugeValue,
			v,
			labelValues...,
		),
	}

	return nil
}

type propertyStore struct {
	defaultSubsystem string
	defaultLabels    []string
	store            map[string]property
}

func (p *propertyStore) find(name string) (property, error) {
	prop, ok := p.store[name]
	if !ok {
		prop = newProperty(
			p.defaultSubsystem,
			name,
			propertyUnsupportedDesc,
			transformNumeric,
			p.defaultLabels...,
		)
		return prop, errUnsupportedProperty
	}
	return prop, nil
}

func registerCollector(collector string, isDefaultEnabled bool, defaultProps string, factory factoryFunc) {
	helpDefaultState := helpDefaultStateDisabled
	if isDefaultEnabled {
		helpDefaultState = helpDefaultStateEnabled
	}

	enabledFlagName := fmt.Sprintf("collector.%s", collector)
	enabledFlagHelp := fmt.Sprintf("Enable the %s collector (default: %s)", collector, helpDefaultState)
	enabledDefaultValue := fmt.Sprintf("%t", isDefaultEnabled)

	propsFlagName := fmt.Sprintf("properties.%s", collector)
	propsFlagHelp := fmt.Sprintf("Properties to include for the %s collector, comma-separated.", collector)

	enabledFlag := kingpin.Flag(enabledFlagName, enabledFlagHelp).Default(enabledDefaultValue).Bool()
	propsFlag := kingpin.Flag(propsFlagName, propsFlagHelp).Default(defaultProps).String()

	collectorStates[collector] = State{
		Enabled:    enabledFlag,
		Properties: propsFlag,
		factory:    factory,
	}
}

func expandMetricName(prefix string, context ...string) string {
	return strings.Join(append(context, prefix), `-`)
}

func newProperty(subsystem, metricName, helpText string, transform transformFunc, labels ...string) property {
	name := prometheus.BuildFQName(namespace, subsystem, metricName)
	return property{
		name:      name,
		desc:      prometheus.NewDesc(name, helpText, labels, nil),
		transform: transform,
	}
}
