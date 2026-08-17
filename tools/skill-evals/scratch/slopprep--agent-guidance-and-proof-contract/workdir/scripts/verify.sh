#!/usr/bin/env bash
set -euo pipefail
npm run typecheck
npm test
curl --fail --silent http://127.0.0.1:4317/health
