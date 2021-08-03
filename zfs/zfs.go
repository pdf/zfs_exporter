package zfs

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

// zfs is a helper function to wrap typical calls to zfs.
func zfs(arg ...string) ([][]string, error) {
	c := command{Command: "zfs"}
	return c.Run(arg...)
}

// Snapshots returns a slice of ZFS snapshots.
// A filter argument may be passed to select a snapshot with the matching name,
// or empty string ("") may be used to select all snapshots.
func Snapshots(filter string) ([]*Dataset, error) {
	return listByType(DatasetSnapshot, filter)
}

// Filesystems returns a slice of ZFS filesystems.
// A filter argument may be passed to select a filesystem with the matching name,
// or empty string ("") may be used to select all filesystems.
func Filesystems(filter string) ([]*Dataset, error) {
	return listByType(DatasetFilesystem, filter)
}

// Volumes returns a slice of ZFS volumes.
// A filter argument may be passed to select a volume with the matching name,
// or empty string ("") may be used to select all volumes.
func Volumes(filter string) ([]*Dataset, error) {
	return listByType(DatasetVolume, filter)
}
