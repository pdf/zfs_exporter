FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN go build -o ./zfs_exporter


FROM alpine:3.20

WORKDIR /app

RUN	apk update --no-cache \
	&& apk upgrade --no-cache \
	&& apk add zfs \
	&& rm -rf /var/cache/apk/*

COPY --from=builder /app/zfs_exporter /app/

ENTRYPOINT ["/app/zfs_exporter"]
