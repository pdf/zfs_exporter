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

You may also build the latest version using Go v1.11+ via `go get`:

```bash
go get -u github.com/pdf/zfs_exporter
```

## Usage

```
usage: zfs_exporter [<flags>]

Flags:
  -h, --help                 Show context-sensitive help (also try --help-long and --help-man).
      --collector.dataset-filesystem
                             Enable the dataset-filesystem collector (default: enabled)
      --properties.dataset-filesystem="available,logicalused,quota,referenced,used,usedbydataset,written"
                             Properties to include for the dataset-filesystem collector, comma-separated.
      --collector.dataset-snapshot
                             Enable the dataset-snapshot collector (default: disabled)
      --properties.dataset-snapshot="logicalused,referenced,used,written"
                             Properties to include for the dataset-snapshot collector, comma-separated.
      --collector.dataset-volume
                             Enable the dataset-volume collector (default: enabled)
      --properties.dataset-volume="available,logicalused,referenced,used,usedbydataset,volsize,written"
                             Properties to include for the dataset-volume collector, comma-separated.
      --collector.pool       Enable the pool collector (default: enabled)
      --properties.pool="allocated,dedupratio,fragmentation,free,freeing,health,leaked,readonly,size"
                             Properties to include for the pool collector, comma-separated.
      --web.listen-address=":9134"
                             Address on which to expose metrics and web interface.
      --web.telemetry-path="/metrics"
                             Path under which to expose metrics.
      --deadline=8s          Maximum duration that a collection should run before returning cached data. Should
                             be set to a value shorter than your scrape timeout duration. The current
                             collection run will continue and update the cache when complete (default: 8s)
      --pool=POOL ...        Name of the pool(s) to collect, repeat for multiple pools (default: all pools).
      --exclude=EXCLUDE ...  Exclude datasets/snapshots/volumes that match the provided regex (e.g.
                             '^rpool/docker/'), may be specified multiple times.
      --log.level=info       Only log messages with the given severity or above. One of: [debug, info, warn,
                             error]
      --log.format=logfmt    Output format of log messages. One of: [logfmt, json]
      --version              Show application version.
```

Collectors that are enabled by default can be negated by prefixing the flag with `--no-*`, ie:

```
zfs_exporter --no-collector.dataset-filesystem
```

## Caveats

The collector may need to be run as root on some platforms (ie - Linux prior to ZFS v0.7.0).

Whilst inspiration was taken from some of the alternative ZFS collectors, metric names may not be compatible.

## Alternatives

In no particular order, here are some alternative implementations:

- https://github.com/eliothedeman/zfs_exporter
- https://github.com/ncabatoff/zfs-exporter
- https://github.com/eripa/prometheus-zfs
