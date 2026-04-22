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
        raw_output = (
            "{"
            f'"title":"{copy.title}",'
            f'"selling_points":{copy.selling_points!r},'
            f'"localized_hook":"{copy.localized_hook}",'
            f'"risk_notes":{copy.risk_notes!r}'
            "}"
        )
        return raw_output, copy

    @staticmethod
    def _translate_zh(text: str) -> str:
        """简易 mock 翻译：把常见英文 highlight 短语转成中文。"""
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
