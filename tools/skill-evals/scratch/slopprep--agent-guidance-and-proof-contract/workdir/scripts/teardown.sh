#!/usr/bin/env bash
set -euo pipefail
pkill -f 'node server.js' 2>/dev/null || true
