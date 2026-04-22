from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path


@dataclass
class CommissionLookupTool:
    """本地 demo：JSON 里有的 SKU 用配置值；否则用 default_mock_rate（含 fallback-unknown）。"""

    rates: dict[str, float]
    default_mock_rate: float = 0.15

    @classmethod
    def from_file(cls, path: Path) -> "CommissionLookupTool":
        return cls(rates=json.loads(path.read_text()))

    def lookup(self, product_id: str) -> float:
        if product_id in self.rates:
            return float(self.rates[product_id])
        return float(self.default_mock_rate)
