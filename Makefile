LOG ?= /tmp/gojellyfin.log

# The binary carries no default DSN, so the development one lives here. `?=`
# leaves an existing DATABASE_URL alone, which is how CI and a scratch database
# override it.
DATABASE_URL ?= postgres://localhost:5432/gojellyfin_development?sslmode=disable
export DATABASE_URL

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
	go run ./cmd/gojellyfin server 2>&1 | tee $(LOG)

.PHONY: test
test: build
	go test ./...

# Behind a tag so `make test` stays fast: this one creates a database, applies
# the migrations and boots the whole server. `-count 1` because it has side
# effects, so a cached pass would mean nothing ran.
.PHONY: e2e
e2e: build
	go test -tags e2e -count 1 -timeout 5m ./cmd/gojellyfin/

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
