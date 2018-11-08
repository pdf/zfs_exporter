include Makefile.common

.PHONY: build
build: export GO111MODULE=on
build: common-build