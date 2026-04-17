from __future__ import annotations

import logging
import os
import sys
from pathlib import Path

from bridge.protocol import dump_response, load_request
from bridge.runtime import build_runtime, dispatch


def _configure_logging() -> None:
    level_name = os.getenv("AFFILIATE_LOG_LEVEL", "INFO").upper()
    level = getattr(logging, level_name, logging.INFO)
    handler = logging.StreamHandler(stream=sys.stderr)
    handler.setFormatter(
        logging.Formatter(
            "%(asctime)s %(levelname)s %(name)s %(message)s",
            datefmt="%Y-%m-%dT%H:%M:%S",
        )
    )
    root = logging.getLogger()
    root.handlers.clear()
    root.addHandler(handler)
    root.setLevel(level)


def main() -> None:
    _configure_logging()
    project_root = Path(__file__).resolve().parents[1]
    runtime = build_runtime(project_root)
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        request = load_request(line)
        response = dispatch(runtime, request)
        sys.stdout.write(dump_response(response))
        sys.stdout.flush()


if __name__ == "__main__":
    main()
