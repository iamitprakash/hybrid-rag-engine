package vector

import "testing"

func TestBuildFilterMapsMetadataAndTopLevelFields(t *testing.T) {
	filter := buildFilter(map[string]string{
		"tenant": "demo",
		"source": "docs",
	})

	must, ok := filter["must"].([]map[string]any)
	if !ok {
		t.Fatalf("expected must clauses, got %#v", filter["must"])
	}
	if len(must) != 2 {
		t.Fatalf("expected two filter clauses, got %d", len(must))
	}

	keys := map[string]bool{}
	for _, clause := range must {
		keys[clause["key"].(string)] = true
	}
	if !keys["metadata.tenant"] {
		t.Fatal("expected metadata tenant filter")
	}
	if !keys["source"] {
		t.Fatal("expected top-level source filter")
	}
}
