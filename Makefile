LOG ?= /tmp/gojellyfin.log

.PHONY: dev
dev:
	air 2>&1 | tee $(LOG)

.PHONY: generate
generate:
	go generate ./...

.PHONY: build
build: generate
	go build ./...

.PHONY: run
run: build
	go run ./cmd/server 2>&1 | tee $(LOG)

.PHONY: test
test: build
	go test ./...

.PHONY: fmt
fmt:
	gofmt -w internal cmd
