package app

import (
	"log"
	"os"
	"time"

	"hybrid-rag-engine/go-api/internal/api"
	"hybrid-rag-engine/go-api/internal/bm25"
	"hybrid-rag-engine/go-api/internal/llm"
	"hybrid-rag-engine/go-api/internal/retrieval"
	"hybrid-rag-engine/go-api/internal/vector"
)

func Run() {
	aiBaseURL := env("AI_SERVICE_URL", "http://localhost:8000")
	qdrantURL := env("QDRANT_URL", "http://localhost:6333")
	collection := env("QDRANT_COLLECTION", "documents")

	corpus := retrieval.SampleDocuments()
	bm25Index := bm25.NewIndex(corpus)
	vectorClient := vector.NewClient(qdrantURL, collection, 384)
	aiClient := retrieval.NewAIClient(aiBaseURL, 30*time.Second)
	orchestrator := retrieval.NewOrchestrator(
		bm25Index,
		vectorClient,
		aiClient,
		llm.NewSynthesizer(),
	)

	router := api.NewRouter(orchestrator, aiClient, vectorClient, bm25Index, corpus)
	log.Println("Go API running on :8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}

func env(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
