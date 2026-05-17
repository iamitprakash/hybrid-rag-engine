import hashlib
import os
from functools import lru_cache

import numpy as np

EMBEDDING_MODEL = os.getenv("EMBEDDING_MODEL", "sentence-transformers/all-MiniLM-L6-v2")
DISABLE_MODELS = os.getenv("DISABLE_MODELS", "false").lower() == "true"
DIMENSION = int(os.getenv("EMBEDDING_DIMENSION", "384"))


@lru_cache(maxsize=1)
def _model():
    if DISABLE_MODELS:
        return None
    from sentence_transformers import SentenceTransformer

    return SentenceTransformer(EMBEDDING_MODEL)


def embed_text(text: str) -> list[float]:
    model = _model()
    if model is None:
        return _hash_embedding(text)
    return model.encode(text, normalize_embeddings=True).astype(float).tolist()


def _hash_embedding(text: str) -> list[float]:
    vector = np.zeros(DIMENSION, dtype=np.float32)
    for token in text.lower().split():
        digest = hashlib.sha256(token.encode("utf-8")).digest()
        idx = int.from_bytes(digest[:4], "big") % DIMENSION
        sign = 1.0 if digest[4] % 2 == 0 else -1.0
        vector[idx] += sign
    norm = np.linalg.norm(vector)
    if norm == 0:
        return vector.tolist()
    return (vector / norm).tolist()
