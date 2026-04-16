from __future__ import annotations

from pathlib import Path


def load_kb_documents(base_dir: Path) -> list[dict]:
    documents: list[dict] = []
    for path in sorted(base_dir.rglob("*.md")):
        documents.append(
            {
                "id": path.stem,
                "title": path.stem.replace("_", " ").title(),
                "text": path.read_text(),
                "source": path.parent.name,
            }
        )
    return documents
