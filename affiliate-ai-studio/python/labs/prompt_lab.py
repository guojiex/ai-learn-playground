from __future__ import annotations

from langchain_core.prompts import ChatPromptTemplate

from llm.local_model import LocalModelAdapter


def render_prompt(
    product_info: dict,
    platform: str,
    locale: str,
    style: str,
    context_docs: list[str],
) -> str:
    prompt = ChatPromptTemplate.from_messages(
        [
            (
                "system",
                (
                    "You are an affiliate copywriting assistant. "
                    "Generate compliant copy for {platform} in locale {locale} "
                    "with a {style} style."
                ),
            ),
            (
                "human",
                (
                    "Product title: {title}\n"
                    "Highlights: {highlights}\n"
                    "Policy context: {policy_context}\n"
                    "Return a concise affiliate copy plan."
                ),
            ),
        ]
    )
    messages = prompt.format_messages(
        platform=platform,
        locale=locale,
        style=style,
        title=product_info.get("title", ""),
        highlights=", ".join(product_info.get("highlights", [])),
        policy_context="\n".join(context_docs),
    )
    return "\n".join(f"{message.type}: {message.content}" for message in messages)


def run_prompt_lab(
    product_info: dict,
    platform: str,
    locale: str,
    style: str,
    context_docs: list[str],
    model: LocalModelAdapter | None = None,
) -> dict:
    adapter = model or LocalModelAdapter()
    rendered_prompt = render_prompt(product_info, platform, locale, style, context_docs)
    raw_output, structured_output = adapter.generate_structured_copy(
        rendered_prompt, product_info, platform, locale, style, context_docs
    )
    return {
        "rendered_prompt": rendered_prompt,
        "raw_output": raw_output,
        "structured_output": structured_output,
    }
