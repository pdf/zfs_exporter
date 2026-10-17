//go:build !linux

package zfs

import "errors"

// NewHostRoot is only supported on Linux.
func NewHostRoot(cfg HostRootConfig) (Client, HostRootInfo, error) {
	return nil, HostRootInfo{}, errors.New(`host root mode is only supported on Linux`)
}
