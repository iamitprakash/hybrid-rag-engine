import os
from functools import lru_cache

RERANKER_MODEL = os.getenv("RERANKER_MODEL", "cross-encoder/ms-marco-MiniLM-L-6-v2")
DISABLE_MODELS = os.getenv("DISABLE_MODELS", "false").lower() == "true"


@lru_cache(maxsize=1)
def _model():
    if DISABLE_MODELS:
        return None
    from sentence_transformers import CrossEncoder

    return CrossEncoder(RERANKER_MODEL)


def rerank(query: str, documents: list[dict], top_k: int) -> list[dict]:
    if not documents:
        return []

    model = _model()
    if model is None:
        ranked = _lexical_rerank(query, documents)
    else:
        pairs = [[query, doc["content"]] for doc in documents]
        scores = model.predict(pairs)
        ranked = []
        for doc, score in zip(documents, scores):
            enriched = dict(doc)
            enriched["score"] = float(score)
            ranked.append(enriched)
        ranked.sort(key=lambda item: item["score"], reverse=True)

    return ranked[:top_k]


def _lexical_rerank(query: str, documents: list[dict]) -> list[dict]:
    query_terms = set(query.lower().split())
    ranked = []
    for doc in documents:
        content_terms = set(doc.get("content", "").lower().split())
        overlap = len(query_terms & content_terms)
        enriched = dict(doc)
        enriched["score"] = float(overlap) + float(doc.get("score", 0))
        ranked.append(enriched)
    ranked.sort(key=lambda item: item["score"], reverse=True)
    return ranked
