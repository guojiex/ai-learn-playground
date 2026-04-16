from __future__ import annotations

import sys
from pathlib import Path

from bridge.protocol import dump_response, load_request
from bridge.runtime import build_runtime, dispatch


def main() -> None:
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
