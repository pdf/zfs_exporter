# The exporter alone: it runs the host's own zpool and zfs, chrooted into
# the host root mounted at /host, so the image never depends on the host's
# ZFS version. See the README's Container section for the required mount,
# device and capabilities.
#
# As in Prometheus's own exporters, this doesn't compile anything: it copies
# the binary promu already built into .build/<os>-<arch>/, so the image
# carries the same build as the release tarballs. Build that first (README,
# Building the image), then run `make docker`.
ARG ARCH="amd64"
ARG OS="linux"
FROM scratch
ARG ARCH="amd64"
ARG OS="linux"
COPY .build/${OS}-${ARCH}/zfs_exporter /bin/zfs_exporter
COPY LICENSE /LICENSE

EXPOSE 9134
ENTRYPOINT ["/bin/zfs_exporter", "--host-root=/host"]
