from labs.prompt_lab import render_prompt, run_prompt_lab
from llm.local_model import LocalModelAdapter


def test_prompt_lab_returns_structured_copy() -> None:
    model = LocalModelAdapter(mode="mock")
    result = run_prompt_lab(
        {
            "title": "10000mAh Fast Charging Power Bank",
            "highlights": ["10000mAh portable battery", "dual-port fast charging"],
        },
        "TikTok",
        "id-ID",
        "casual",
        ["Avoid exaggerated claims."],
        model=model,
    )

    assert result["structured_output"].title
    assert len(result["structured_output"].selling_points) == 3


def test_render_prompt_includes_platform_and_style() -> None:
    prompt = render_prompt(
        {
            "title": "10000mAh Fast Charging Power Bank",
            "highlights": ["10000mAh portable battery"],
        },
        "TikTok",
        "id-ID",
        "casual",
        ["Avoid exaggerated claims."],
    )

    assert "TikTok" in prompt
    assert "casual" in prompt
