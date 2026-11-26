#!/usr/bin/env bash
# Joy topish buyruqlari: bash discover.sh wikidata|research|stats [--dry-run]
set -euo pipefail
cd /opt/yoldosh/deploy
env -i PATH="$PATH" HOME="$HOME" docker compose run --rm -T --entrypoint /app/bin/discover api "$@"
