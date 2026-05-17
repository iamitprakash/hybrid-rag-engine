package cache

import "testing"

func TestKeyIsStable(t *testing.T) {
	payload := map[string]any{"query": "rag", "top_k": 5}
	first := Key("search", payload)
	second := Key("search", payload)

	if first != second {
		t.Fatalf("expected stable keys, got %s and %s", first, second)
	}
	if first == "search:" {
		t.Fatal("expected hashed suffix")
	}
}
