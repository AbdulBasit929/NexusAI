package forensictext

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// Tier says HOW a text matched a query. A search that returns everything an analyst could mean has to look past spelling that differs for reasons that are not the analyst's (a number written with other digits
// or separators, a word with a reading error), and it has to say which kind of match each hit is, so an exact match is never hidden behind a close one and a close one is never passed off as exact.
type Tier string

const (
	TierNone        Tier = ""
	TierExact       Tier = "exact"       // the text holds the literal as typed (the semantics of Match)
	TierNormalised  Tier = "normalised"  // the same words or number once case, Unicode form, digit script, Arabic-script letter variants and separators are folded
	TierApproximate Tier = "approximate" // within a small number of character edits of it: a likely reading error, to be checked by eye
)

// Rank orders the tiers, best first.
func (t Tier) Rank() int {
	switch t {
	case TierExact:
		return 3
	case TierNormalised:
		return 2
	case TierApproximate:
		return 1
	}
	return 0
}

// Fold is the comparison form of a text for tolerant matching: compatibility-normalised (Arabic presentation forms become letters), case-folded, Eastern Arabic and Persian digits become 0 to 9,
// the common Arabic and Urdu letter variants become one letter (yeh, kaf, heh), and marks, tatweel and zero-width characters are removed. It is used on BOTH sides of a comparison and never
// replaces retained evidence.
func Fold(value string) string {
	value = norm.NFKC.String(value)
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch {
		case r >= '٠' && r <= '٩':
			r = '0' + (r - '٠')
		case r >= '۰' && r <= '۹':
			r = '0' + (r - '۰')
		}
		switch r {
		case 'ي', 'ى': // Arabic yeh and alef maksura
			r = 'ی' // Farsi/Urdu yeh
		case 'ك': // Arabic kaf
			r = 'ک' // keheh
		case 'ه': // Arabic heh
			r = 'ہ' // Urdu heh goal
		case 'أ', 'إ', 'ٱ': // alef with hamza above or below, alef wasla
			r = 'ا'
		case 'ـ', '\u200b', '\u200c', '\u200d', '\u200e', '\u200f', '\u2060', '\ufeff': // tatweel, zero-width and direction marks
			continue
		}
		if r >= 'ً' && r <= 'ٟ' || r == 'ٰ' { // Arabic vowel marks
			continue
		}
		b.WriteRune(r)
	}
	return cases.Fold().String(b.String())
}

func wordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) }

// tokenize splits a folded text into runs of letters and digits; every separator (space, hyphen, slash, dot, bracket) ends a run.
func tokenize(folded string) []string {
	return strings.FieldsFunc(folded, func(r rune) bool { return !wordRune(r) })
}

