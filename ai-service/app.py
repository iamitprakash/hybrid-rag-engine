from fastapi import FastAPI
from pydantic import BaseModel, Field

from embeddings import embed_text
from reranker import rerank

app = FastAPI(title="Hybrid RAG AI Service", version="0.1.0")


class EmbedRequest(BaseModel):
    text: str = Field(min_length=1)


class Document(BaseModel):
    id: str
    title: str
    content: str
    source: str | None = None
    metadata: dict[str, str] | None = None
    score: float | None = None


class RerankRequest(BaseModel):
    query: str = Field(min_length=1)
    documents: list[Document]
    top_k: int = 5


@app.get("/health")
def health() -> dict:
    return {"status": "ok"}


@app.post("/embed")
def embed(payload: EmbedRequest) -> dict:
    return {"embedding": embed_text(payload.text)}


@app.post("/rerank")
def rerank_documents(payload: RerankRequest) -> dict:
    docs = [doc.model_dump() for doc in payload.documents]
    return {"documents": rerank(payload.query, docs, payload.top_k)}
