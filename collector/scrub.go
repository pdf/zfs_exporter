package collector

import (
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	"github.com/pdf/zfs_exporter/v2/zfs"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	defaultScrubProps = `state,errors,repaired_bytes,last_completed,total_bytes,scanned_bytes,issued_bytes,rate_bytes_per_second`
)

var (
	scrubLabels     = []string{`pool`}
	scrubProperties = propertyStore{
		defaultSubsystem: subsystemPool,
		defaultLabels:    scrubLabels,
		store: map[string]property{
			`state`: newProperty(
				subsystemPool,
				`scrub_state`,
				fmt.Sprintf("State code for the most recent scrub or resilver [%d: %s, %d: %s, %d: %s, %d: %s, %d: %s, %d: %s, %d: %s].",
					scanStateNone, scanStateNone.label(),
					scanStateScrubInProgress, scanStateScrubInProgress.label(),
					scanStateScrubFinished, scanStateScrubFinished.label(),
					scanStateScrubCanceled, scanStateScrubCanceled.label(),
					scanStateScrubPaused, scanStateScrubPaused.label(),
					scanStateResilverInProgress, scanStateResilverInProgress.label(),
					scanStateResilverFinished, scanStateResilverFinished.label(),
				),
				transformScanStateCode,
				prometheus.GaugeValue,
				scrubLabels...,
			),
			`errors`: newProperty(
				subsystemPool,
				`scrub_errors`,
				`Number of errors detected during the most recent scrub or resilver.`,
				transformNumeric,
				prometheus.GaugeValue,
				scrubLabels...,
			),
			`repaired_bytes`: newProperty(
				subsystemPool,
				`scrub_repaired_bytes`,
				`Bytes repaired during the most recent scrub or resilver.`,
				transformNumeric,
				prometheus.GaugeValue,
				scrubLabels...,
			),
			`last_completed`: newProperty(
				subsystemPool,
				`scrub_last_completed_timestamp_seconds`,
				`Unix timestamp of the most recently completed scrub or resilver, or 0 if none.`,
				transformNumeric,
				prometheus.GaugeValue,
				scrubLabels...,
			),
			`total_bytes`: newProperty(
				subsystemPool,
				`scrub_total_bytes`,
				`Total bytes to scan in the in-progress scrub or resilver, 0 if no scan is running.`,
				transformNumeric,
				prometheus.GaugeValue,
				scrubLabels...,
			),
			`scanned_bytes`: newProperty(
				subsystemPool,
				`scrub_scanned_bytes`,
				`Bytes read so far by the in-progress scrub or resilver, 0 if no scan is running.`,
				transformNumeric,
				prometheus.GaugeValue,
				scrubLabels...,
			),
			`issued_bytes`: newProperty(
				subsystemPool,
				`scrub_issued_bytes`,
				`Bytes verified so far by the in-progress scrub or resilver, 0 if no scan is running.`,
				transformNumeric,
				prometheus.GaugeValue,
				scrubLabels...,
			),
			`rate_bytes_per_second`: newProperty(
				subsystemPool,
				`scrub_rate_bytes_per_second`,
				`Current scan rate of the in-progress scrub or resilver, 0 if no scan is running.`,
				transformNumeric,
				prometheus.GaugeValue,
				scrubLabels...,
			),
		},
	}
)

func init() {
	registerCollector(`pool-scrub`, defaultEnabled, defaultScrubProps, newScrubCollector)
}

type scrubCollector struct {
	log    *slog.Logger
	client zfs.Client
	props  []string
}

func (c *scrubCollector) describe(ch chan<- *prometheus.Desc) {
	for _, k := range c.props {
		prop, err := scrubProperties.find(k)
		if err != nil {
			c.log.Warn(propertyUnsupportedMsg, `help`, helpIssue, `collector`, `pool-scrub`, `property`, k, `err`, err)
			continue
		}
		ch <- prop.desc
	}
}

func (c *scrubCollector) update(ch chan<- metric, pools []string, excludes regexpCollection) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(pools))
	for _, pool := range pools {
		wg.Add(1)
		go func(pool string) {
			if err := c.updateScrubMetrics(ch, pool); err != nil {
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

func (c *scrubCollector) updateScrubMetrics(ch chan<- metric, pool string) error {
	p := c.client.Pool(pool)
	scan, err := p.Scan()
	if err != nil {
		return err
	}

	labelValues := []string{pool}
	values := scrubMetricValues(scan)

	for _, k := range c.props {
		prop, err := scrubProperties.find(k)
		if err != nil {
			c.log.Warn(propertyUnsupportedMsg, `help`, helpIssue, `collector`, `pool-scrub`, `property`, k, `err`, err)
			continue
		}
		v, ok := values[k]
		if !ok {
			continue
		}
		if err = prop.push(ch, v, labelValues...); err != nil {
			return err
		}
	}

	return nil
}

func scrubMetricValues(scan zfs.PoolScan) map[string]string {
	completed := `0`
	if !scan.CompletedAt.IsZero() {
		completed = strconv.FormatInt(scan.CompletedAt.Unix(), 10)
	}
	return map[string]string{
		`state`:                 string(scan.Function) + `:` + string(scan.State),
		`errors`:                strconv.FormatUint(scan.Errors, 10),
		`repaired_bytes`:        strconv.FormatUint(scan.Repaired, 10),
		`last_completed`:        completed,
		`total_bytes`:           strconv.FormatUint(scan.Total, 10),
		`scanned_bytes`:         strconv.FormatUint(scan.Scanned, 10),
		`issued_bytes`:          strconv.FormatUint(scan.Issued, 10),
		`rate_bytes_per_second`: strconv.FormatUint(scan.Rate, 10),
	}
}

func newScrubCollector(l *slog.Logger, c zfs.Client, props []string) (Collector, error) {
	return &scrubCollector{log: l, client: c, props: props}, nil
}
