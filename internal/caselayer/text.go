package caselayer

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// nationalIDDashed is a Pakistani national ID number in its written form, 5-7-1 digits. The engine counts and withholds them; the case layer does not store them either.
var nationalIDDashed = regexp.MustCompile(`\b\d{5}-\d{7}-\d\b`)

const nationalIDMask = "#####-#######-#"

// MaskNationalIDs replaces each dashed national ID number with a mask and says how many it found. The question that is stored and searched is the masked one; the engine receives the original and
// refuses it, as it always has.
func MaskNationalIDs(s string) (string, int) {
	n := len(nationalIDDashed.FindAllStringIndex(s, -1))
	if n == 0 {
		return s, 0
	}
	return nationalIDDashed.ReplaceAllString(s, nationalIDMask), n
}

// scrubText masks every dashed national ID number in a text the layer stores.
func scrubText(s string) string {
	out, _ := MaskNationalIDs(s)
	return out
}

// cleanLine trims s and checks it is one line of between min and max characters with no control character.
func cleanLine(name, s string, min, max int) (string, error) {
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	switch {
	case n < min:
		if min == 1 {
			return "", BadRequest("%s is required.", name)
		}
		return "", BadRequest("%s must have at least %d characters.", name, min)
	case n > max:
		return "", BadRequest("%s is longer than %d characters.", name, max)
	case strings.IndexFunc(s, unicode.IsControl) >= 0:
		return "", BadRequest("%s must not contain control characters or line breaks.", name)
	case !utf8.ValidString(s):
		return "", BadRequest("%s is not valid text.", name)
	}
	return s, nil
}

// cleanBody is cleanLine for text that may span lines and keep its tabs.
func cleanBody(name, s string, min, max int) (string, error) {
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	switch {
	case n < min:
		return "", BadRequest("%s is required.", name)
	case n > max:
		return "", BadRequest("%s is longer than %d characters.", name, max)
	case !utf8.ValidString(s):
		return "", BadRequest("%s is not valid text.", name)
	}
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return "", BadRequest("%s must not contain control characters.", name)
		}
	}
	return s, nil
}

// clip shortens s to at most n characters for a title or a snippet, with an ellipsis, without cutting a character.
func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:n-1])) + "…"
}

// likePattern escapes the three characters that mean something to LIKE, so a word typed by an analyst is searched as written. The folding of the word is done by the database
// (forensic.search_normalize), the same function that folded the stored text.
func likePattern(word string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(word)
}

// searchWords splits a search text into at most eight words.
func searchWords(q string) []string {
	words := strings.Fields(q)
	if len(words) > 8 {
		words = words[:8]
	}
	out := make([]string, 0, len(words))
	for _, w := range words {
		if utf8.RuneCountInString(w) > 80 {
			w = string([]rune(w)[:80])
		}
		out = append(out, likePattern(w))
	}
	return out
}
