package collector

import (
	"fmt"
	"strconv"

	"github.com/pdf/zfs_exporter/v2/zfs"
)

type poolHealthCode int

const (
	poolOnline poolHealthCode = iota
	poolDegraded
	poolFaulted
	poolOffline
	poolUnavail
	poolRemoved
	poolSuspended
)

type poolScanStateCode int

const (
	scanStateNone poolScanStateCode = iota
	scanStateScrubInProgress
	scanStateScrubFinished
	scanStateScrubCanceled
	scanStateScrubPaused
	scanStateResilverInProgress
	scanStateResilverFinished
)

func (c poolScanStateCode) label() string {
	switch c {
	case scanStateNone:
		return `none`
	case scanStateScrubInProgress:
		return `scrub_in_progress`
	case scanStateScrubFinished:
		return `scrub_finished`
	case scanStateScrubCanceled:
		return `scrub_canceled`
	case scanStateScrubPaused:
		return `scrub_paused`
	case scanStateResilverInProgress:
		return `resilver_in_progress`
	case scanStateResilverFinished:
		return `resilver_finished`
	}
	return ``
}

func transformNumeric(value string) (float64, error) {
	if value == `-` || value == `none` {
		return 0, nil
	}
	return strconv.ParseFloat(value, 64)
}

func transformHealthCode(status string) (float64, error) {
	var result poolHealthCode
	switch zfs.PoolStatus(status) {
	case zfs.PoolOnline:
		result = poolOnline
	case zfs.PoolDegraded:
		result = poolDegraded
	case zfs.PoolFaulted:
		result = poolFaulted
	case zfs.PoolOffline:
		result = poolOffline
	case zfs.PoolUnavail:
		result = poolUnavail
	case zfs.PoolRemoved:
		result = poolRemoved
	case zfs.PoolSuspended:
		result = poolSuspended
	default:
		return -1, fmt.Errorf(`unknown pool heath status: %s`, status)
	}

	return float64(result), nil
}

func transformScanStateCode(value string) (float64, error) {
	switch value {
	case `none:none`:
		return float64(scanStateNone), nil
	case `scrub:in_progress`:
		return float64(scanStateScrubInProgress), nil
	case `scrub:finished`:
		return float64(scanStateScrubFinished), nil
	case `scrub:canceled`:
		return float64(scanStateScrubCanceled), nil
	case `scrub:paused`:
		return float64(scanStateScrubPaused), nil
	case `resilver:in_progress`:
		return float64(scanStateResilverInProgress), nil
	case `resilver:finished`:
		return float64(scanStateResilverFinished), nil
	}
	return -1, fmt.Errorf(`unknown scan state: %s`, value)
}

func transformBool(value string) (float64, error) {
	switch value {
	case `on`, `yes`, `enabled`, `active`:
		return 1, nil
	case `off`, `no`, `disabled`, `inactive`, `-`:
		return 0, nil
	}

	return -1, fmt.Errorf(`could not convert '%s' to bool`, value)
}

func transformPercentage(value string) (float64, error) {
	if len(value) > 0 && value[len(value)-1] == '%' {
		value = value[:len(value)-1]
	}
	v, err := transformNumeric(value)
	if err != nil {
		return -1, err
	}

	return v / 100, nil
}

func transformMultiplier(value string) (float64, error) {
	if len(value) > 0 && value[len(value)-1] == 'x' {
		value = value[:len(value)-1]
	}
	v, err := transformNumeric(value)
	if err != nil {
		return -1, err
	}
	return 1 / v, nil
}
