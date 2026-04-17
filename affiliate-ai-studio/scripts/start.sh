#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PYTHON="${PYTHON:-python3}"
PORT="${PORT:-8080}"
URL="http://127.0.0.1:${PORT}/studio"

cd "$PROJECT_DIR"

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

echo "[start] launching Go web server on :${PORT}..."
cd go
open_browser &
OPEN_PID=$!
trap 'kill "$OPEN_PID" 2>/dev/null || true' EXIT
exec go run ./cmd/studio
