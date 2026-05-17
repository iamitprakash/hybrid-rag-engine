package app

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"hybrid-rag-engine/go-api/internal/api"
	"hybrid-rag-engine/go-api/internal/bm25"
	"hybrid-rag-engine/go-api/internal/cache"
	"hybrid-rag-engine/go-api/internal/ingest"
	"hybrid-rag-engine/go-api/internal/jobs"
	"hybrid-rag-engine/go-api/internal/llm"
	"hybrid-rag-engine/go-api/internal/metadata"
	"hybrid-rag-engine/go-api/internal/metrics"
	"hybrid-rag-engine/go-api/internal/observability"
	"hybrid-rag-engine/go-api/internal/retrieval"
	"hybrid-rag-engine/go-api/internal/tenantauth"
	"hybrid-rag-engine/go-api/internal/vector"
)

func Run() {
	shutdownTracing, err := observability.InitTracing(context.Background(), "hybrid-rag-go-api")
	if err != nil {
		log.Printf("tracing disabled: %v", err)
	}
	defer func() {
		if shutdownTracing != nil {
			_ = shutdownTracing(context.Background())
		}
	}()

	aiBaseURL := env("AI_SERVICE_URL", "http://localhost:8000")
	qdrantURL := env("QDRANT_URL", "http://localhost:6333")
	collection := env("QDRANT_COLLECTION", "documents")
	llmBaseURL := env("LLM_BASE_URL", "https://api.openai.com/v1")
	llmAPIKey := env("LLM_API_KEY", "")
	llmModel := env("LLM_MODEL", "gpt-4o-mini")
	redisURL := env("REDIS_URL", "")
	cacheTTL := durationEnv("CACHE_TTL", 5*time.Minute)
	postgresDSN := env("POSTGRES_DSN", "")
	tenantAPIKeys := env("TENANT_API_KEYS", "")
	ingestBatchSize := intEnv("INGEST_BATCH_SIZE", 16)
	ingestWorkers := intEnv("INGEST_WORKERS", 4)

	corpus := retrieval.SampleDocuments()
	bm25Index := bm25.NewIndex(corpus)
	vectorClient := vector.NewClient(qdrantURL, collection, 384)
	aiClient := retrieval.NewAIClient(aiBaseURL, 30*time.Second)
	orchestrator := retrieval.NewOrchestrator(
		bm25Index,
		vectorClient,
		aiClient,
		llm.NewSynthesizer(llm.Config{
			BaseURL: llmBaseURL,
			APIKey:  llmAPIKey,
			Model:   llmModel,
			Timeout: 45 * time.Second,
		}),
	)

	cacheClient, err := cache.New(redisURL, cacheTTL)
	if err != nil {
		log.Printf("cache disabled: %v", err)
		cacheClient, _ = cache.New("", 0)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	metadataStore, err := metadata.New(ctx, postgresDSN)
	if err != nil {
		log.Printf("metadata store disabled: %v", err)
		metadataStore, _ = metadata.New(context.Background(), "")
	}
	defer metadataStore.Close()

	metricsRecorder := metrics.NewRecorder()
	jobStore, err := jobs.NewStore(ctx, postgresDSN)
	if err != nil {
		log.Printf("job store disabled: %v", err)
		jobStore, _ = jobs.NewStore(context.Background(), "")
	}
	defer jobStore.Close()
	jobManager := jobs.NewManager(jobStore)
	auth := tenantauth.New(tenantAPIKeys)
	processor := ingest.NewProcessor(aiClient, vectorClient, ingestBatchSize, ingestWorkers)

	router := api.NewRouter(orchestrator, processor, bm25Index, cacheClient, metadataStore, metricsRecorder, jobManager, auth, corpus)
	log.Println("Go API running on :8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return duration
}

func env(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
