from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable

from graph.affiliate_graph import run_affiliate_graph
from labs.prompt_lab import run_prompt_lab
from llm.local_model import LocalModelAdapter
from rag.indexer import load_kb_documents
from rag.retriever import KeywordRetriever
from tools.commission_tools import CommissionLookupTool
from tools.product_tools import ProductLookupTool


@dataclass
class Runtime:
    product_tool: ProductLookupTool
    commission_tool: CommissionLookupTool
    retriever: KeywordRetriever
    model: LocalModelAdapter
    prompt_runner: Callable[..., dict]


def build_runtime(project_root: Path) -> Runtime:
    assets_dir = project_root / "assets"
    return Runtime(
        product_tool=ProductLookupTool.from_file(assets_dir / "products" / "sample_products.json"),
        commission_tool=CommissionLookupTool.from_file(assets_dir / "products" / "commission_rates.json"),
        retriever=KeywordRetriever(load_kb_documents(assets_dir / "kb")),
        model=LocalModelAdapter(
            mode=os.getenv("AFFILIATE_MODEL_MODE", "mock"),
            model_path=os.getenv("AFFILIATE_MODEL_PATH"),
        ),
        prompt_runner=run_prompt_lab,
    )


def dispatch(runtime: Runtime, request: dict[str, Any]) -> dict[str, Any]:
    action = request.get("action")
    session_id = request.get("session_id", "")
    payload = request.get("payload", {})

    try:
        if action == "ping":
            return {"status": "ok", "data": {"message": "pong"}}
        if action == "render_prompt":
            result = runtime.prompt_runner(
                payload.get("product_info", {}),
                payload.get("platform", ""),
                payload.get("locale", ""),
                payload.get("style", ""),
                payload.get("context_docs", []),
                model=runtime.model,
            )
            result["structured_output"] = result["structured_output"].model_dump()
            return {"status": "ok", "data": result}
        if action == "run_affiliate_graph":
            data = run_affiliate_graph(runtime, {**payload, "session_id": session_id})
            return {"status": "ok", "data": data}
        return {
            "status": "error",
            "error": {"code": "validation_error", "message": f"unsupported action: {action}"},
        }
    except LookupError as exc:
        return {"status": "error", "error": {"code": "tool_error", "message": str(exc)}}
    except Exception as exc:  # pragma: no cover - defensive path for worker stability
        return {"status": "error", "error": {"code": "generation_error", "message": str(exc)}}
