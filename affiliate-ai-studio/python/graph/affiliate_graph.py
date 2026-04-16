from __future__ import annotations

from typing import Any, Dict, List, Optional, TypedDict

from langgraph.graph import END, StateGraph


class AffiliateState(TypedDict, total=False):
    session_id: str
    product_url: str
    product_text: str
    platform: str
    locale: str
    style: str
    min_commission_rate: float
    enable_compression: bool
    product_info: Dict[str, Any]
    commission_rate: float
    commission_ok: bool
    retrieved_docs: List[Dict[str, Any]]
    compressed_docs: List[Dict[str, Any]]
    copy: Optional[Dict[str, Any]]
    raw_output: str
    rendered_prompt: str
    result: Dict[str, Any]
    tools: List[Dict[str, Any]]
    trace: List[Dict[str, Any]]


def _trace(state: AffiliateState, node: str, summary: str, output: dict[str, Any]) -> list[dict[str, Any]]:
    return state.get("trace", []) + [
        {
            "node": node,
            "summary": summary,
            "status": "ok",
            "input_snapshot": {
                "product_url": state.get("product_url", ""),
                "platform": state.get("platform", ""),
            },
            "output_snapshot": output,
        }
    ]


def run_affiliate_graph(runtime, payload: dict[str, Any]) -> dict[str, Any]:
    def normalize_input(state: AffiliateState) -> dict[str, Any]:
        return {
            "trace": _trace(state, "normalize_input", "Normalized affiliate request", {}),
        }

    def fetch_product_info(state: AffiliateState) -> dict[str, Any]:
        product = runtime.product_tool.lookup(state.get("product_url", ""), state.get("product_text", ""))
        tool_calls = state.get("tools", []) + [
            {"tool": "fetch_product_info", "input": {"product_url": state.get("product_url", "")}, "output": product}
        ]
        return {
            "product_info": product,
            "tools": tool_calls,
            "trace": _trace(state, "fetch_product_info", "Loaded product fixture", {"product_id": product["id"]}),
        }

    def check_commission(state: AffiliateState) -> dict[str, Any]:
        rate = runtime.commission_tool.lookup(state["product_info"]["id"])
        tool_calls = state.get("tools", []) + [
            {"tool": "check_commission", "input": {"product_id": state["product_info"]["id"]}, "output": {"rate": rate}}
        ]
        return {
            "commission_rate": rate,
            "commission_ok": rate >= float(state.get("min_commission_rate", 0.0)),
            "tools": tool_calls,
            "trace": _trace(state, "check_commission", "Evaluated commission threshold", {"commission_rate": rate}),
        }

    def route_commission(state: AffiliateState) -> str:
        return "rejected" if not state.get("commission_ok") else "accepted"

    def retrieve_policy_context(state: AffiliateState) -> dict[str, Any]:
        query = f"{state['product_info']['title']} affiliate claims"
        hits = runtime.retriever.search(query, top_k=3)
        compressed = runtime.retriever.compress(query, hits) if state.get("enable_compression", True) else hits
        tool_calls = state.get("tools", []) + [
            {"tool": "search_policy_kb", "input": {"query": query}, "output": {"hits": len(hits)}}
        ]
        return {
            "retrieved_docs": hits,
            "compressed_docs": compressed,
            "tools": tool_calls,
            "trace": _trace(state, "retrieve_policy_context", "Retrieved KB evidence", {"hits": len(hits)}),
        }

    def generate_structured_copy(state: AffiliateState) -> dict[str, Any]:
        context_docs = [item["excerpt"] for item in state.get("compressed_docs", [])]
        prompt_result = runtime.prompt_runner(
            state["product_info"],
            state.get("platform", ""),
            state.get("locale", ""),
            state.get("style", ""),
            context_docs,
            model=runtime.model,
        )
        return {
            "copy": prompt_result["structured_output"].model_dump(),
            "raw_output": prompt_result["raw_output"],
            "rendered_prompt": prompt_result["rendered_prompt"],
            "trace": _trace(state, "generate_structured_copy", "Generated structured affiliate copy", {"title": prompt_result["structured_output"].title}),
        }

    def finalize_rejected(state: AffiliateState) -> dict[str, Any]:
        return {
            "result": {
                "decision": "rejected",
                "reason": "Commission below minimum threshold",
                "commission_rate": state.get("commission_rate", 0.0),
                "copy": None,
            },
            "trace": _trace(state, "finalize_rejected", "Stopped before generation", {}),
        }

    def finalize_accepted(state: AffiliateState) -> dict[str, Any]:
        return {
            "result": {
                "decision": "accepted",
                "reason": "Commission threshold met",
                "commission_rate": state.get("commission_rate", 0.0),
                "copy": state.get("copy"),
            },
            "trace": _trace(state, "finalize_accepted", "Prepared success response", {}),
        }

    workflow = StateGraph(AffiliateState)
    workflow.add_node("normalize_input", normalize_input)
    workflow.add_node("fetch_product_info", fetch_product_info)
    workflow.add_node("check_commission", check_commission)
    workflow.add_node("retrieve_policy_context", retrieve_policy_context)
    workflow.add_node("generate_structured_copy", generate_structured_copy)
    workflow.add_node("finalize_rejected", finalize_rejected)
    workflow.add_node("finalize_accepted", finalize_accepted)
    workflow.set_entry_point("normalize_input")
    workflow.add_edge("normalize_input", "fetch_product_info")
    workflow.add_edge("fetch_product_info", "check_commission")
    workflow.add_conditional_edges(
        "check_commission",
        route_commission,
        {
            "rejected": "finalize_rejected",
            "accepted": "retrieve_policy_context",
        },
    )
    workflow.add_edge("retrieve_policy_context", "generate_structured_copy")
    workflow.add_edge("generate_structured_copy", "finalize_accepted")
    workflow.add_edge("finalize_rejected", END)
    workflow.add_edge("finalize_accepted", END)

    compiled = workflow.compile()
    final_state = compiled.invoke(payload)
    return {
        "session_id": final_state.get("session_id", ""),
        "result": final_state["result"],
        "retrieval": {
            "hits": final_state.get("retrieved_docs", []),
            "compressed_context": final_state.get("compressed_docs", []),
        },
        "tools": final_state.get("tools", []),
        "trace": final_state.get("trace", []),
        "debug": {
            "rendered_prompt": final_state.get("rendered_prompt", ""),
            "raw_output": final_state.get("raw_output", ""),
        },
    }
