package zfs

// DefaultHostPath is where NewHostRoot looks for zpool and zfs inside the
// host root: /usr/local, merged and unmerged /usr and sbin layouts, and NixOS.
var DefaultHostPath = []string{
	`/usr/local/sbin`,
	`/usr/local/bin`,
	`/usr/sbin`,
	`/usr/bin`,
	`/sbin`,
	`/bin`,
	`/run/current-system/sw/bin`,
}

// HostRootConfig runs the host's own ZFS CLI from inside a container, so the
// CLI always matches the host's kernel module.
type HostRootConfig struct {
	// Root is the host's root filesystem as mounted in the container.
	Root string
	// Path is searched, in order, for zpool and zfs inside Root.
	Path []string
	// UID and GID the commands run as inside Root.
	UID, GID uint32
}

// HostRootInfo describes the CLI NewHostRoot found.
type HostRootInfo struct {
	Zpool, ZFS string
	// Version is the output of `zpool version`: the host's userland and
	// kernel module versions.
	Version string
}
