package app

import (
	"log"
	"os"
	"time"

	"hybrid-rag-engine/go-api/internal/api"
	"hybrid-rag-engine/go-api/internal/bm25"
	"hybrid-rag-engine/go-api/internal/cache"
	"hybrid-rag-engine/go-api/internal/llm"
	"hybrid-rag-engine/go-api/internal/retrieval"
	"hybrid-rag-engine/go-api/internal/vector"
)

func Run() {
	aiBaseURL := env("AI_SERVICE_URL", "http://localhost:8000")
	qdrantURL := env("QDRANT_URL", "http://localhost:6333")
	collection := env("QDRANT_COLLECTION", "documents")
	llmBaseURL := env("LLM_BASE_URL", "https://api.openai.com/v1")
	llmAPIKey := env("LLM_API_KEY", "")
	llmModel := env("LLM_MODEL", "gpt-4o-mini")
	redisURL := env("REDIS_URL", "")
	cacheTTL := durationEnv("CACHE_TTL", 5*time.Minute)

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

	router := api.NewRouter(orchestrator, aiClient, vectorClient, bm25Index, cacheClient, corpus)
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
