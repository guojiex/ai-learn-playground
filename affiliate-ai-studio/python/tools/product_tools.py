from __future__ import annotations

import json
import logging
from dataclasses import dataclass
from pathlib import Path

logger = logging.getLogger("affiliate.tools.product")

FALLBACK_PRODUCT_ID = "fallback-unknown"


def _build_fallback(product_url: str, product_text: str) -> dict:
    title = (product_text or product_url or "Unknown product").strip()
    if len(title) > 80:
        title = title[:77] + "..."
    return {
        "id": FALLBACK_PRODUCT_ID,
        "url": product_url,
        "title": title or "Unknown product",
        "price": 0.0,
        "category": "unknown",
        "highlights": [
            "fallback fixture: no local product record matched",
            "downstream tools will treat this as an unknown SKU",
        ],
        "fallback": True,
    }


@dataclass
class ProductLookupTool:
    fixtures: list[dict]
    strict: bool = False

    @classmethod
    def from_file(cls, path: Path, *, strict: bool = False) -> "ProductLookupTool":
        return cls(fixtures=json.loads(path.read_text()), strict=strict)

    def lookup(self, product_url: str, product_text: str) -> dict:
        if product_url:
            for fixture in self.fixtures:
                if fixture.get("url") == product_url:
                    return fixture
        if product_text:
            lowered = product_text.lower()
            for fixture in self.fixtures:
                title = fixture.get("title", "").lower()
                if title and (title in lowered or lowered in title):
                    return fixture

        if self.strict:
            raise LookupError(
                f"no matching product fixture found: url={product_url!r} "
                f"text={product_text!r} (available={len(self.fixtures)})"
            )

        fallback = _build_fallback(product_url, product_text)
        logger.warning(
            "product_lookup_fallback url=%r text=%r fixtures=%d",
            product_url,
            product_text,
            len(self.fixtures),
        )
        return fallback
