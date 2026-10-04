#!/usr/bin/env bash
set -euo pipefail

export GOCACHE="${GOCACHE:-${TMPDIR:-/tmp}/yub-wpanel-go-build-cache}"
mkdir -p "$GOCACHE"

go test ./...
go vet ./...
go build -o "${TMPDIR:-/tmp}/yub-wpanel-verify" ./cmd/yub-wpanel
git diff --check

if command -v php >/dev/null 2>&1; then
  php -l web/plugins/yub-wpanel-optimizer/yub-wpanel-optimizer.php
else
  echo "php not found; skipped php -l web/plugins/yub-wpanel-optimizer/yub-wpanel-optimizer.php"
fi
