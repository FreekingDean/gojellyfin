LOG ?= /tmp/gojellyfin.log
WORKER_LOG ?= /tmp/gojellyfin-worker.log

# The binary carries no default DSN, so the development one lives here. `?=`
# leaves an existing DATABASE_URL alone, which is how CI and a scratch database
# override it.
DATABASE_URL ?= postgres://localhost:5432/gojellyfin_development?sslmode=disable
export DATABASE_URL

# The local Temporal dev server `make temporal` starts, and what `make worker`
# dials. Deliberately not exported: jobs.NewClient dials at construction, so a
# server told about a Temporal that is not running refuses to start, while one
# told nothing serves with background work off. Export it yourself to give
# `make dev` the ScheduledTasks API: TEMPORAL_HOSTPORT=localhost:7233 make dev
TEMPORAL_HOSTPORT ?= localhost:7233
TEMPORAL_NAMESPACE ?= gojellyfin_development

# gojellyfin's own build, stamped into internal/system. The Jellyfin API
# version is a different fact and comes from the vendored spec.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

STAMP := github.com/FreekingDean/gojellyfin/internal/system
LDFLAGS := -X $(STAMP).buildVersion=$(VERSION) -X $(STAMP).buildCommit=$(COMMIT) -X $(STAMP).buildDate=$(DATE)

.PHONY: generate
generate:
	go generate ./...

.PHONY: build
build: generate
	go build -ldflags "$(LDFLAGS)" ./...

.PHONY: run
run: build
	go run -ldflags "$(LDFLAGS)" ./cmd/gojellyfin server 2>&1 | tee $(LOG)

.PHONY: dev
dev: migrate build
	TEMPORAL_HOSTPORT=$(TEMPORAL_HOSTPORT) TEMPORAL_NAMESPACE=$(TEMPORAL_NAMESPACE) \
	CORS_ORIGINS="*" go run -ldflags "$(LDFLAGS)" ./cmd/gojellyfin server 2>&1 | tee $(LOG)

# brew install temporal, or https://temporal.download/cli.sh. Not `go install`:
# the CLI's go.mod carries replace directives, which go install refuses.
# Ephemeral: history lives in memory, so a restart is a clean slate. The web UI
# is on http://localhost:8233.
.PHONY: temporal
temporal:
	@command -v temporal >/dev/null || \
		(echo "temporal not found: brew install temporal" && exit 1)
	temporal server start-dev --ip 127.0.0.1 --namespace $(TEMPORAL_NAMESPACE)

.PHONY: worker
worker: build
	TEMPORAL_HOSTPORT=$(TEMPORAL_HOSTPORT) TEMPORAL_NAMESPACE=$(TEMPORAL_NAMESPACE) \
		go run -ldflags "$(LDFLAGS)" ./cmd/gojellyfin worker 2>&1 | tee $(WORKER_LOG)

.PHONY: migrate
migrate: build
	go run -ldflags "$(LDFLAGS)" ./cmd/gojellyfin migrate

.PHONY: test
test: build
	go test ./...

# Jest driving the real jellyfin-web client in Chrome. Apart from `make test`
# because it wants Node, Docker and a browser.
.PHONY: e2e
e2e:
	cd e2e && npm ci && npm test

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
