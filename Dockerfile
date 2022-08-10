FROM golang:1.18-alpine

RUN apk --update-cache add zfs

WORKDIR /opt/zfs_exporter

COPY go.mod go.sum ./

RUN go mod download && go mod verify

COPY . .

RUN go build -v -o /usr/local/bin/zfs_exporter

ENTRYPOINT ["zfs_exporter"]
