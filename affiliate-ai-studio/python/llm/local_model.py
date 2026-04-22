from __future__ import annotations

import json
import logging
import re
from dataclasses import dataclass, field
from typing import Any

from schemas.models import AffiliateCopy

logger = logging.getLogger("affiliate.llm")

_LLM_SYSTEM_PROMPT = """\
You are an affiliate copywriting assistant.
Generate compliant, engaging marketing copy for the given product on {platform}.
Locale: {locale}. Tone / style: {style}.

IMPORTANT: you MUST reply with a single JSON object and nothing else. Schema:
{{
  "title": "string — an engaging copy title in the target locale",
  "selling_points": ["string", "string", "string"],
  "localized_hook": "string — one-liner hook in the target locale's natural tone",
  "risk_notes": ["compliance reminder 1", "compliance reminder 2"]
}}
"""

_LLM_USER_PROMPT = """\
Product: {title}
Highlights: {highlights}
Policy context:
{policy_context}

Return the JSON now."""


@dataclass
class LocalModelAdapter:
    mode: str = "mock"
    model_path: str | None = None
    _model: Any = field(default=None, repr=False)
    _tokenizer: Any = field(default=None, repr=False)

    def generate_structured_copy(
        self,
        prompt: str,
        product_info: dict,
        platform: str,
        locale: str,
        style: str,
        context_docs: list[str],
    ) -> tuple[str, AffiliateCopy]:
        if self.mode == "mock":
            return self._mock_generate(product_info, platform, locale, style, context_docs)
        return self._llm_generate(product_info, platform, locale, style, context_docs)

    # ------------------------------------------------------------------
    # Real LLM path — reuses lora/ infra (Qwen via transformers)
    # ------------------------------------------------------------------

    def _ensure_model(self) -> None:
        if self._model is not None:
            return
        import torch
        from transformers import AutoModelForCausalLM, AutoTokenizer

        model_id = self.model_path or "Qwen/Qwen1.5-1.8B-Chat"
        logger.info("loading LLM %s …", model_id)

        self._tokenizer = AutoTokenizer.from_pretrained(model_id, trust_remote_code=True)

        load_kw: dict[str, Any] = dict(trust_remote_code=True, low_cpu_mem_usage=True)
        if torch.cuda.is_available():
            from transformers import BitsAndBytesConfig
            load_kw["quantization_config"] = BitsAndBytesConfig(
                load_in_4bit=True, bnb_4bit_quant_type="nf4",
                bnb_4bit_compute_dtype=torch.bfloat16,
            )
            load_kw["device_map"] = "auto"
            self._device = torch.device("cuda")
        elif torch.backends.mps.is_available():
            load_kw["dtype"] = torch.bfloat16
            self._device = torch.device("mps")
        else:
            load_kw["dtype"] = torch.bfloat16
            self._device = torch.device("cpu")

        self._model = AutoModelForCausalLM.from_pretrained(model_id, **load_kw)
        if "device_map" not in load_kw:
            self._model = self._model.to(self._device)
        self._model.eval()
        logger.info("LLM ready on %s", self._device)

    def _llm_generate(
        self, product_info: dict, platform: str, locale: str, style: str, context_docs: list[str],
    ) -> tuple[str, AffiliateCopy]:
        import torch

        self._ensure_model()
        system = _LLM_SYSTEM_PROMPT.format(platform=platform, locale=locale, style=style)
        user = _LLM_USER_PROMPT.format(
            title=product_info.get("title", ""),
            highlights=", ".join(product_info.get("highlights", [])),
            policy_context="\n".join(context_docs) or "(none)",
        )
        messages = [
            {"role": "system", "content": system},
            {"role": "user", "content": user},
        ]
        text = self._tokenizer.apply_chat_template(messages, tokenize=False, add_generation_prompt=True)
        inputs = self._tokenizer(text, return_tensors="pt").to(self._device)

        with torch.no_grad():
            out = self._model.generate(
                **inputs, max_new_tokens=512, temperature=0.7, top_p=0.9, do_sample=True,
            )
        raw = self._tokenizer.decode(out[0][inputs["input_ids"].shape[1]:], skip_special_tokens=True).strip()
        logger.debug("LLM raw output: %s", raw)

        copy = self._parse_llm_output(raw, product_info, platform, locale, style)
        return raw, copy

    @staticmethod
    def _parse_llm_output(
        raw: str, product_info: dict, platform: str, locale: str, style: str,
    ) -> AffiliateCopy:
        """Best-effort parse; fall back to wrapping raw text as body if JSON fails."""
        m = re.search(r"\{.*\}", raw, re.DOTALL)
        if m:
            try:
                d = json.loads(m.group())
                return AffiliateCopy(
                    title=d.get("title", f"{platform} pick: {product_info.get('title', '')}"),
                    selling_points=d.get("selling_points", [])[:3] or ["See product details"],
                    localized_hook=d.get("localized_hook", ""),
                    risk_notes=d.get("risk_notes", []),
                )
            except (json.JSONDecodeError, KeyError, TypeError):
                pass

        logger.warning("LLM output was not valid JSON, wrapping as freeform copy")
        return AffiliateCopy(
            title=f"{platform} pick: {product_info.get('title', 'Product')}",
            selling_points=[raw[:200] if raw else "See product details"],
            localized_hook="",
            risk_notes=[],
        )

    # ------------------------------------------------------------------
    # Mock path — deterministic, no GPU needed
    # ------------------------------------------------------------------

    def _mock_generate(
        self, product_info: dict, platform: str, locale: str, style: str, context_docs: list[str],
    ) -> tuple[str, AffiliateCopy]:
        title = product_info.get("title", "Untitled Product")
        is_zh = locale.lower().startswith("zh")

        highlights = list(product_info.get("highlights", []))
        if is_zh:
            highlights = [self._translate_zh(h) for h in highlights]
        while len(highlights) < 3:
            highlights.append(
                f"适合 {platform} 用户的高性价比之选" if is_zh
                else f"Reliable value for {platform} shoppers"
            )

        localized_hook = self._localized_hook(locale, style)

        risk_notes = (
            ["避免夸大宣传", "仅使用真实产品卖点"] if is_zh
            else ["Avoid exaggerated claims", "Use real product benefits only"]
        )
        if context_docs:
            risk_notes.append(
                "文案需符合平台政策" if is_zh
                else "Keep copy aligned with retrieved platform policy"
            )

        display_title = (
            f"{platform} 精选: {title}" if is_zh
            else f"{platform} pick: {title}"
        )

        copy = AffiliateCopy(
            title=display_title,
            selling_points=highlights[:3],
            localized_hook=localized_hook,
            risk_notes=risk_notes,
        )
        raw_output = json.dumps(copy.model_dump(), ensure_ascii=False)
        return raw_output, copy

    @staticmethod
    def _translate_zh(text: str) -> str:
        table: dict[str, str] = {
            "10000mAh portable battery": "10000mAh 大容量便携电池",
            "dual-port fast charging": "双口快充，效率翻倍",
            "compact body for travel": "小巧机身，出行无忧",
        }
        return table.get(text, text)

    def _localized_hook(self, locale: str, style: str) -> str:
        lang = locale.lower()
        if lang.startswith("zh"):
            return f"专为日常用户打造，{style} 风格，主打高性价比。"
        if lang.startswith("id"):
            return f"Cocok untuk pengguna harian dengan gaya {style} dan fokus value."
        if lang.startswith("th"):
            return f"เหมาะกับการใช้งานทุกวันในโทน {style} และเน้นความคุ้มค่า"
        if lang.startswith("vi"):
            return f"Phu hop cho nhu cau hang ngay voi phong cach {style} va gia tri that."
        return f"Built for everyday shoppers with a {style} tone and practical value."
