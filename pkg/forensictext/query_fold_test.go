package forensictext

import "testing"

func TestCaseInsensitiveQueryFoldsBothSides(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		query Query
		want  bool
	}{
		{"default stays exact", "Wire Transfer received", Query{LiteralText: "wire transfer", MatchSemantic: PhraseContains}, false},
		{"folded phrase", "Wire Transfer received", Query{LiteralText: "wire transfer", MatchSemantic: PhraseContains, CaseInsensitive: true}, true},
		{"folded the other way", "wire transfer received", Query{LiteralText: "WIRE Transfer", MatchSemantic: PhraseContains, CaseInsensitive: true}, true},
		{"folded exact phrase keeps its word boundaries", "the Invoices were late", Query{LiteralText: "invoice", MatchSemantic: ExactPhrase, CaseInsensitive: true}, false},
		{"folded exact phrase as a whole word", "the Invoice was late", Query{LiteralText: "invoice", MatchSemantic: ExactPhrase, CaseInsensitive: true}, true},
		{"folded token search reaches every token", "Payment for the INVOICE", Query{LiteralText: "invoice payment", MatchSemantic: TokenSearch, CaseInsensitive: true}, true},
		{"token search still needs every token", "Payment for the INVOICE", Query{LiteralText: "invoice receipt", MatchSemantic: TokenSearch, CaseInsensitive: true}, false},
		{"token search stays exact by default", "Payment for the INVOICE", Query{LiteralText: "invoice payment", MatchSemantic: TokenSearch}, false},
		{"scripts without case are unchanged", "اگوا شدگان کا ذکر", Query{LiteralText: "اگوا شدگان", MatchSemantic: PhraseContains, CaseInsensitive: true}, true},
		{"folded exact value", "ABC-123", Query{LiteralText: "abc-123", MatchSemantic: ExactValue, CaseInsensitive: true}, true},
	}
	for _, c := range cases {
		if got := Match(c.text, c.query); got != c.want {
			t.Errorf("%s: Match(%q, %+v) = %v, want %v", c.name, c.text, c.query, got, c.want)
		}
	}
}
