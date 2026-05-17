from fastapi.testclient import TestClient

import app


def test_health_endpoint():
    client = TestClient(app.app)
    response = client.get("/health")
    assert response.status_code == 200
    assert response.json()["status"] == "ok"


def test_embed_endpoint(monkeypatch):
    monkeypatch.setattr(app, "embed_text", lambda text: [1.0, 2.0])
    client = TestClient(app.app)
    response = client.post("/embed", json={"text": "hello"})
    assert response.status_code == 200
    assert response.json()["embedding"] == [1.0, 2.0]


def test_rerank_endpoint(monkeypatch):
    monkeypatch.setattr(app, "rerank", lambda query, docs, top_k: [{"id": docs[0]["id"], "score": 0.9}])
    client = TestClient(app.app)
    response = client.post(
        "/rerank",
        json={
            "query": "hybrid retrieval",
            "top_k": 1,
            "documents": [{"id": "doc-1", "title": "Doc", "content": "context"}],
        },
    )
    assert response.status_code == 200
    assert response.json()["documents"][0]["id"] == "doc-1"