func hasDigit(value string) bool {
	for _, r := range value {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func arabicScript(value string) bool {
	letters, arabic := 0, 0
	for _, r := range value {
		if unicode.IsLetter(r) {
			letters++
			if unicode.Is(unicode.Arabic, r) {
				arabic++
			}
		}
	}
	return letters > 0 && arabic*2 > letters
}

// editsAllowed is how many character edits a token may differ by and still count as a reading error of the literal. Short tokens must match exactly (a one-letter difference between two short words is a
// different word); a number may differ by one digit once it is long enough to be an identifier; Arabic-script words are shorter on average and are confused more often by readers, so they get the
// allowance one character sooner.
func editsAllowed(token string) int {
	n := utf8.RuneCountInString(token)
	switch {
	case hasDigit(token):
		if n >= 8 {
			return 1
		}
		return 0
	case arabicScript(token):
		if n >= 10 {
			return 2
		}
		if n >= 5 {
			return 1
		}
	default:
		if n >= 10 {
			return 2
		}
		if n >= 6 {
			return 1
		}
	}
	return 0
}

// withinEdits reports whether two strings are at most max single-character insertions, deletions or substitutions apart.
func withinEdits(a, b string, max int) bool {
	if a == b {
		return true
	}
	x, y := []rune(a), []rune(b)
	if len(x) > len(y) {
		x, y = y, x
	}
	if len(y)-len(x) > max {
		return false
	}
	previous := make([]int, len(x)+1)
	for i := range previous {
		previous[i] = i
	}
	for j := 1; j <= len(y); j++ {
		current := make([]int, len(x)+1)
		current[0] = j
		best := current[0]
		for i := 1; i <= len(x); i++ {
			cost := 1
			if x[i-1] == y[j-1] {
				cost = 0
			}
			current[i] = min(previous[i]+1, current[i-1]+1, previous[i-1]+cost)
			best = min(best, current[i])
		}
		if best > max {
			return false
		}
		previous = current
	}
	return previous[len(x)] <= max
}

// MatchTier tells whether, and how well, a text matches a query. The first tier is the existing Match; the others fold the writing (see Fold) and then compare whole tokens, so a number is found whether it
// is written 042-35761234, 04235761234, 042 35761234 or with other digits, a plate whether it is SYNT9150 or SYNT-۹۱۵۰, and a word in any case. Approximate matching is for tokens long enough to carry a
// reading error and never mixes two numbers: it allows at most one or two character edits in the whole phrase. The caller keeps the tiers apart.
func MatchTier(text string, q Query) Tier {
	if q.Validate() != nil {
		return TierNone
	}
	if Match(text, q) {
		return TierExact
	}
	switch q.MatchSemantic {
	case PhraseContains, ExactPhrase, TokenSearch:
	default:
		return TierNone
	}
	lit := tokenize(Fold(q.LiteralText))
	hay := tokenize(Fold(text))
	if len(lit) == 0 || len(hay) == 0 {
		return TierNone
	}
	if q.MatchSemantic == TokenSearch {
		tier := TierNormalised
		for _, word := range lit {
			switch wordTier(hay, word, false) {
			case TierNone:
				return TierNone
			case TierApproximate:
				tier = TierApproximate
			}
		}
		return tier
	}
	contains := q.MatchSemantic == PhraseContains
	best := sequenceTier(hay, lit, contains)
	if best == TierNormalised {
		return best
	}
	// A code written with other separators can be the better reading of a passage that also holds a one-digit-off token: the better tier wins.
	if tier := codeRunTier(hay, lit); tier.Rank() > best.Rank() {
		best = tier
	}
	return best
}

// numberForms are the ways one national telephone number is written in Pakistani documents: with the trunk zero (03000001005), with the country code (923000001005, as +92 300 0001005 or
// 92(300)0001005) and with the international prefix (0092 300 0001005). The equivalence is a fact about how the number is written, not about the question: the same subscriber is meant.
// A code that is not all digits, or whose length does not fit a national number of nine or ten digits after the zero, has no other forms.
func numberForms(code string) []string {
	for _, r := range code {
		if r < '0' || r > '9' {
			return nil
		}
	}
	national := ""
	switch {
	case strings.HasPrefix(code, "0092") && len(code) >= 13 && len(code) <= 14:
		national = "0" + code[4:]
	case strings.HasPrefix(code, "92") && len(code) >= 11 && len(code) <= 12:
		national = "0" + code[2:]
	case strings.HasPrefix(code, "0") && !strings.HasPrefix(code, "00") && len(code) >= 10 && len(code) <= 11:
		national = code
	}
	if national == "" {
		return nil
	}
	return []string{national, "92" + national[1:], "0092" + national[1:]}
}

// wordTier finds one literal token among the text's tokens: equal (or, for a contains search, holding it) is normalised; within the allowed edits is approximate.
func wordTier(hay []string, word string, contains bool) Tier {
	best := TierNone
	for _, token := range hay {
		if token == word || contains && utf8.RuneCountInString(word) >= 3 && strings.Contains(token, word) {
			return TierNormalised
		}
		if allowed := editsAllowed(word); allowed > 0 && best == TierNone && withinEdits(token, word, allowed) {
			best = TierApproximate
		}
	}
	return best
}

// sequenceTier finds the literal's tokens as consecutive tokens of the text.
func sequenceTier(hay, lit []string, contains bool) Tier {
	if len(lit) == 1 {
		return wordTier(hay, lit[0], contains)
	}
	best := TierNone
	for start := 0; start+len(lit) <= len(hay); start++ {
		exact, edits, ok := true, 0, true
		for offset, word := range lit {
			token := hay[start+offset]
			switch {
			case token == word:
			case contains && offset == 0 && strings.HasSuffix(token, word), contains && offset == len(lit)-1 && strings.HasPrefix(token, word):
			default:
				exact = false
				allowed := editsAllowed(word)
				if allowed == 0 || !withinEdits(token, word, allowed) {
					ok = false
				} else {
					edits++
				}
			}
			if !ok {
				break
			}
		}
		if !ok {
			continue
		}
		if exact {
			return TierNormalised
		}
		if edits <= 2 {
			best = TierApproximate
		}
	}
	return best
}

// codeRunTier finds the literal as ONE code or number that the text writes with other separators: consecutive tokens of the text whose concatenation equals the literal's tokens concatenated. Boundaries
// have to fall on token edges, so two neighbouring numbers never combine into a match.
func codeRunTier(hay, lit []string) Tier {
	code := strings.Join(lit, "")
	if utf8.RuneCountInString(code) < 4 {
		return TierNone
	}
	allowed := editsAllowed(code)
	forms := numberForms(code)
	longest := utf8.RuneCountInString(code)
	for _, form := range forms {
		longest = max(longest, utf8.RuneCountInString(form))
	}
	best := TierNone
	for start := range hay {
		joined := ""
		for end := start; end < len(hay) && end < start+len(lit)+5; end++ {
			joined += hay[end]
			if joined == code {
				return TierNormalised
			}
			for _, form := range forms {
				if joined == form {
					return TierNormalised
				}
			}
			if allowed > 0 && best == TierNone && withinEdits(joined, code, allowed) && utf8.RuneCountInString(joined) >= utf8.RuneCountInString(code)-allowed {
				best = TierApproximate
			}
			if utf8.RuneCountInString(joined) > longest+allowed {
				break
			}
		}
	}
	return best
}
