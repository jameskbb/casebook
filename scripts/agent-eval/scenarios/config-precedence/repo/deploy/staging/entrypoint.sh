#!/bin/sh
# Container entrypoint for svc. The platform injects PORT for its health
# checks; keep the service on it.
set -eu
exec /usr/local/bin/svc -config /etc/svc/svc.conf -port "${PORT:-8080}" "$@"
