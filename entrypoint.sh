#!/bin/sh
set -e

if [ "$MIGRATE_ON_START" = "true" ]; then
	gojellyfin migrate
fi

exec gojellyfin "$@"
