#!/bin/sh
set -e

if [ "$MIGRATE_ON_START" = "true" ]; then
	migrate
fi

exec "$@"
