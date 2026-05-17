import reranker


def test_lexical_rerank_prioritizes_overlap():
    ranked = reranker._lexical_rerank(
        "hybrid retrieval",
        [
            {"id": "a", "content": "hybrid retrieval with bm25"},
            {"id": "b", "content": "vector database only"},
        ],
    )

    assert ranked[0]["id"] == "a"
    assert ranked[0]["score"] > ranked[1]["score"]
