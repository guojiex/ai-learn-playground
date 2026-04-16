from bridge.runtime import Runtime
from graph.affiliate_graph import run_affiliate_graph
from labs.prompt_lab import run_prompt_lab
from llm.local_model import LocalModelAdapter
from rag.retriever import KeywordRetriever
from tools.commission_tools import CommissionLookupTool
from tools.product_tools import ProductLookupTool


def make_runtime(commission: float) -> Runtime:
    return Runtime(
        product_tool=ProductLookupTool(
            fixtures=[
                {
                    "id": "power-bank-10000",
                    "url": "https://shop.example/power-bank-10000",
                    "title": "10000mAh Fast Charging Power Bank",
                    "highlights": [
                        "10000mAh portable battery",
                        "dual-port fast charging",
                        "compact body for travel",
                    ],
                }
            ]
        ),
        commission_tool=CommissionLookupTool(rates={"power-bank-10000": commission}),
        retriever=KeywordRetriever(
            [
                {
                    "id": "policy-1",
                    "title": "Affiliate Policy",
                    "text": "Avoid exaggerated claims and unsupported guarantees in affiliate copy.",
                    "source": "policies",
                }
            ]
        ),
        model=LocalModelAdapter(mode="mock"),
        prompt_runner=run_prompt_lab,
    )


def test_graph_rejects_low_commission_before_generation() -> None:
    result = run_affiliate_graph(
        make_runtime(0.06),
        {
            "product_url": "https://shop.example/power-bank-10000",
            "product_text": "",
            "platform": "TikTok",
            "locale": "id-ID",
            "style": "casual",
            "min_commission_rate": 0.1,
            "enable_compression": True,
        },
    )

    assert result["result"]["decision"] == "rejected"
    assert result["result"]["copy"] is None


def test_graph_accepts_and_returns_copy_when_commission_meets_threshold() -> None:
    result = run_affiliate_graph(
        make_runtime(0.18),
        {
            "product_url": "https://shop.example/power-bank-10000",
            "product_text": "",
            "platform": "TikTok",
            "locale": "id-ID",
            "style": "casual",
            "min_commission_rate": 0.1,
            "enable_compression": True,
        },
    )

    assert result["result"]["decision"] == "accepted"
    assert result["result"]["copy"]["title"]
    assert result["retrieval"]["compressed_context"]
