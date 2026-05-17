import math

import embeddings


def test_hash_embedding_is_deterministic_and_normalized():
    vec1 = embeddings._hash_embedding("hybrid retrieval")
    vec2 = embeddings._hash_embedding("hybrid retrieval")

    assert vec1 == vec2
    norm = math.sqrt(sum(value * value for value in vec1))
    assert abs(norm - 1.0) < 1e-5
