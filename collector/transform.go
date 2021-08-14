package collector

import (
	"fmt"
	"strconv"

	"github.com/pdf/zfs_exporter/zfs"
)

type poolHealthCode int

const (
	poolOnline poolHealthCode = iota
	poolDegraded
	poolFaulted
	poolOffline
	poolUnavail
	poolRemoved
)

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
	default:
		return -1, fmt.Errorf(`unknown pool heath status: %s`, status)
	}

	return float64(result), nil
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
	v, err := transformNumeric(value)
	if err != nil {
		return -1, err
	}

	return v / 100, nil
}

func transformMultiplier(value string) (float64, error) {
	v, err := transformNumeric(value)
	if err != nil {
		return -1, err
	}
	return 1 / v, nil
}
