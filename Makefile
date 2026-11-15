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
VERSION ?= $(patsubst v%,%,$(shell git describe --tags --always --dirty 2>/dev/null || echo dev))
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

STAMP := github.com/FreekingDean/gojellyfin/internal/system
LDFLAGS := -X $(STAMP).buildVersion=$(VERSION) -X $(STAMP).buildCommit=$(COMMIT) -X $(STAMP).buildDate=$(DATE)

.PHONY: generate
generate:
	go generate ./...

# The admin app the binary embeds and serves at /admin. A file target, so it
# costs an npm run only when something under admin/ or the spec changed. A
# plain `go build` without it still compiles, and /admin answers 503.
ADMIN_APP := internal/server/admin/dist/index.html
ADMIN_SOURCES := $(shell find admin -path admin/node_modules -prune -o -type f -print) spec/jellyfin-openapi-stable.patched.json

.PHONY: admin
admin: $(ADMIN_APP)

$(ADMIN_APP): $(ADMIN_SOURCES)
	cd admin && npm ci && npm run build

.PHONY: build
build: generate admin
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

# Playwright driving the real jellyfin-web client. Apart from `make test`
# because it wants Node, Docker, ffmpeg and a browser. `e2e` renders in the
# pinned Playwright image and compares screenshots, as CI does; `e2e-update`
# rewrites the baselines from that same image; `e2e-local` uses a browser on
# this machine and skips the pixels.
.PHONY: e2e
e2e:
	cd e2e && npm ci && npm run visual

.PHONY: e2e-update
e2e-update:
	cd e2e && npm ci && npm run visual:update

.PHONY: e2e-local
e2e-local:
	cd e2e && npm ci && npx playwright install chromium && npm test

# The stock Android TV app, driven over adb. Wants a running emulator or a
# television `adb devices` lists, on top of what `e2e` wants.
.PHONY: e2e-androidtv
e2e-androidtv:
	cd e2e && npm ci && npm run androidtv

# The stock Android phone app, the same way, on a phone emulator.
.PHONY: e2e-android
e2e-android:
	cd e2e && npm ci && npm run android

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
