package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"hybrid-rag-engine/go-api/internal/retrieval"
	"hybrid-rag-engine/go-api/internal/vector"
)

func NewRouter(orchestrator *retrieval.Orchestrator, ai *retrieval.AIClient, vectorClient *vector.Client, corpus []retrieval.Document) *gin.Engine {
	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.POST("/search", func(c *gin.Context) {
		var req retrieval.SearchRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		resp, err := orchestrator.Search(c.Request.Context(), req.Query, req.TopK)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, resp)
	})

	router.POST("/ingest/sample", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
		defer cancel()

		if err := vectorClient.EnsureCollection(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		vectors := make([][]float32, 0, len(corpus))
		for _, doc := range corpus {
			embedding, err := ai.Embed(doc.Title + "\n" + doc.Content)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			vectors = append(vectors, embedding)
		}
		if err := vectorClient.Upsert(ctx, corpus, vectors); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ingested": len(corpus)})
	})

	return router
}
