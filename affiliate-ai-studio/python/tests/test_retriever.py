from rag.retriever import KeywordRetriever


def test_retriever_returns_policy_hits() -> None:
    retriever = KeywordRetriever(
        [
            {
                "id": "policy-1",
                "title": "Affiliate Policy",
                "text": "Avoid exaggerated claims and unsupported guarantees in affiliate copy.",
                "source": "policies",
            }
        ]
    )

    hits = retriever.search("affiliate claims", top_k=1)

    assert hits[0]["id"] == "policy-1"


def test_retriever_compresses_context() -> None:
    retriever = KeywordRetriever(
        [
            {
                "id": "policy-1",
                "title": "Affiliate Policy",
                "text": "Avoid exaggerated claims and unsupported guarantees in affiliate copy.",
                "source": "policies",
            }
        ]
    )

    hits = retriever.search("affiliate claims", top_k=1)
    compressed = retriever.compress("affiliate claims", hits)

    assert compressed[0]["excerpt"]
