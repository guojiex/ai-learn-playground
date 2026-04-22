#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Use PYTHON for both pip and the Go worker subprocess. On macOS, prefer a
# Homebrew Python (linked against OpenSSL) over /usr/bin/python3 (LibreSSL),
# which triggers urllib3's NotOpenSSLWarning.
if [ -z "${PYTHON:-}" ]; then
  if [ "$(uname -s)" = "Darwin" ]; then
    for candidate in \
      /opt/homebrew/bin/python3.13 \
      /opt/homebrew/bin/python3.12 \
      /opt/homebrew/bin/python3.11 \
      /usr/local/bin/python3.13 \
      /usr/local/bin/python3.12 \
      /usr/local/bin/python3.11; do
      if [ -x "$candidate" ]; then
        PYTHON="$candidate"
        break
      fi
    done
  fi
  if [ -z "${PYTHON:-}" ]; then
    for name in python3.13 python3.12 python3.11; do
      if command -v "$name" >/dev/null 2>&1; then
        PYTHON="$(command -v "$name")"
        break
      fi
    done
  fi
fi
PYTHON="${PYTHON:-python3}"
export PYTHON

PORT="${PORT:-8080}"
URL="http://127.0.0.1:${PORT}/studio"

cd "$PROJECT_DIR"

echo "[start] using python: ${PYTHON}"
echo "[start] checking python dependencies..."
if ! "$PYTHON" -c "import pydantic, langchain_core, langgraph" >/dev/null 2>&1; then
  echo "[start] missing deps, installing from python/requirements.txt"
  "$PYTHON" -m pip install -q -r python/requirements.txt
else
  echo "[start] python dependencies ok"
fi

open_browser() {
  for _ in $(seq 1 40); do
    if curl -fs "http://127.0.0.1:${PORT}/healthz" >/dev/null 2>&1; then
      echo "[start] server is up, opening ${URL}"
      if command -v open >/dev/null 2>&1; then
        open "$URL" || true
      elif command -v xdg-open >/dev/null 2>&1; then
        xdg-open "$URL" >/dev/null 2>&1 || true
      fi
      return
    fi
    sleep 0.25
  done
  echo "[start] server did not become ready in time; open ${URL} manually"
}

echo "[start] building frontend (web-studio)..."
cd web-studio
if [ ! -d node_modules ]; then
  npm install
fi
npm run build
cd "$PROJECT_DIR"

echo "[start] launching Go web server on :${PORT}..."
cd go
open_browser &
OPEN_PID=$!
trap 'kill "$OPEN_PID" 2>/dev/null || true' EXIT
exec go run ./cmd/studio
