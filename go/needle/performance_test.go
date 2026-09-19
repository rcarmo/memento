package needle

import "testing"

func BenchmarkTokenizerVocabularyLookups(b *testing.B) {
	tokenizer, err := TokenizerFromBytes(syntheticTokenizer(true))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = tokenizer.VocabSize()
		_, _ = tokenizer.TokenToID("▁")
	}
}

func TestTokenizerVocabularyLookupAllocationBudget(t *testing.T) {
	tokenizer, err := TokenizerFromBytes(syntheticTokenizer(true))
	if err != nil {
		t.Fatal(err)
	}
	allocations := testing.AllocsPerRun(1000, func() {
		_ = tokenizer.VocabSize()
		_, _ = tokenizer.TokenToID("▁")
	})
	if allocations != 0 {
		t.Fatalf("vocabulary lookup allocs/run = %v, want 0", allocations)
	}
}
