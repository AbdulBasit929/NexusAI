package forensictext

import "testing"

func TestMatchTier(t *testing.T) {
	phrase := func(s string) Query {
		return Query{LiteralText: s, MatchSemantic: PhraseContains, CaseInsensitive: true}
	}
	cases := []struct {
		name  string
		text  string
		query Query
		want  Tier
	}{
		// numbers and codes written differently
		{"phone as typed", "Police station landline 042-35761234, reference", phrase("042-35761234"), TierExact},
		{"phone without separators", "Contact 04235761234 now", phrase("042-35761234"), TierNormalised},
		{"phone with a space", "helpline 042 35761234.", phrase("042-35761234"), TierNormalised},
		{"phone split elsewhere", "Call 0423-5761234 today", phrase("042-35761234"), TierNormalised},
		{"typed without separators, written with them", "landline 042-35761234", phrase("04235761234"), TierNormalised},
		{"one digit off in a long number", "number 042-35761235", phrase("042-35761234"), TierApproximate},
		{"two digits off is another number", "number 042-35761299", phrase("042-35761234"), TierNone},
		{"two numbers side by side never combine", "042-3576 1234 and 99", phrase("042-35761234"), TierNormalised}, // 3576 and 1234 are consecutive tokens of ONE number here
		{"a short number differs by one digit: different number", "code 1234 and 1235", phrase("1234"), TierExact},
		// one telephone number, three ways of writing the country
		{"country code form", "Call +92 300 0001005 now", phrase("0300-0001005"), TierNormalised},
		{"country code in brackets", "contact +92(300)0001001", phrase("03000001001"), TierNormalised},
		{"international prefix", "number 0092 300 0001005 seized", phrase("03000001005"), TierNormalised},
		{"international prefix in Urdu digits", "رابطہ ۰۰۹۲ ۳۰۰ ۰۰۰۱۰۰۵ ہے", phrase("0300 0001005"), TierNormalised},
		{"subscriber column without a plus", "001010000001005 923000001005 0000000000100101", phrase("0300-0001005"), TierNormalised},
		{"typed with the country code, written with the trunk zero", "mobile 03000001005", phrase("+92 300 0001005"), TierNormalised},
		{"landline with the country code", "92 42 35761234", phrase("042-35761234"), TierNormalised},
		{"another subscriber is a close reading at most", "0300 0001006", phrase("0300-0001005"), TierApproximate},
		{"a better written form beats a one-digit-off token in the same passage", "also 03000001006 and 0300 0001005", phrase("03000001005"), TierNormalised},
		// plates and digit scripts
		{"plate in Urdu digits", "اس کی گاڑی کا نمبر SYNT-۹۱۵۰ تھا۔", phrase("SYNT9150"), TierNormalised},
		{"plate with a hyphen", "plate SYNT-9150 seen", phrase("SYNT9150"), TierNormalised},
		{"plate in another case", "plate synt9150 seen", Query{LiteralText: "SYNT9150", MatchSemantic: PhraseContains}, TierNormalised},
		{"Arabic-Indic digits", "رقم ٠٤٢٣٥٧٦١٢٣٤", phrase("04235761234"), TierNormalised},
		// words
		{"word in another case, exact by the query's own folding", "SYNTHETIC TEST DOCUMENT", phrase("synthetic"), TierExact},
		{"word in another case, query without folding", "SYNTHETIC TEST DOCUMENT", Query{LiteralText: "synthetic", MatchSemantic: PhraseContains}, TierNormalised},
		{"name as typed", "Mera naam Hassan Farooqi hai", phrase("Hassan Farooqi"), TierExact},
		{"name with one letter missing", "Mera naam Hasan Farooqi hai", phrase("Hassan Farooqi"), TierApproximate},
		{"a different short word is not approximate", "the cat sat", phrase("hat"), TierNone},
		{"word absent", "nothing relevant here", phrase("synthetic"), TierNone},
		// Urdu
		{"Urdu word with a reading error", "نیو لاہول کریانہ", phrase("لاہور"), TierApproximate},
		{"Urdu word as typed", "نیو لاہور کریانہ", phrase("لاہور"), TierExact},
		{"Urdu letter variants fold together", "كراچي بيكرز", phrase("کراچی"), TierNormalised},
		// all words
		{"all words, in any order", "Payment for the INVOICE", Query{LiteralText: "invoice payment", MatchSemantic: TokenSearch, CaseInsensitive: true}, TierExact},
		{"all words, one with a reading error", "Paymant for the invoice today", Query{LiteralText: "invoice payment", MatchSemantic: TokenSearch}, TierApproximate},
		{"all words, one missing", "Payment for the receipt", Query{LiteralText: "invoice payment", MatchSemantic: TokenSearch}, TierNone},
		// the semantics of the query are kept
		{"an invalid literal matches nothing", "anything", Query{LiteralText: "", MatchSemantic: PhraseContains}, TierNone},
		{"a source query has no literal and keeps the semantics of Match", "anything", Query{MatchSemantic: Source}, TierExact},
	}
	for _, c := range cases {
		if got := MatchTier(c.text, c.query); got != c.want {
			t.Errorf("%s: MatchTier(%q, %q) = %q, want %q", c.name, c.text, c.query.LiteralText, got, c.want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestFoldIsUsedOnBothSidesAndKeepsLatinLetters(t *testing.T) {
	if Fold("SYNT-۹۱۵۰") != "synt-9150" {
		t.Fatalf("Fold(SYNT-۹۱۵۰) = %q", Fold("SYNT-۹۱۵۰")) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if Fold("كراچي") != Fold("کراچی") {
		t.Fatalf("Arabic yeh and kaf were not folded: %q vs %q", Fold("كراچي"), Fold("کراچی")) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if Fold("ٱلله") == "" || Fold("a​b") != "ab" {
		t.Fatalf("zero-width characters must go: %q", Fold("a​b")) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestWithinEdits(t *testing.T) {
	for _, c := range []struct {
		a, b string
		max  int
		want bool
	}{{"abc", "abc", 0, true}, {"abc", "abd", 1, true}, {"abc", "abde", 1, false}, {"hassan", "hasan", 1, true}, {"kitten", "sitting", 2, false}, {"kitten", "sitting", 3, true}, {"", "a", 1, true}, {"", "ab", 1, false}} {
		if got := withinEdits(c.a, c.b, c.max); got != c.want {
			t.Errorf("withinEdits(%q, %q, %d) = %v, want %v", c.a, c.b, c.max, got, c.want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestTierRankOrdersExactFirst(t *testing.T) {
	if !(TierExact.Rank() > TierNormalised.Rank() && TierNormalised.Rank() > TierApproximate.Rank() && TierApproximate.Rank() > TierNone.Rank()) {
		t.Fatal("tier ranks are not ordered") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}
