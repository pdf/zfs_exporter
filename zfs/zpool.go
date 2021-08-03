package zfs

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

// zpool is a helper function to wrap typical calls to zpool.
func zpool(arg ...string) ([][]string, error) {
	c := command{Command: "zpool"}
	return c.Run(arg...)
}

// GetZpool retrieves a single ZFS zpool by name.
func GetZpool(name string) (*Zpool, error) {
	args := zpoolArgs
	args = append(args, name)
	out, err := zpool(args...)
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
	out, err := zpool(args...)
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
