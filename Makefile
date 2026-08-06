LOG ?= /tmp/gojellyfin.log

.PHONY: dev run build test generate fmt kill

# Watch and restart, with the log on screen and in $(LOG) for tooling to read.
dev:
	air 2>&1 | tee $(LOG)

run:
	go run ./cmd/server 2>&1 | tee $(LOG)

build:
	go build ./...

test:
	go test ./...

# Re-emits ~95k lines; only needed when the spec or the gen tool changes.
generate:
	go generate ./internal/server/api

fmt:
	gofmt -w internal cmd

# Only the listener: without -sTCP:LISTEN this also matches clients connected
# to the port, such as a browser.
kill:
	@pids=$$(lsof -ti:8081 -sTCP:LISTEN); \
	if [ -n "$$pids" ]; then kill -9 $$pids && echo "killed $$pids"; else echo "nothing listening on :8081"; fi
