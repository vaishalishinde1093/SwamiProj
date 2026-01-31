#!/usr/bin/env bash
set -euo pipefail

# Railway provides PORT; default is for local dev.
export PORT="${PORT:-8081}"

# These must be provided in Railway Variables:
# - POSTGRES_DSN
# - API_KEY

: "${API_KEY:?API_KEY is required}"
: "${ADMIN_PASSWORD_HASH:?ADMIN_PASSWORD_HASH is required}"
: "${ADMIN_SESSION_SECRET:?ADMIN_SESSION_SECRET is required}"

exec ./app IMPORT_MEMBERS=1
