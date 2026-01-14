#!/usr/bin/env bash
set -euo pipefail

# Railway provides PORT; default is for local dev.
export PORT="${PORT:-8081}"

# These must be provided in Railway Variables:
# - POSTGRES_DSN
# - API_KEY

exec go run .
