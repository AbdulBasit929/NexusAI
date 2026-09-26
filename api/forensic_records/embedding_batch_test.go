package main

import (
	"context"
	"errors"
	"testing"
)

// recordingEmbedder captures how the catalogue was chunked and returns
// deterministic vectors so order can be checked.
type recordingEmbedder struct {
	calls [][]string
	fail  bool
}

func (r *recordingEmbedder) Embed(_ context.Context, inputs []string) ([][]float64, error) {
	if r.fail {
		return nil, errors.New("embedding backend unavailable")
	}
	r.calls = append(r.calls, append([]string(nil), inputs...))
	out := make([][]float64, len(inputs))
	for i, in := range inputs {
		out[i] = []float64{float64(len(in)), float64(i)}
	}
	return out, nil
}

// A 66-descriptor catalogue issued 66 sequential HTTP calls -- ~17 s per
// question on questions whose SQL took under half a second.
func TestEmbeddingBatchChunksTheCatalogue(t *testing.T) {
	descriptors := make([]semanticEmbeddingDescriptor, 66)
	for i := range descriptors {
		descriptors[i] = semanticEmbeddingDescriptor{ID: string(rune('a'+i%26)) + string(rune('0'+i/26)), Text: "field text"}
	}
	rec := &recordingEmbedder{}
	entry := &semanticEmbeddingCacheEntry{ready: make(chan struct{})}
	populateSemanticDescriptorEmbeddings(rec, "k", descriptors, entry)
	<-entry.ready

	if entry.err != nil {
		t.Fatalf("populate failed: %v", entry.err)
	}
	if len(rec.calls) > 3 {
		t.Errorf("66 descriptors took %d calls; batching is meant to keep this at 3", len(rec.calls))
	}
	for _, call := range rec.calls {
		if len(call) > semanticEmbeddingCatalogBatch {
			t.Errorf("a batch carried %d inputs, above the %d limit", len(call), semanticEmbeddingCatalogBatch)
		}
	}
	if len(entry.vectors) != len(descriptors) {
		t.Errorf("resolved %d vectors for %d descriptors", len(entry.vectors), len(descriptors))
	}
}

// The batch loop appends each response in order, so a backend that reordered
// its output would silently mis-assign every vector to the wrong field. That
// would be a WRONG retrieval ranking, not a crash -- assert the contract.
func TestEmbeddingBatchPreservesInputOrder(t *testing.T) {
	descriptors := []semanticEmbeddingDescriptor{
		{ID: "one", Text: "a"}, {ID: "two", Text: "bb"}, {ID: "three", Text: "ccc"},
	}
	entry := &semanticEmbeddingCacheEntry{ready: make(chan struct{})}
	populateSemanticDescriptorEmbeddings(&recordingEmbedder{}, "k", descriptors, entry)
	<-entry.ready

	// recordingEmbedder encodes len(text) as the first component, so each
	// descriptor must carry the vector built from ITS OWN text.
	for _, want := range []struct {
		id  string
		len float64
	}{{"one", 1}, {"two", 2}, {"three", 3}} {
		vector, ok := entry.vectors[want.id]
		if !ok {
			t.Fatalf("descriptor %q resolved no vector", want.id)
		}
		if vector[0] != want.len {
			t.Errorf("descriptor %q got the vector for a different field (%v)", want.id, vector[0])
		}
	}
}

// A failed batch must resolve nothing rather than a partial catalogue: half an
// embedding catalogue would rank fields against vectors that do not exist.
func TestEmbeddingBatchFailureResolvesNothing(t *testing.T) {
	entry := &semanticEmbeddingCacheEntry{ready: make(chan struct{})}
	populateSemanticDescriptorEmbeddings(&recordingEmbedder{fail: true}, "k",
		[]semanticEmbeddingDescriptor{{ID: "one", Text: "a"}}, entry)
	<-entry.ready
	if len(entry.vectors) != 0 {
		t.Errorf("a failed catalogue must resolve no vectors, got %d", len(entry.vectors))
	}
}
