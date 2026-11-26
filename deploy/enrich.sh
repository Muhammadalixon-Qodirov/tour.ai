#!/usr/bin/env bash
# Joylarni ochiq manbalardan boyitish: Commons rasmlari + Vikipediya tavsiflari.
#   bash enrich.sh [--dry-run] [--limit N] [--no-geo] [--no-desc]
set -euo pipefail
cd /opt/yoldosh/deploy
env -i PATH="$PATH" HOME="$HOME" docker compose run --rm -T --entrypoint /app/bin/enrich api "$@"
