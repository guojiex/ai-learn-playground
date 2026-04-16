from __future__ import annotations

import re
from dataclasses import dataclass


def _tokenize(text: str) -> set[str]:
    return {token for token in re.findall(r"[a-zA-Z0-9]+", text.lower()) if token}


@dataclass
class KeywordRetriever:
    documents: list[dict]

    def search(self, query: str, top_k: int = 3) -> list[dict]:
        query_tokens = _tokenize(query)
        scored: list[tuple[int, dict]] = []
        for doc in self.documents:
            score = len(query_tokens.intersection(_tokenize(doc.get("text", "") + " " + doc.get("title", ""))))
            if score > 0:
                scored.append((score, {**doc, "score": score}))
        if not scored and self.documents:
            scored = [(1, {**self.documents[0], "score": 1})]
        scored.sort(key=lambda item: item[0], reverse=True)
        return [item[1] for item in scored[:top_k]]

    def compress(self, query: str, hits: list[dict]) -> list[dict]:
        query_tokens = _tokenize(query)
        compressed: list[dict] = []
        for hit in hits:
            sentences = re.split(r"(?<=[.!?])\s+", hit.get("text", "").strip())
            excerpt = ""
            for sentence in sentences:
                if query_tokens.intersection(_tokenize(sentence)):
                    excerpt = sentence
                    break
            if not excerpt:
                excerpt = sentences[0] if sentences and sentences[0] else hit.get("text", "")[:180]
            compressed.append(
                {
                    "id": hit.get("id"),
                    "title": hit.get("title"),
                    "source": hit.get("source"),
                    "score": hit.get("score", 0),
                    "excerpt": excerpt,
                }
            )
        return compressed
