from __future__ import annotations

from dataclasses import dataclass

from schemas.models import AffiliateCopy


@dataclass
class LocalModelAdapter:
    mode: str = "mock"
    model_path: str | None = None

    def generate_structured_copy(
        self,
        prompt: str,
        product_info: dict,
        platform: str,
        locale: str,
        style: str,
        context_docs: list[str],
    ) -> tuple[str, AffiliateCopy]:
        title = product_info.get("title", "Untitled Product")
        highlights = list(product_info.get("highlights", []))
        while len(highlights) < 3:
            highlights.append(f"Reliable value for {platform} shoppers")

        localized_hook = self._localized_hook(locale, style)
        risk_notes = ["Avoid exaggerated claims", "Use real product benefits only"]
        if context_docs:
            risk_notes.append("Keep copy aligned with retrieved platform policy")

        copy = AffiliateCopy(
            title=f"{platform} pick: {title}",
            selling_points=highlights[:3],
            localized_hook=localized_hook,
            risk_notes=risk_notes,
        )
        raw_output = (
            "{"
            f'"title":"{copy.title}",'
            f'"selling_points":{copy.selling_points!r},'
            f'"localized_hook":"{copy.localized_hook}",'
            f'"risk_notes":{copy.risk_notes!r}'
            "}"
        )
        return raw_output, copy

    def _localized_hook(self, locale: str, style: str) -> str:
        if locale.lower().startswith("id"):
            return f"Cocok untuk pengguna harian dengan gaya {style} dan fokus value."
        if locale.lower().startswith("th"):
            return f"เหมาะกับการใช้งานทุกวันในโทน {style} และเน้นความคุ้มค่า"
        if locale.lower().startswith("vi"):
            return f"Phu hop cho nhu cau hang ngay voi phong cach {style} va gia tri that."
        return f"Built for everyday shoppers with a {style} tone and practical value."
