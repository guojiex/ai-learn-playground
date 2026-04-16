from tools.commission_tools import CommissionLookupTool
from tools.product_tools import ProductLookupTool


def test_lookup_product_by_url_returns_fixture() -> None:
    tool = ProductLookupTool(
        fixtures=[
            {
                "id": "power-bank-10000",
                "url": "https://shop.example/power-bank-10000",
                "title": "10000mAh Fast Charging Power Bank",
            }
        ]
    )

    product = tool.lookup("https://shop.example/power-bank-10000", "")

    assert product["id"] == "power-bank-10000"


def test_lookup_commission_returns_rate() -> None:
    tool = CommissionLookupTool(rates={"power-bank-10000": 0.18})

    assert tool.lookup("power-bank-10000") == 0.18


def test_lookup_product_raises_when_not_found() -> None:
    tool = ProductLookupTool(
        fixtures=[
            {
                "id": "power-bank-10000",
                "url": "https://shop.example/power-bank-10000",
                "title": "10000mAh Fast Charging Power Bank",
            }
        ]
    )

    try:
        tool.lookup("https://shop.example/missing", "")
    except LookupError as exc:
        assert "product fixture" in str(exc)
    else:
        raise AssertionError("expected LookupError when fixture is missing")
