#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_ROOT="$(cd "$PROJECT_DIR/.." && pwd)"
VENV_DIR="$PROJECT_DIR/.venv"

# ---------------------------------------------------------------------------
# 1) 找到一个可用的基础 Python 解释器（≥3.11，ARM64 原生优先）
# ---------------------------------------------------------------------------
find_base_python() {
  # lora venv 里有 ARM64 原生 Python 3.11，优先用它来创建 venv
  local lora_py="$REPO_ROOT/lora/venv/bin/python3"
  if [ -x "$lora_py" ]; then echo "$lora_py"; return; fi

  # Homebrew
  if [ "$(uname -s)" = "Darwin" ]; then
    for c in /opt/homebrew/bin/python3.{13,12,11} /usr/local/bin/python3.{13,12,11}; do
      if [ -x "$c" ]; then echo "$c"; return; fi
    done
  fi

  # PATH
  for n in python3.13 python3.12 python3.11 python3; do
    if command -v "$n" >/dev/null 2>&1; then command -v "$n"; return; fi
  done
}

# ---------------------------------------------------------------------------
# 2) 确保本项目有自己的 venv（带 pip + 所有依赖）
# ---------------------------------------------------------------------------
ensure_venv() {
  if [ -x "$VENV_DIR/bin/python3" ]; then return; fi

  local base_py
  base_py="$(find_base_python)"
  echo "[start] creating venv with: $base_py"
  "$base_py" -m venv "$VENV_DIR"
  "$VENV_DIR/bin/python3" -m ensurepip --upgrade 2>/dev/null || true
}

ensure_venv
PYTHON="$VENV_DIR/bin/python3"
export PYTHON

PORT="${PORT:-8080}"
URL="http://127.0.0.1:${PORT}/studio"

cd "$PROJECT_DIR"

echo "[start] using python: ${PYTHON}"
echo "[start] checking python dependencies..."
if ! "$PYTHON" -c "import pydantic, langchain_core, langgraph, torch, transformers" >/dev/null 2>&1; then
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
