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

Queue a large ingest job asynchronously:

```bash
curl -X POST http://localhost:8080/ingest/async \
  -H 'Content-Type: application/json' \
  -d '{"documents":[{"id":"doc-1","title":"Architecture","content":"Long source text"}]}'
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

Export traces to an OTLP collector:

```bash
OTEL_TRACES_EXPORTER=otlp
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318
```

Enable authenticated multi-tenant isolation:

```bash
TENANT_API_KEYS=tenant-a:secret-a,tenant-b:secret-b
```

When configured, the API expects `X-API-Key` on every endpoint except `/health`. The API key resolves to a tenant, ingestion stamps that tenant into document metadata, and retrieval automatically scopes results to that tenant.

Tune batched ingestion worker pools:

```bash
INGEST_BATCH_SIZE=16
INGEST_WORKERS=4
```

The Go service embeds documents in batches and processes each batch with a bounded worker pool before upserting vectors.

Enable Prometheus metrics export:

```bash
PROMETHEUS_METRICS_ENABLED=true
```

When enabled, Prometheus can scrape `GET /metrics/prometheus` while the existing `GET /metrics` JSON endpoint remains available for quick inspection.

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

### `GET /metrics/prometheus`

Available when `PROMETHEUS_METRICS_ENABLED=true`. Exposes Prometheus counters and histograms for search request volume, cache hits, errors, latency, and grounding quality.

### `POST /evaluation/run`

Runs a retrieval regression dataset through the live search pipeline and returns hit rate plus MRR:

```json
{
  "top_k": 3,
  "examples": [
    {
      "name": "hybrid-basics",
      "query": "how does hybrid retrieval improve rag quality?",
      "expected_ids": ["rag-001", "rag-002"]
    }
  ]
}
```

Sample dataset: [sample_dataset.json](/Users/amitprakash/Documents/New%20project/hybrid-rag-engine/evaluation/sample_dataset.json)

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

### `POST /ingest/async`

Starts the same ingestion pipeline in a background job and returns `202 Accepted` with a job descriptor.

### `GET /jobs/:id`

Returns job state for async ingestion:

```json
{
  "id": "7ab3...",
  "type": "ingest",
  "tenant": "tenant-a",
  "status": "completed",
  "result": {
    "documents": 1,
    "chunks": 3
  }
}
```

### `GET /jobs`

Lists recent async ingestion jobs. When PostgreSQL is configured, job state survives process restarts.

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

- Add persistent vector/BM25 backfill workers.
- Add distributed async job execution.
- Add evaluation diff tooling against saved baseline runs.
