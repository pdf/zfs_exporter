package zfs

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"strings"
	"syscall"
)

type hostRootRunner struct {
	cfg  HostRootConfig
	bins map[string]string
}

// NewHostRoot returns a Client that runs each ZFS command chrooted into
// cfg.Root as cfg.UID and cfg.GID. It finds zpool and zfs by running each
// candidate in cfg.Path inside the chroot, so symlinks resolve against the
// host (NixOS links into /nix/store), and checks that zpool can list pools.
func NewHostRoot(cfg HostRootConfig) (Client, HostRootInfo, error) {
	r := &hostRootRunner{cfg: cfg, bins: map[string]string{}}
	var info HostRootInfo
	for _, name := range []string{`zpool`, `zfs`} {
		path, out, err := r.find(name)
		if err != nil {
			return nil, info, err
		}
		r.bins[name] = path
		if name == `zpool` {
			info.Zpool, info.Version = path, strings.TrimSpace(out)
		} else {
			info.ZFS = path
		}
	}
	c := clientImpl{run: r, sysfsRoot: cfg.Root}
	if _, err := c.PoolNames(); err != nil {
		return nil, info, fmt.Errorf("listing pools in host root %s failed, is /dev/zfs available to the container? %w", cfg.Root, err)
	}
	return c, info, nil
}

func (r *hostRootRunner) command(name string, args ...string) *exec.Cmd {
	path, ok := r.bins[name]
	if !ok {
		path = name
	}
	return r.exec(path, args...)
}

func (r *hostRootRunner) exec(path string, args ...string) *exec.Cmd {
	return &exec.Cmd{
		Path: path,
		Args: append([]string{path}, args...),
		Env:  []string{`PATH=` + strings.Join(r.cfg.Path, `:`)},
		Dir:  `/`,
		SysProcAttr: &syscall.SysProcAttr{
			Chroot:     r.cfg.Root,
			Credential: &syscall.Credential{Uid: r.cfg.UID, Gid: r.cfg.GID, Groups: []uint32{}},
		},
	}
}

// find runs `<dir>/<name> version` for each directory in the search path and
// returns the first that the kernel can execute, with its output. A non-zero
// exit still counts: older releases have no version subcommand.
func (r *hostRootRunner) find(name string) (string, string, error) {
	for _, dir := range r.cfg.Path {
		path := strings.TrimSuffix(dir, `/`) + `/` + name
		c := r.exec(path, `version`)
		var out bytes.Buffer
		c.Stdout, c.Stderr = &out, &out
		err := c.Run()
		var exit *exec.ExitError
		switch {
		case err == nil, errors.As(err, &exit):
			return path, out.String(), nil
		case errors.Is(err, fs.ErrNotExist), errors.Is(err, syscall.ENOTDIR):
			continue
		case errors.Is(err, fs.ErrPermission) && !errors.Is(err, syscall.EPERM):
			return ``, ``, fmt.Errorf("running %s in host root %s as %d:%d was refused: %w", path, r.cfg.Root, r.cfg.UID, r.cfg.GID, err)
		default:
			return ``, ``, fmt.Errorf("running %s in host root %s failed, does the container have CAP_SYS_CHROOT, CAP_SETUID and CAP_SETGID? %w", path, r.cfg.Root, err)
		}
	}
	return ``, ``, fmt.Errorf("%s not found in host root %s under %s", name, r.cfg.Root, strings.Join(r.cfg.Path, `:`))
}
