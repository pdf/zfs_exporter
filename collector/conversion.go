package collector

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/pdf/zfs_exporter/basic_zfs"
)

var PropertyConversionError = errors.New("converting property failed")

func float64FromBoolProp(propValue string) (float64, error) {
	switch propValue {
	case "on", "yes":
		return 1, nil
	case "off", "no":
		return 0, nil
	}

	return 0, fmt.Errorf("converting %s to bool: %w", propValue, PropertyConversionError)
}

func float64FromNumProp(dsPropValue string) float64 {
	var v float64
	if dsPropValue != "-" && dsPropValue != "none" {
		var err error
		v, err = strconv.ParseFloat(dsPropValue, 64)
		if err != nil {
			return 0
		}
	}
	return v
}

func healthCodeFromString(status string) (healthCode, error) {
	switch status {
	case basic_zfs.ZpoolOnline:
		return online, nil
	case basic_zfs.ZpoolDegraded:
		return degraded, nil
	case basic_zfs.ZpoolFaulted:
		return faulted, nil
	case basic_zfs.ZpoolOffline:
		return offline, nil
	case basic_zfs.ZpoolUnavail:
		return unavail, nil
	case basic_zfs.ZpoolRemoved:
		return removed, nil
	}

	return -1, fmt.Errorf(`unknown pool heath status: %s`, status)
}
