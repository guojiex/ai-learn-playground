from tools.commission_tools import CommissionLookupTool
from tools.product_tools import FALLBACK_PRODUCT_ID, ProductLookupTool


def _sample_fixtures() -> list[dict]:
    return [
        {
            "id": "power-bank-10000",
            "url": "https://shop.example/power-bank-10000",
            "title": "10000mAh Fast Charging Power Bank",
        }
    ]


def test_lookup_product_by_url_returns_fixture() -> None:
    tool = ProductLookupTool(fixtures=_sample_fixtures())

    product = tool.lookup("https://shop.example/power-bank-10000", "")

    assert product["id"] == "power-bank-10000"
    assert not product.get("fallback")


def test_lookup_commission_returns_rate() -> None:
    tool = CommissionLookupTool(rates={"power-bank-10000": 0.18})

    assert tool.lookup("power-bank-10000") == 0.18


def test_lookup_commission_unknown_product_uses_mock_default() -> None:
    tool = CommissionLookupTool(rates={"power-bank-10000": 0.18})

    assert tool.lookup("fallback-unknown") == 0.15
    assert tool.lookup("any-unlisted-sku") == 0.15


def test_lookup_commission_custom_mock_default() -> None:
    tool = CommissionLookupTool(rates={}, default_mock_rate=0.08)

    assert tool.lookup("x") == 0.08


def test_lookup_product_returns_fallback_when_not_found() -> None:
    tool = ProductLookupTool(fixtures=_sample_fixtures())

    product = tool.lookup("https://shopee.co.id/product/1/2", "")

    assert product["id"] == FALLBACK_PRODUCT_ID
    assert product["fallback"] is True
    assert product["url"] == "https://shopee.co.id/product/1/2"
    assert product["category"] == "unknown"


def test_lookup_product_fallback_uses_text_when_url_empty() -> None:
    tool = ProductLookupTool(fixtures=_sample_fixtures())

    product = tool.lookup("", "completely unseen product name")

    assert product["id"] == FALLBACK_PRODUCT_ID
    assert "unseen product" in product["title"]


def test_lookup_product_strict_mode_still_raises() -> None:
    tool = ProductLookupTool(fixtures=_sample_fixtures(), strict=True)

    try:
        tool.lookup("https://shop.example/missing", "")
    except LookupError as exc:
        assert "product fixture" in str(exc)
        assert "missing" in str(exc)
    else:
        raise AssertionError("expected LookupError in strict mode")
