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

# Pinned to the version .github/workflows/ci.yml installs, so a green run here
# means a green run there.
GOLANGCI_VERSION ?= v2.12.2

.PHONY: lint
lint:
	@test -z "$$(gofmt -l internal cmd)" || \
		(echo "gofmt: $$(gofmt -l internal cmd | tr '\n' ' ')" && echo "run make fmt" && exit 1)
	go vet ./...
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION) run
