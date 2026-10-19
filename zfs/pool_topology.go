package zfs

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// PoolVdev describes usable ZFS pool-vdev accounting. SizeBytes is the
// vdev's allocatable size, NOT the sum of its physical member capacities.
type PoolVdev struct {
	Name           string
	SizeBytes      uint64
	AllocatedBytes uint64
	FreeBytes      uint64
}

// PoolDisk reports the raw capacity of a Linux block device (or partition)
// participating in the named vdev. ZFS does not report per-leaf ALLOC/FREE
// under RAIDZ or mirror: see PoolVdev for consumption.
type PoolDisk struct {
	Vdev      string
	Device    string
	SizeBytes uint64
}

type PoolTopology struct {
	Vdevs []PoolVdev
	Disks []PoolDisk
}

// PoolTopology queries the host's ZFS CLI, including under --host-root.
// -H: tab-separated script output, -p: exact bytes, -L -P: resolved paths.
func (z clientImpl) PoolTopology(pool string) (PoolTopology, error) {
	command := z.run.command("zpool", "list", "-v", "-H", "-p", "-L", "-P", "-o", "name,size,allocated,free", pool)
	output, err := command.CombinedOutput()
	if err != nil {
		return PoolTopology{}, fmt.Errorf("zpool list -v %s: %w: %s", pool, err, strings.TrimSpace(string(output)))
	}
	root := z.sysfsRoot
	if root == "" {
		root = "/"
	}
	return parsePoolTopology(strings.NewReader(string(output)), pool, root)
}

type topologyLevel struct {
	indent int
	vdev   string
}

// parsePoolTopology keeps the indentation hierarchy from zpool list -v -H.
// In scripted output leading tabs in the first column represent descendants.
func parsePoolTopology(r io.Reader, pool, hostRoot string) (PoolTopology, error) {
	var topology PoolTopology
	var stack []topologyLevel
	seenVdev := make(map[string]bool)
	seenDisk := make(map[string]bool)
	foundPool := false
	lineNumber := 0

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		trimmed := strings.TrimLeft(line, " \t")
		prefix := line[:len(line)-len(trimmed)]
		indent := strings.Count(prefix, "\t")
		if indent == 0 {
			indent = len(prefix) / 2
		}
		columns := strings.Split(trimmed, "\t")
		// The top-level pool row respects -o, but some OpenZFS versions
		// print additional columns for leaf devices with zpool list -v.
		// The first four fields are always NAME, SIZE, ALLOC and FREE
		// for this command. Ignore trailing optional fields.
		if len(columns) < 4 {
			return PoolTopology{}, fmt.Errorf("invalid zpool list output at line %d: need at least 4 columns, got %d", lineNumber, len(columns))
		}
		name := strings.TrimSpace(columns[0])
		if name == "" {
			return PoolTopology{}, fmt.Errorf("empty name at line %d", lineNumber)
		}

		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		parentVdev := ""
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i].vdev != "" {
				parentVdev = stack[i].vdev
				break
			}
		}
		if indent == 0 {
			if name != pool || foundPool {
				return PoolTopology{}, fmt.Errorf("unexpected pool name %q at line %d", name, lineNumber)
			}
			foundPool = true
			stack = append(stack, topologyLevel{indent: indent})
			continue
		}
		if !foundPool {
			return PoolTopology{}, fmt.Errorf("vdev before pool at line %d", lineNumber)
		}

		// Leaf block devices are recognizable after -L -P. This also
		// distinguishes disk-based vdevs from groups (logs/spares/cache).
		isDisk := strings.HasPrefix(name, "/dev/")
		isVdev := false
		if columns[1] != "-" && columns[2] != "-" && columns[3] != "-" {
			size, e1 := strconv.ParseUint(columns[1], 10, 64)
			allocated, e2 := strconv.ParseUint(columns[2], 10, 64)
			free, e3 := strconv.ParseUint(columns[3], 10, 64)
			if e1 != nil || e2 != nil || e3 != nil {
				return PoolTopology{}, fmt.Errorf("invalid VDEV capacity at line %d: %q", lineNumber, line)
			}
			isVdev = true
			if seenVdev[name] {
				return PoolTopology{}, fmt.Errorf("duplicate VDEV name %q in pool %s", name, pool)
			}
			seenVdev[name] = true
			topology.Vdevs = append(topology.Vdevs, PoolVdev{
				Name: name, SizeBytes: size, AllocatedBytes: allocated, FreeBytes: free,
			})
		}

		if isDisk {
			vdev := parentVdev
			if isVdev { // a single disk/partition can itself be the top-level VDEV
				vdev = name
			}
			if vdev == "" {
				return PoolTopology{}, fmt.Errorf("disk %q has no allocatable VDEV ancestor", name)
			}
			key := vdev + "\x00" + name
			if !seenDisk[key] {
				bytes, err := hostBlockDeviceBytes(hostRoot, name)
				if err != nil {
					return PoolTopology{}, fmt.Errorf("pool %s device %s: %w", pool, name, err)
				}
				seenDisk[key] = true
				topology.Disks = append(topology.Disks, PoolDisk{
					Vdev: vdev, Device: name, SizeBytes: bytes,
				})
			}
		}
		next := topologyLevel{indent: indent}
		if isVdev {
			next.vdev = name
		}
		stack = append(stack, next)
	}
	if err := scanner.Err(); err != nil {
		return PoolTopology{}, err
	}
	if !foundPool {
		return PoolTopology{}, fmt.Errorf("no pool row for %q", pool)
	}
	return topology, nil
}

func hostBlockDeviceBytes(hostRoot, devicePath string) (uint64, error) {
	name := filepath.Base(devicePath)
	if name == "." || name == "/" || name == "" {
		return 0, fmt.Errorf("invalid block device %q", devicePath)
	}
	// sysfs 'size' is measured in 512-byte units even on 4KiB-sector disks.
	path := filepath.Join(hostRoot, "sys", "class", "block", name, "size")
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", path, err)
	}
	sectors, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", path, err)
	}
	if sectors > math.MaxUint64/512 {
		return 0, fmt.Errorf("size overflow for %s", devicePath)
	}
	return sectors * 512, nil
}
