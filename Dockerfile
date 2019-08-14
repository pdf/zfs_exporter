FROM golang:alpine AS build

WORKDIR /zfs_exporter
ADD . .
RUN apk --no-cache add make git && \
    make promu
RUN make build

FROM alpine

RUN apk --no-cache add zfs-libs

COPY --from=build /zfs_exporter/zfs_exporter /zfs_exporter

ENTRYPOINT ["/zfs_exporter"]
