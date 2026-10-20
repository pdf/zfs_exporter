# ZFS Exporter

[![Test](https://github.com/pdf/zfs_exporter/actions/workflows/test.yml/badge.svg)](https://github.com/pdf/zfs_exporter/actions/workflows/test.yml)
[![Release](https://github.com/pdf/zfs_exporter/actions/workflows/release.yml/badge.svg)](https://github.com/pdf/zfs_exporter/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/pdf/zfs_exporter)](https://goreportcard.com/report/github.com/pdf/zfs_exporter)
[![License](https://img.shields.io/badge/License-MIT-%23a31f34)](https://github.com/pdf/zfs_exporter/blob/master/LICENSE)

Prometheus exporter for ZFS (pools, filesystems, snapshots and volumes). Other implementations exist, however performance can be quite variable, producing occasional timeouts (and associated alerts). This exporter was built with a few features aimed at allowing users to avoid collecting more than they need to, and to ensure timeouts cannot occur, but that we eventually return useful data:

- **Pool selection** - allow the user to select which pools are collected
- **Multiple collectors** - allow the user to select which data types are collected (pools, filesystems, snapshots and volumes)
- **Property selection** - allow the user to select which properties are collected per data type (enabling only required properties will increase collector performance, by reducing metadata queries)
- **Collection deadline and caching** - if the collection duration exceeds the configured deadline, cached data from the last run will be returned for any metrics that have not yet been collected, and the current collection run will continue in the background. Collections will not run concurrently, so that when a system is running slowly, we don't compound the problem - if an existing collection is still running, cached data will be returned.

## Installation

Download the [latest release](https://github.com/pdf/zfs_exporter/releases/latest) for your platform, and unpack it somewhere on your filesystem.

You may also build the latest version using Go v1.11 - 1.17 via `go get`:

```bash
go get -u github.com/pdf/zfs_exporter
```

Installation can also be accomplished using `go install`:

```bash
version=latest # or a specific version tag
go install github.com/pdf/zfs_exporter/v2@$version
```

## Usage

```
usage: zfs_exporter [<flags>]


Flags:
  -h, --[no-]help                Show context-sensitive help (also try --help-long and --help-man).
      --[no-]collector.dataset-filesystem  
                                 Enable the dataset-filesystem collector (default: enabled)
      --properties.dataset-filesystem="available,logicalused,quota,referenced,used,usedbydataset,written"  
                                 Properties to include for the dataset-filesystem collector, comma-separated.
      --[no-]collector.dataset-snapshot  
                                 Enable the dataset-snapshot collector (default: disabled)
      --properties.dataset-snapshot="logicalused,referenced,used,written"  
                                 Properties to include for the dataset-snapshot collector, comma-separated.
      --[no-]collector.dataset-volume  
                                 Enable the dataset-volume collector (default: enabled)
      --properties.dataset-volume="available,logicalused,referenced,used,usedbydataset,volsize,written"  
                                 Properties to include for the dataset-volume collector, comma-separated.
      --[no-]collector.pool      Enable the pool collector (default: enabled)
      --properties.pool="allocated,dedupratio,fragmentation,free,freeing,health,leaked,readonly,size"  
                                 Properties to include for the pool collector, comma-separated.
      --web.telemetry-path="/metrics"  
                                 Path under which to expose metrics.
      --[no-]web.disable-exporter-metrics  
                                 Exclude metrics about the exporter itself (promhttp_*, process_*, go_*).
      --deadline=8s              Maximum duration that a collection should run before returning cached data. Should be set to a value shorter than your scrape timeout duration. The current collection run will continue and update the cache when
                                 complete (default: 8s)
      --pool=POOL ...            Name of the pool(s) to collect, repeat for multiple pools (default: all pools).
      --exclude=EXCLUDE ...      Exclude datasets/snapshots/volumes that match the provided regex (e.g. '^rpool/docker/'), may be specified multiple times.
      --host-root=HOST-ROOT      For containers: run zpool and zfs chrooted into this directory, where the host's root filesystem is mounted (the image sets /host). Unset (the default), they run from the exporter's own PATH as its own user. Linux
                                 only; needs CAP_SYS_CHROOT, CAP_SETUID and CAP_SETGID.
      --host-root.path="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/run/current-system/sw/bin"  
                                 With --host-root: colon-separated directories inside the host root searched for zpool and zfs.
      --host-root.uid=65534      With --host-root: user ID that zpool and zfs run as.
      --host-root.gid=65534      With --host-root: group ID that zpool and zfs run as.
      --[no-]web.systemd-socket  Use systemd socket activation listeners instead of port listeners (Linux only).
      --web.listen-address=:9134 ...  
                                 Addresses on which to expose metrics and web interface. Repeatable for multiple addresses. Examples: `:9100` or `[::1]:9100` for http, `vsock://:9100` for vsock
      --web.config.file=""       Path to configuration file that can enable TLS or authentication. See: https://github.com/prometheus/exporter-toolkit/blob/master/docs/web-configuration.md
      --log.level=info           Only log messages with the given severity or above. One of: [debug, info, warn, error]
      --log.format=logfmt        Output format of log messages. One of: [logfmt, json]
      --[no-]version             Show application version.
```

Collectors that are enabled by default can be negated by prefixing the flag with `--no-*`, ie:

```
zfs_exporter --no-collector.dataset-filesystem
```

## Container

The image runs on Linux only, and contains nothing but the exporter: with `--host-root=/host` (the image's default) it runs the host's own `zpool` and `zfs`, chrooted into the host's root filesystem, so the CLI always matches the host's kernel module whatever its ZFS version or distribution. Inside the chroot, each command runs as `nobody` (`--host-root.uid` and `--host-root.gid`), and `zpool` and `zfs` are found on `--host-root.path`, which covers `/usr/local`, merged and unmerged `/usr` layouts and NixOS.

The container needs:

- the host's root filesystem mounted read-only at `/host`, as a slave of the host's mounts (`rslave` in Docker, `mountPropagation: HostToContainer` in Kubernetes). With the default private mount, the container keeps a copy of every dataset mounted when it started, so a dataset the host later unmounts stays mounted in the container, and the host can't destroy it (`dataset is busy`) until the container restarts.
- access to `/dev/zfs`
- the capabilities `CAP_SYS_CHROOT`, `CAP_SETUID` and `CAP_SETGID`, which container runtimes grant by default; all others can be dropped

It doesn't need the host's PID namespace, and needs a privileged container only where there is no other way to grant `/dev/zfs` (see Kubernetes, below).

### Image

Each release publishes `ghcr.io/pdf/zfs_exporter` for `linux/amd64` and `linux/arm64`, tagged `vX.Y.Z`, `vX.Y`, `vX` and `latest`. Its build provenance is attested by GitHub, and can be checked with:

```console
gh attestation verify oci://ghcr.io/pdf/zfs_exporter:v2 --owner pdf
```

### Docker

```console
docker run -d --name zfs_exporter --restart unless-stopped --read-only --cap-drop ALL --cap-add SYS_CHROOT --cap-add SETUID --cap-add SETGID --security-opt no-new-privileges -v /:/host:ro,rslave --device /dev/zfs -p 9134:9134 ghcr.io/pdf/zfs_exporter:v2
```

Or with Compose:

```yaml
services:
  zfs_exporter:
    image: ghcr.io/pdf/zfs_exporter:v2
    restart: unless-stopped
    read_only: true
    cap_drop: [ALL]
    cap_add: [SYS_CHROOT, SETUID, SETGID]
    security_opt: [no-new-privileges]
    devices: [/dev/zfs]
    volumes:
      - /:/host:ro,rslave
    ports:
      - 9134:9134
```

Extra flags go after the image name (or in `command:`), e.g. `--pool=tank`.

### Kubernetes

Kubernetes has no equivalent of `--device`: a hostPath of `/dev/zfs` alone is refused by the container's device cgroup. There are two ways to run it, both shown in [examples/kubernetes/daemonset.yaml](examples/kubernetes/daemonset.yaml):

- **Privileged (recommended):** works on any cluster with nothing else installed, and is how comparable node agents run, such as OpenEBS ZFS LocalPV and prometheus-smartctl-exporter. The `zpool` and `zfs` commands still run as `nobody`.
- **With a device plugin:** where the namespace's Pod Security level must stay below `privileged`, expose `/dev/zfs` with a device plugin such as [generic-device-plugin](https://github.com/squat/generic-device-plugin), request its resource (e.g. `squat.ai/zfs: 1`), and drop all capabilities but the three above.

### Building the image

Following the Prometheus convention, the `Dockerfile` doesn't compile the exporter: it copies the binary that [promu](https://github.com/prometheus/promu) built into `.build/<os>-<arch>/`, so the image and the release tarballs carry the same build. With only a copy, building for other architectures needs no emulation. To build both platforms as the release does (`promu crossbuild` builds inside Prometheus's golang-builder container, so needs Docker):

```console
make promu
$(go env GOPATH)/bin/promu crossbuild -p linux/amd64 -p linux/arm64
docker buildx create --name multiarch --driver docker-container --use
docker buildx build --platform linux/amd64,linux/arm64 -t zfs_exporter .
```

Docker's default builder can only build several platforms at once with the containerd image store enabled, hence the separate builder.

For a quick local image on linux/amd64, without the builder container:

```console
make promu
$(go env GOPATH)/bin/promu build --prefix .build/linux-amd64
make docker DOCKER_ARCHS=amd64
```

This tags `ghcr.io/pdf/zfs_exporter-linux-amd64:<branch>`.

## TLS endpoint

**EXPERIMENTAL**

The exporter supports TLS via a new web configuration file.

```console
./zfs_exporter --web.config.file=web-config.yml
```

See the [exporter-toolkit https package](https://github.com/prometheus/exporter-toolkit/blob/v0.1.0/https/README.md) for more details.

## Caveats

The collector may need to be run as root on some platforms (ie - Linux prior to ZFS v0.7.0).

Whilst inspiration was taken from some of the alternative ZFS collectors, metric names may not be compatible.

## Alternatives

In no particular order, here are some alternative implementations:

- https://github.com/eliothedeman/zfs_exporter
- https://github.com/ncabatoff/zfs-exporter
- https://github.com/eripa/prometheus-zfs
