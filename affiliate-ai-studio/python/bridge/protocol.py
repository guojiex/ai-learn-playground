from __future__ import annotations

import json
from typing import Any


def load_request(line: str) -> dict[str, Any]:
    return json.loads(line)


def dump_response(payload: dict[str, Any]) -> str:
    return json.dumps(payload, ensure_ascii=False) + "\n"
