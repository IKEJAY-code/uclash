VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  = -s -w \
	-X gitee.com/IKEJAY-code/uclash/internal/version.Version=$(VERSION) \
	-X gitee.com/IKEJAY-code/uclash/internal/version.Commit=$(COMMIT) \
	-X gitee.com/IKEJAY-code/uclash/internal/version.Date=$(DATE)

.PHONY: build linux darwin release vet test clean

build:
	go build -ldflags "$(LDFLAGS)" -o dist/uclash ./cmd/uclash

linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/uclash-linux-amd64 ./cmd/uclash
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/uclash-linux-arm64 ./cmd/uclash

darwin:
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/uclash-darwin-amd64 ./cmd/uclash
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/uclash-darwin-arm64 ./cmd/uclash

release: linux darwin

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -rf dist
