package retrieval

type Document struct {
	ID       string            `json:"id"`
	Title    string            `json:"title"`
	Content  string            `json:"content"`
	Source   string            `json:"source,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Score    float64           `json:"score,omitempty"`
}

type SearchRequest struct {
	Query   string            `json:"query" binding:"required"`
	TopK    int               `json:"top_k"`
	Filters map[string]string `json:"filters,omitempty"`
}

type SearchResponse struct {
	Query     string     `json:"query"`
	Answer    string     `json:"answer"`
	Documents []Document `json:"documents"`
	Trace     Trace      `json:"trace"`
}

type Trace struct {
	BM25Hits     int  `json:"bm25_hits"`
	VectorHits   int  `json:"vector_hits"`
	FusedHits    int  `json:"fused_hits"`
	RerankedHits int  `json:"reranked_hits"`
	CacheHit     bool `json:"cache_hit"`
}

func SampleDocuments() []Document {
	return []Document{
		{
			ID:      "rag-001",
			Title:   "Hybrid Retrieval",
			Content: "Hybrid RAG combines lexical BM25 search with semantic vector retrieval to improve recall before reranking.",
			Source:  "sample-corpus",
		},
		{
			ID:      "rag-002",
			Title:   "Reranking Pipeline",
			Content: "A cross-encoder reranker scores query and document pairs after initial retrieval to prioritize precision.",
			Source:  "sample-corpus",
		},
		{
			ID:      "rag-003",
			Title:   "Go Orchestration",
			Content: "The Go API coordinates request lifecycle, concurrent retrieval, rank fusion, and LLM response synthesis.",
			Source:  "sample-corpus",
		},
		{
			ID:      "rag-004",
			Title:   "Vector Search",
			Content: "Qdrant stores dense embeddings and returns nearest document chunks for semantic similarity search.",
			Source:  "sample-corpus",
		},
		{
			ID:      "rag-005",
			Title:   "LLM Synthesis",
			Content: "The synthesis layer assembles retrieved context and creates a grounded final answer for the user query.",
			Source:  "sample-corpus",
		},
	}
}
