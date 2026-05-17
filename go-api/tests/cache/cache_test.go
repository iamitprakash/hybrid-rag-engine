package cachetest

import (
	"context"
	"testing"

	"hybrid-rag-engine/go-api/internal/cache"
)

func TestKeyIsStable(t *testing.T) {
	payload := map[string]any{"query": "rag", "top_k": 5}
	first := cache.Key("search", payload)
	second := cache.Key("search", payload)

	if first != second {
		t.Fatalf("expected stable keys, got %s and %s", first, second)
	}
	if first == "search:" {
		t.Fatal("expected hashed suffix")
	}
}

func TestDisabledCacheReturnsMiss(t *testing.T) {
	client, err := cache.New("", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var out map[string]any
	hit, err := client.GetJSON(context.Background(), "k", &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hit {
		t.Fatal("expected cache miss when cache is disabled")
	}
	if err := client.SetJSON(context.Background(), "k", map[string]string{"a": "b"}); err != nil {
		t.Fatalf("unexpected set error: %v", err)
	}
	if client.Enabled() {
		t.Fatal("expected disabled cache client")
	}
}
