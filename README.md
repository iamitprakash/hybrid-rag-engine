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
- `redis`: Optional query cache for repeated non-stream search requests.
- `postgres`: Optional durable metadata store for source documents and chunks.

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

Run a filtered search:

```bash
curl -X POST http://localhost:8080/search \
  -H 'Content-Type: application/json' \
  -d '{"query":"retrieval orchestration","top_k":5,"filters":{"tenant":"demo","source":"docs"}}'
```

Run an authenticated tenant-scoped search:

```bash
curl -X POST http://localhost:8080/search \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: secret-a' \
  -d '{"query":"retrieval orchestration","top_k":5}'
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

Configure Redis query caching:

```bash
REDIS_URL=redis://localhost:6379/0
CACHE_TTL=5m
```

If `REDIS_URL` is empty or invalid, search still works without caching.

Configure PostgreSQL metadata storage:

```bash
POSTGRES_DSN=postgres://rag:rag@localhost:5432/rag?sslmode=disable
```

When configured, the Go service creates `documents` and `chunks` tables on startup and `/documents` reads from Postgres. Without `POSTGRES_DSN`, the service keeps using the in-memory BM25 corpus for local development.

Enable local OpenTelemetry trace output:

```bash
OTEL_TRACES_EXPORTER=stdout
```

Tracing is disabled by default. When enabled, the Go service emits spans for embedding generation, BM25 retrieval, vector retrieval, rank fusion, reranking, and synthesis.

Enable authenticated multi-tenant isolation:

```bash
TENANT_API_KEYS=tenant-a:secret-a,tenant-b:secret-b
```

When configured, the API expects `X-API-Key` on every endpoint except `/health`. The API key resolves to a tenant, ingestion stamps that tenant into document metadata, and retrieval automatically scopes results to that tenant.

## API

### `GET /health`

Returns service health.

### `GET /metrics`

Returns in-process retrieval quality and operational counters:

```json
{
  "search_requests": 10,
  "search_errors": 0,
  "cache_hits": 3,
  "average_latency_ms": 84,
  "average_grounded_ratio": 0.76,
  "high_risk_grounding_hits": 1
}
```

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

Returns the active chunk corpus. If PostgreSQL is configured, this reads durable metadata from Postgres; otherwise it returns the in-memory BM25 corpus.

### `POST /search`

Request:

```json
{
  "query": "how does reranking improve retrieval precision?",
  "top_k": 5,
  "filters": {
    "tenant": "demo",
    "source": "docs"
  }
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
    "reranked_hits": 5,
    "cache_hit": false
  },
  "grounding": {
    "context_document_count": 5,
    "context_token_count": 180,
    "answer_token_count": 42,
    "grounded_token_ratio": 0.78,
    "unsupported_terms": ["example"],
    "hallucination_risk": "low"
  }
}
```

Filters are exact-match constraints. `source` and `id` map to top-level document fields; other keys map to document metadata.
The `grounding` block is a lightweight hallucination risk signal based on answer-term support in retrieved context. It is not a replacement for human review, but it is useful for experiments and regression tracking.

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
│   │   ├── metadata/
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

- Add authenticated multi-tenant indexing.
- Add async ingestion jobs for large corpora.
- Add evaluation datasets and retrieval regression reports.
