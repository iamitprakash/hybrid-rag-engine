# Hybrid RAG Engine

Production-style Hybrid RAG Engine demonstrating semantic vector retrieval, BM25 lexical search, hybrid rank fusion, reranking, and final answer synthesis with a Go orchestration layer and Python model service.

## Architecture

```text
Client Query
  -> Go API Gateway
  -> Query Orchestrator
  -> Parallel Retrieval: BM25 + Vector Search
  -> Hybrid Rank Fusion
  -> Python Reranker Service
  -> LLM Synthesis Layer
  -> Final Response
```

## Services

- `go-api`: Gin API gateway, retrieval orchestration, concurrency, reciprocal-rank fusion, and response synthesis.
- `ai-service`: FastAPI service for embeddings and reranking with Sentence Transformers.
- `qdrant`: Vector database for semantic search.

## Quick Start

```bash
docker compose up --build
```

Seed the sample corpus into Qdrant:

```bash
curl -X POST http://localhost:8080/ingest/sample
```

Run a hybrid search:

```bash
curl -X POST http://localhost:8080/search \
  -H 'Content-Type: application/json' \
  -d '{"query":"how does hybrid retrieval improve rag quality?","top_k":5}'
```

Stream a grounded answer with server-sent events:

```bash
curl -N -X POST http://localhost:8080/search/stream \
  -H 'Content-Type: application/json' \
  -d '{"query":"how does hybrid retrieval improve rag quality?","top_k":5}'
```

## Local Development

Run the Python service:

```bash
cd ai-service
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
uvicorn app:app --reload --port 8000
```

Run Qdrant:

```bash
docker run --rm -p 6333:6333 qdrant/qdrant:v1.12.6
```

Run the Go API:

```bash
cd go-api
go run .
```

For offline development without downloading transformer models, set:

```bash
DISABLE_MODELS=true
```

The AI service then uses deterministic hash embeddings and lexical reranking.

Configure OpenAI-compatible synthesis:

```bash
LLM_BASE_URL=https://api.openai.com/v1
LLM_API_KEY=your-api-key
LLM_MODEL=gpt-4o-mini
```

For Groq, use:

```bash
LLM_BASE_URL=https://api.groq.com/openai/v1
LLM_MODEL=llama-3.1-8b-instant
```

If `LLM_API_KEY` is empty, the Go service uses a deterministic local synthesis fallback.

## API

### `GET /health`

Returns service health.

### `POST /ingest/sample`

Embeds and upserts the built-in sample corpus into Qdrant.

### `POST /ingest`

Chunks arbitrary documents, embeds each chunk, upserts the vectors into Qdrant, and appends the same chunks to the in-memory BM25 index.

Request:

```json
{
  "chunking": {
    "chunk_size": 180,
    "overlap": 30
  },
  "documents": [
    {
      "id": "architecture-note",
      "title": "Architecture Note",
      "content": "Long source document text...",
      "source": "docs",
      "metadata": {
        "tenant": "demo"
      }
    }
  ]
}
```

Response:

```json
{
  "documents": 1,
  "chunks": 3
}
```

### `GET /documents`

Returns the active in-memory BM25 corpus. This is useful during local development to verify ingested chunks.

### `POST /search`

Request:

```json
{
  "query": "how does reranking improve retrieval precision?",
  "top_k": 5
}
```

Response:

```json
{
  "query": "how does reranking improve retrieval precision?",
  "answer": "Based on the retrieved context...",
  "documents": [],
  "trace": {
    "bm25_hits": 3,
    "vector_hits": 5,
    "fused_hits": 5,
    "reranked_hits": 5
  }
}
```

### `POST /search/stream`

Runs the same retrieval and reranking pipeline as `/search`, then streams synthesis as server-sent events:

- `retrieval`: ranked documents and retrieval trace
- `token`: incremental answer text
- `error`: synthesis error, if one occurs
- `done`: terminal event

## Repository Layout

```text
hybrid-rag-engine/
├── go-api/
│   ├── cmd/server/
│   ├── internal/
│   │   ├── api/
│   │   ├── app/
│   │   ├── bm25/
│   │   ├── fusion/
│   │   ├── llm/
│   │   ├── retrieval/
│   │   └── vector/
│   ├── Dockerfile
│   ├── go.mod
│   └── main.go
├── ai-service/
│   ├── app.py
│   ├── embeddings.py
│   ├── reranker.py
│   ├── Dockerfile
│   └── requirements.txt
├── docker-compose.yml
├── .env
└── README.md
```

## Next Milestones

- Add metadata filtering.
- Add PostgreSQL for document metadata and Redis for query cache.
- Add OpenTelemetry spans for embedding, BM25, vector search, fusion, reranking, and synthesis.
- Add retrieval quality metrics and hallucination checks.
