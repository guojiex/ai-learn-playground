from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path


@dataclass
class CommissionLookupTool:
    rates: dict[str, float]

    @classmethod
    def from_file(cls, path: Path) -> "CommissionLookupTool":
        return cls(rates=json.loads(path.read_text()))

    def lookup(self, product_id: str) -> float:
        return float(self.rates.get(product_id, 0.0))
