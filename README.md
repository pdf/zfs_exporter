# ZFS Exporter

Prometheus exporter for ZFS (pools, filesystems, snapshots and volumes). Other implementations exist, however performance can be quite variable, producing occasional timeouts (and associated alerts). This exporter was built with a few features aimed at allowing users to avoid collecting more than they need to, and to ensure timeouts cannot occur, but that we eventually return useful data:

- __Pool selection__ - allow the user to select which pools are collected
- __Multiple collectors__ - allow the user to select which data types are collected (pools, filesystems, snapshots and volumes)
- __Collection deadline and caching__ - if the collection duration exceeds the configured deadline, cached data from the last run will be returned for any metrics that have not yet been collected, and the current collection run will continue in the background.  Collections will not run concurrently, so that when a system is running slowly, we don't compound the problem - if an existing collection is still running, cached data will be returned.

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
  -h, --help               Show context-sensitive help (also try --help-long and --help-man).
      --collector.dataset-filesystem
                           Enable the dataset-filesystem collector (default: enabled)
      --collector.dataset-snapshot
                           Enable the dataset-snapshot collector (default: disabled)
      --collector.dataset-volume
                           Enable the dataset-volume collector (default: enabled)
      --collector.pool     Enable the pool collector (default: enabled)
      --web.listen-address=":9134"
                           Address on which to expose metrics and web interface.
      --web.telemetry-path="/metrics"
                           Path under which to expose metrics.
      --deadline=8s        Maximum duration that a collection should run before returning cached data. Should be set to a value shorter than your
                           scrape timeout duration. The current collection run will continue and update the cache when complete (default: 8s)
      --pool=POOL ...      Name of the pool(s) to collect, repeat for multiple pools (default: all pools).
      --ignore=IGNORE ...  Regex to match datasets/volumes to ignore
      --log.level="info"   Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal]
      --log.format="logger:stderr"
                           Set the log target and format. Example: "logger:syslog?appname=bob&local=7" or "logger:stdout?json=true"
      --version            Show application version.
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
