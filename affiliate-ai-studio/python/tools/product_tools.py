from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path


@dataclass
class ProductLookupTool:
    fixtures: list[dict]

    @classmethod
    def from_file(cls, path: Path) -> "ProductLookupTool":
        return cls(fixtures=json.loads(path.read_text()))

    def lookup(self, product_url: str, product_text: str) -> dict:
        if product_url:
            for fixture in self.fixtures:
                if fixture.get("url") == product_url:
                    return fixture
        if product_text:
            lowered = product_text.lower()
            for fixture in self.fixtures:
                if fixture.get("title", "").lower() in lowered or lowered in fixture.get("title", "").lower():
                    return fixture
        raise LookupError("no matching product fixture found")
