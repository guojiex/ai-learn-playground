from bridge.runtime import Runtime, dispatch
from labs.prompt_lab import run_prompt_lab
from llm.local_model import LocalModelAdapter
from rag.retriever import KeywordRetriever
from tools.commission_tools import CommissionLookupTool
from tools.product_tools import ProductLookupTool


def build_runtime() -> Runtime:
    return Runtime(
        product_tool=ProductLookupTool(
            fixtures=[
                {
                    "id": "power-bank-10000",
                    "url": "https://shop.example/power-bank-10000",
                    "title": "10000mAh Fast Charging Power Bank",
                    "highlights": ["10000mAh portable battery"],
                }
            ]
        ),
        commission_tool=CommissionLookupTool(rates={"power-bank-10000": 0.18}),
        retriever=KeywordRetriever(
            [
                {
                    "id": "policy-1",
                    "title": "Affiliate Policy",
                    "text": "Avoid exaggerated claims in affiliate copy.",
                    "source": "policies",
                }
            ]
        ),
        model=LocalModelAdapter(mode="mock"),
        prompt_runner=run_prompt_lab,
    )


def test_worker_handles_run_affiliate_graph_action() -> None:
    response = dispatch(
        build_runtime(),
        {
            "action": "run_affiliate_graph",
            "session_id": "session-1",
            "payload": {
                "product_url": "https://shop.example/power-bank-10000",
                "product_text": "",
                "platform": "TikTok",
                "locale": "id-ID",
                "style": "casual",
                "min_commission_rate": 0.1,
                "enable_compression": True,
            },
        },
    )

    assert response["status"] == "ok"
    assert response["data"]["result"]["decision"] == "accepted"
