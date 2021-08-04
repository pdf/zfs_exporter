package zfs

import (
	"errors"
	"strings"
)

// ZFS dataset types, which can indicate if a dataset is a filesystem,
// snapshot, or volume.
const (
	DatasetFilesystem = "filesystem"
	DatasetSnapshot   = "snapshot"
	DatasetVolume     = "volume"
)

// Dataset is a ZFS dataset.  A dataset could be a clone, filesystem, snapshot,
// or volume.  The Type struct member can be used to determine a dataset's type.
//
// The field definitions can be found in the ZFS manual:
// http://www.freebsd.org/cgi/man.cgi?zfs(8).
type Dataset struct {
	Name          string
	Origin        string
	Used          uint64
	Avail         uint64
	Mountpoint    string
	Compression   string
	Type          string
	Written       uint64
	Volsize       uint64
	Logicalused   uint64
	Usedbydataset uint64
	Quota         uint64
	Referenced    uint64
}

// List of ZFS properties to retrieve from zfs list command
var dsPropList = []string{
	"name",
	"origin",
	"used",
	"available",
	"mountpoint",
	"compression",
	"type",
	"volsize",
	"quota",
	"referenced",
	"written",
	"logicalused",
	"usedbydataset",
}
var dsPropListOptions = strings.Join(dsPropList, ",")

// Filesystems returns a slice of ZFS filesystems.
// A filter argument may be passed to select a filesystem with the matching name,
// or empty string ("") may be used to select all filesystems.
func Filesystems(filter string) ([]*Dataset, error) {
	return listByType(DatasetFilesystem, filter)
}

// Snapshots returns a slice of ZFS snapshots.
// A filter argument may be passed to select a snapshot with the matching name,
// or empty string ("") may be used to select all snapshots.
func Snapshots(filter string) ([]*Dataset, error) {
	return listByType(DatasetSnapshot, filter)
}

// Volumes returns a slice of ZFS volumes.
// A filter argument may be passed to select a volume with the matching name,
// or empty string ("") may be used to select all volumes.
func Volumes(filter string) ([]*Dataset, error) {
	return listByType(DatasetVolume, filter)
}

func listByType(t, filter string) ([]*Dataset, error) {
	args := []string{"list", "-rHp", "-t", t, "-o", dsPropListOptions}

	if filter != "" {
		args = append(args, filter)
	}
	c := command{Command: "zfs"}
	out, err := c.Run(args...)
	if err != nil {
		return nil, err
	}

	var datasets []*Dataset

	name := ""
	var ds *Dataset
	for _, line := range out {
		if name != line[0] {
			name = line[0]
			ds = &Dataset{Name: name}
			datasets = append(datasets, ds)
		}
		if err := ds.parseLine(line); err != nil {
			return nil, err
		}
	}

	return datasets, nil
}

func (ds *Dataset) parseLine(line []string) error {
	var err error

	if len(line) != len(dsPropList) {
		return errors.New("Output does not match what is expected on this platform")
	}
	setString(&ds.Name, line[0])
	setString(&ds.Origin, line[1])

	if err = setUint(&ds.Used, line[2]); err != nil {
		return err
	}
	if err = setUint(&ds.Avail, line[3]); err != nil {
		return err
	}

	setString(&ds.Mountpoint, line[4])
	setString(&ds.Compression, line[5])
	setString(&ds.Type, line[6])

	if err = setUint(&ds.Volsize, line[7]); err != nil {
		return err
	}
	if err = setUint(&ds.Quota, line[8]); err != nil {
		return err
	}
	if err = setUint(&ds.Referenced, line[9]); err != nil {
		return err
	}

	if err = setUint(&ds.Written, line[10]); err != nil {
		return err
	}
	if err = setUint(&ds.Logicalused, line[11]); err != nil {
		return err
	}
	if err = setUint(&ds.Usedbydataset, line[12]); err != nil {
		return err
	}

	return nil
}
