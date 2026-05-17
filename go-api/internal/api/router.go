package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"hybrid-rag-engine/go-api/internal/bm25"
	"hybrid-rag-engine/go-api/internal/cache"
	"hybrid-rag-engine/go-api/internal/chunking"
	"hybrid-rag-engine/go-api/internal/metadata"
	"hybrid-rag-engine/go-api/internal/metrics"
	"hybrid-rag-engine/go-api/internal/quality"
	"hybrid-rag-engine/go-api/internal/retrieval"
	"hybrid-rag-engine/go-api/internal/tenantauth"
	"hybrid-rag-engine/go-api/internal/vector"
)

type ingestRequest struct {
	Documents []chunking.RawDocument `json:"documents" binding:"required"`
	Chunking  chunking.Options       `json:"chunking"`
}

func NewRouter(orchestrator *retrieval.Orchestrator, ai *retrieval.AIClient, vectorClient *vector.Client, bm25Index *bm25.Index, cacheClient *cache.Client, metadataStore *metadata.Store, metricsRecorder *metrics.Recorder, auth *tenantauth.Auth, corpus []retrieval.Document) *gin.Engine {
	router := gin.Default()

	router.Use(func(c *gin.Context) {
		if c.Request.URL.Path == "/health" {
			c.Next()
			return
		}
		if auth == nil || !auth.Enabled() {
			c.Next()
			return
		}
		apiKey := c.GetHeader("X-API-Key")
		tenant, ok := auth.TenantForKey(apiKey)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid api key"})
			return
		}
		c.Set("tenant", tenant)
		c.Next()
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.POST("/search", func(c *gin.Context) {
		var req retrieval.SearchRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		req.Filters = scopedFilters(req.Filters, tenantFromContext(c))
		key := cache.Key("search", req)
		var cached retrieval.SearchResponse
		if ok, err := cacheClient.GetJSON(c.Request.Context(), key, &cached); err == nil && ok {
			cached.Trace.CacheHit = true
			metricsRecorder.RecordCacheHit()
			c.JSON(http.StatusOK, cached)
			return
		}

		started := time.Now()
		resp, err := orchestrator.SearchWithFilters(c.Request.Context(), req)
		if err != nil {
			metricsRecorder.RecordError()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		resp.Grounding = quality.Evaluate(resp.Answer, resp.Documents)
		metricsRecorder.RecordSearch(time.Since(started), resp.Grounding.GroundedTokenRatio, resp.Grounding.HallucinationRisk)
		_ = cacheClient.SetJSON(c.Request.Context(), key, resp)
		c.JSON(http.StatusOK, resp)
	})

	router.POST("/search/stream", func(c *gin.Context) {
		var req retrieval.SearchRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		req.Filters = scopedFilters(req.Filters, tenantFromContext(c))
		tokens, errs, docs, trace, err := orchestrator.StreamWithFilters(c.Request.Context(), req)
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

		tenant := tenantFromContext(c)
		docs := withTenant(corpus, tenant)
		if err := embedAndUpsert(ctx, ai, vectorClient, docs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := metadataStore.UpsertChunks(ctx, docs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		bm25Index.AddDocuments(docs)
		c.JSON(http.StatusOK, gin.H{"ingested": len(docs), "tenant": tenant})
	})

	router.POST("/ingest", func(c *gin.Context) {
		var req ingestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		tenant := tenantFromContext(c)
		req.Documents = withTenantRawDocuments(req.Documents, tenant)
		chunks := chunking.ChunkDocuments(req.Documents, req.Chunking)
		if len(chunks) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no chunks produced from input documents"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
		defer cancel()

		if err := metadataStore.UpsertRawDocuments(ctx, req.Documents); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := embedAndUpsert(ctx, ai, vectorClient, chunks); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := metadataStore.UpsertChunks(ctx, chunks); err != nil {
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
		tenant := tenantFromContext(c)
		if metadataStore.Enabled() {
			docs, err := metadataStore.ListChunks(c.Request.Context(), 100, tenant)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"count":     len(docs),
				"documents": docs,
				"store":     "postgres",
			})
			return
		}
		docs := filterDocuments(bm25Index.Documents(), scopedFilters(nil, tenant))
		c.JSON(http.StatusOK, gin.H{
			"count":     len(docs),
			"documents": docs,
			"store":     "memory",
		})
	})

	router.GET("/metrics", func(c *gin.Context) {
		c.JSON(http.StatusOK, metricsRecorder.Snapshot())
	})

	return router
}

func tenantFromContext(c *gin.Context) string {
	value, _ := c.Get("tenant")
	tenant, _ := value.(string)
	return tenant
}

func scopedFilters(filters map[string]string, tenant string) map[string]string {
	if tenant == "" {
		return filters
	}
	scoped := map[string]string{"tenant": tenant}
	for key, value := range filters {
		if key == "tenant" {
			continue
		}
		scoped[key] = value
	}
	return scoped
}

func withTenant(docs []retrieval.Document, tenant string) []retrieval.Document {
	if tenant == "" {
		return docs
	}
	scoped := make([]retrieval.Document, 0, len(docs))
	for _, doc := range docs {
		cloned := doc
		if cloned.Metadata == nil {
			cloned.Metadata = map[string]string{}
		}
		cloned.Metadata["tenant"] = tenant
		scoped = append(scoped, cloned)
	}
	return scoped
}

func withTenantRawDocuments(docs []chunking.RawDocument, tenant string) []chunking.RawDocument {
	if tenant == "" {
		return docs
	}
	scoped := make([]chunking.RawDocument, 0, len(docs))
	for _, doc := range docs {
		cloned := doc
		if cloned.Metadata == nil {
			cloned.Metadata = map[string]string{}
		}
		cloned.Metadata["tenant"] = tenant
		scoped = append(scoped, cloned)
	}
	return scoped
}

func filterDocuments(docs []retrieval.Document, filters map[string]string) []retrieval.Document {
	if len(filters) == 0 {
		return docs
	}
	filtered := make([]retrieval.Document, 0, len(docs))
	for _, doc := range docs {
		if !documentMatchesFilters(doc, filters) {
			continue
		}
		filtered = append(filtered, doc)
	}
	return filtered
}

func documentMatchesFilters(doc retrieval.Document, filters map[string]string) bool {
	for key, value := range filters {
		switch key {
		case "source":
			if doc.Source != value {
				return false
			}
		case "id":
			if doc.ID != value {
				return false
			}
		default:
			if doc.Metadata == nil || doc.Metadata[key] != value {
				return false
			}
		}
	}
	return true
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
