package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"hybrid-rag-engine/go-api/internal/bm25"
	"hybrid-rag-engine/go-api/internal/chunking"
	"hybrid-rag-engine/go-api/internal/retrieval"
	"hybrid-rag-engine/go-api/internal/vector"
)

type ingestRequest struct {
	Documents []chunking.RawDocument `json:"documents" binding:"required"`
	Chunking  chunking.Options       `json:"chunking"`
}

func NewRouter(orchestrator *retrieval.Orchestrator, ai *retrieval.AIClient, vectorClient *vector.Client, bm25Index *bm25.Index, corpus []retrieval.Document) *gin.Engine {
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

	router.POST("/search/stream", func(c *gin.Context) {
		var req retrieval.SearchRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		tokens, errs, docs, trace, err := orchestrator.Stream(c.Request.Context(), req.Query, req.TopK)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")

		c.SSEvent("retrieval", gin.H{
			"query":     req.Query,
			"trace":     trace,
			"documents": docs,
		})
		c.Writer.Flush()

		for {
			select {
			case token, ok := <-tokens:
				if !ok {
					if err := <-errs; err != nil {
						c.SSEvent("error", gin.H{"error": err.Error()})
					}
					c.SSEvent("done", gin.H{"ok": true})
					c.Writer.Flush()
					return
				}
				c.SSEvent("token", token)
				c.Writer.Flush()
			case <-c.Request.Context().Done():
				return
			}
		}
	})

	router.POST("/ingest/sample", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
		defer cancel()

		if err := embedAndUpsert(ctx, ai, vectorClient, corpus); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ingested": len(corpus)})
	})

	router.POST("/ingest", func(c *gin.Context) {
		var req ingestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		chunks := chunking.ChunkDocuments(req.Documents, req.Chunking)
		if len(chunks) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no chunks produced from input documents"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
		defer cancel()

		if err := embedAndUpsert(ctx, ai, vectorClient, chunks); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		bm25Index.AddDocuments(chunks)
		c.JSON(http.StatusOK, gin.H{
			"documents": len(req.Documents),
			"chunks":    len(chunks),
		})
	})

	router.GET("/documents", func(c *gin.Context) {
		docs := bm25Index.Documents()
		c.JSON(http.StatusOK, gin.H{
			"count":     len(docs),
			"documents": docs,
		})
	})

	return router
}

func embedAndUpsert(ctx context.Context, ai *retrieval.AIClient, vectorClient *vector.Client, docs []retrieval.Document) error {
	if err := vectorClient.EnsureCollection(ctx); err != nil {
		return err
	}
	vectors := make([][]float32, 0, len(docs))
	for _, doc := range docs {
		embedding, err := ai.Embed(doc.Title + "\n" + doc.Content)
		if err != nil {
			return err
		}
		vectors = append(vectors, embedding)
	}
	return vectorClient.Upsert(ctx, docs, vectors)
}
