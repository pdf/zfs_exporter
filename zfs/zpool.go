package zfs

import (
	"strconv"
	"strings"
)

// ZFS zpool states, which can indicate if a pool is online, offline,
// degraded, etc.  More information regarding zpool states can be found here:
// https://docs.oracle.com/cd/E19253-01/819-5461/gamno/index.html.
const (
	ZpoolOnline   = "ONLINE"
	ZpoolDegraded = "DEGRADED"
	ZpoolFaulted  = "FAULTED"
	ZpoolOffline  = "OFFLINE"
	ZpoolUnavail  = "UNAVAIL"
	ZpoolRemoved  = "REMOVED"
)

// Zpool is a ZFS zpool.  A pool is a top-level structure in ZFS, and can
// contain many descendent datasets.
type Zpool struct {
	Name          string
	Health        string
	Allocated     uint64
	Size          uint64
	Free          uint64
	Fragmentation uint64
	ReadOnly      bool
	Freeing       uint64
	Leaked        uint64
	DedupRatio    float64
}

// List of Zpool properties to retrieve from zpool list command on a non-Solaris platform
var zpoolPropList = []string{
	"name",
	"health",
	"allocated",
	"size",
	"free",
	"readonly",
	"dedupratio",
	"fragmentation",
	"freeing",
	"leaked",
}
var zpoolPropListOptions = strings.Join(zpoolPropList, ",")

// GetZpool retrieves a single ZFS zpool by name.
func GetZpool(name string) (*Zpool, error) {
	args := []string{"get", "-p", zpoolPropListOptions}
	args = append(args, name)
	c := command{Command: "zpool"}
	out, err := c.Run(args...)
	if err != nil {
		return nil, err
	}

	// there is no -H
	out = out[1:]

	z := &Zpool{Name: name}
	for _, line := range out {
		if err := z.parseLine(line); err != nil {
			return nil, err
		}
	}

	return z, nil
}

// ListZpools list all ZFS zpools accessible on the current system.
func ListZpools() ([]*Zpool, error) {
	args := []string{"list", "-Ho", "name"}
	c := command{Command: "zpool"}
	out, err := c.Run(args...)
	if err != nil {
		return nil, err
	}

	var pools []*Zpool

	for _, line := range out {
		z, err := GetZpool(line[0])
		if err != nil {
			return nil, err
		}
		pools = append(pools, z)
	}
	return pools, nil
}

func (z *Zpool) parseLine(line []string) error {
	prop := line[1]
	val := line[2]

	var err error

	switch prop {
	case "name":
		setString(&z.Name, val)
	case "health":
		setString(&z.Health, val)
	case "allocated":
		err = setUint(&z.Allocated, val)
	case "size":
		err = setUint(&z.Size, val)
	case "free":
		err = setUint(&z.Free, val)
	case "fragmentation":
		// Trim trailing "%" before parsing uint
		i := strings.Index(val, "%")
		if i < 0 {
			i = len(val)
		}
		err = setUint(&z.Fragmentation, val[:i])
	case "readonly":
		z.ReadOnly = val == "on"
	case "freeing":
		err = setUint(&z.Freeing, val)
	case "leaked":
		err = setUint(&z.Leaked, val)
	case "dedupratio":
		// Trim trailing "x" before parsing float64
		z.DedupRatio, err = strconv.ParseFloat(val[:len(val)-1], 64)
	}
	return err
}
