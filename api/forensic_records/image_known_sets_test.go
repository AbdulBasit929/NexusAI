package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKnownCSVParsing(t *testing.T) {
	sha1, sha2 := strings.Repeat("a", 64), strings.Repeat("B", 64)
	pdq := strings.Repeat("0f", 32)

	t.Run("positional columns, comments, blank lines, a byte-order mark", func(t *testing.T) {
		rows, rejected := parseKnownHashCSV("\ufeff# a list\n" + sha1 + "," + pdq + ",logo\n\n" + sha2 + ",,stock photo\r\n")
		if len(rejected) != 0 || len(rows) != 2 {
			t.Fatalf("rows %v rejected %v", rows, rejected)
		}
		if rows[0].SHA256 != sha1 || rows[0].PDQ256 != pdq || rows[0].Label != "logo" || rows[1].PDQ256 != "" || rows[1].Label != "stock photo" {
			t.Errorf("rows %+v", rows)
		}
	})
	t.Run("a header names the columns in any order, tab and semicolon separators work", func(t *testing.T) {
		rows, rejected := parseKnownHashCSV("Label\tPDQ256\tSHA-256\nbanner\t" + pdq + "\t" + sha1 + "\n")
		if len(rejected) != 0 || len(rows) != 1 || rows[0].Label != "banner" || rows[0].SHA256 != sha1 || rows[0].PDQ256 != pdq {
			t.Fatalf("rows %+v rejected %v", rows, rejected)
		}
		rows, _ = parseKnownHashCSV("sha256;label\n" + sha1 + ";semi\n")
		if len(rows) != 1 || rows[0].Label != "semi" {
			t.Errorf("semicolon rows %+v", rows)
		}
	})
	t.Run("unusable rows are rejected with their line number and the rest are kept", func(t *testing.T) {
		rows, rejected := parseKnownHashCSV("sha256,pdq256,label\n" + sha1 + ",,ok\nnot-a-hash,,bad\n,,empty\n" + sha2 + ",zz,bad pdq\n")
		if len(rows) != 1 || len(rejected) != 3 {
			t.Fatalf("rows %+v rejected %+v", rows, rejected)
		}
		if rejected[0].Line != 3 || rejected[1].Line != 4 || rejected[2].Line != 5 {
			t.Errorf("rejected lines %+v", rejected)
		}
	})
	t.Run("a file with only hashes and no header works", func(t *testing.T) {
		rows, rejected := parseKnownHashCSV(sha1 + "\n" + sha2 + "\n")
		if len(rows) != 2 || len(rejected) != 0 {
			t.Fatalf("rows %+v rejected %+v", rows, rejected)
		}
	})
}

func TestNormaliseKnownHash(t *testing.T) {
	sha, bits, label, err := normaliseKnownHash(knownHashInput{SHA256: " " + strings.Repeat("A", 64) + " ", PDQ256: strings.Repeat("ff", 32), Label: "  x  "})
	if err != nil || sha != strings.Repeat("a", 64) || bits != strings.Repeat("1", 256) || label != "x" {
		t.Fatalf("sha %q bits %d label %q err %v", sha, len(bits), label, err)
	}
	for name, in := range map[string]knownHashInput{
		"nothing":        {},
		"short sha":      {SHA256: "abc"},
		"non-hex sha":    {SHA256: strings.Repeat("g", 64)},
		"short pdq":      {PDQ256: "ff"},
		"label only row": {Label: "only a label"},
	} {
		if _, _, _, err := normaliseKnownHash(in); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	_, _, long, err := normaliseKnownHash(knownHashInput{SHA256: strings.Repeat("a", 64), Label: strings.Repeat("é", 600)})
	if err != nil || len([]rune(long)) != knownSetMaxLabelRunes {
		t.Errorf("a long label is cut to %d characters, got %d (%v)", knownSetMaxLabelRunes, len([]rune(long)), err)
	}
}

func TestKnownCoverageStatement(t *testing.T) {
	cases := []struct {
		in   knownCoverage
		want string
	}{
		{knownCoverage{}, "No known set is active, so nothing was checked."},
		{knownCoverage{ActiveSets: 2, KnownHashes: 40, ImagesInCase: 97, ImagesWithHash: 97}, "Checked 97 of 97 images against 40 hashes in 2 sets."},
		{knownCoverage{ActiveSets: 1, KnownHashes: 1, ImagesInCase: 1, ImagesWithHash: 1}, "Checked 1 of 1 image against 1 hash in 1 set."},
		{knownCoverage{ActiveSets: 1, KnownHashes: 2, ImagesInCase: 6, ImagesWithHash: 5}, "Checked 5 of 6 images against 2 hashes in 1 set. 1 image has no perceptual hash yet and was checked by file hash only."},
		{knownCoverage{ActiveSets: 1, KnownHashes: 2, ImagesInCase: 9, ImagesWithHash: 5}, "Checked 5 of 9 images against 2 hashes in 1 set. 4 images have no perceptual hash yet and were checked by file hash only."},
	}
	for _, c := range cases {
		if got := knownCoverageStatement(c.in); got != c.want {
			t.Errorf("got %q want %q", got, c.want)
		}
	}
}

func TestKnownSetHandlersRefuseBadInputBeforeTouchingTheDatabase(t *testing.T) {
	k := knownSetsDeps{hashMax: 56}
	call := func(handler http.HandlerFunc, method, target, body, id string) int {
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		if id != "" {
			req.SetPathValue("id", id)
		}
		rec := httptest.NewRecorder()
		handler(rec, req)
		return rec.Code
	}
	uuid1 := "39000000-0000-4000-8000-000000000001"
	checks := []struct {
		name string
		got  int
		want int
	}{
		{"a set kind that does not exist", call(k.create, "POST", "/images/known-sets", `{"name":"x","kind":"faces"}`, ""), http.StatusBadRequest},
		{"an empty name", call(k.create, "POST", "/images/known-sets", `{"name":"  ","kind":"of_interest"}`, ""), http.StatusBadRequest},
		{"an unknown field", call(k.create, "POST", "/images/known-sets", `{"name":"x","kind":"of_interest","extra":1}`, ""), http.StatusBadRequest},
		{"not JSON", call(k.create, "POST", "/images/known-sets", `name=x`, ""), http.StatusBadRequest},
		{"a valid set without a database", call(k.create, "POST", "/images/known-sets", `{"name":"x","kind":"of_interest"}`, ""), http.StatusServiceUnavailable},
		{"hashes with a bad set id", call(k.addHashes, "POST", "/images/known-sets/x/hashes", `{"csv":"a"}`, "x"), http.StatusBadRequest},
		{"hashes with nothing in them", call(k.addHashes, "POST", "/images/known-sets/"+uuid1+"/hashes", `{}`, uuid1), http.StatusBadRequest},
		{"hashes naming a bad evidence id", call(k.addHashes, "POST", "/images/known-sets/"+uuid1+"/hashes", `{"evidence_ids":["nope"]}`, uuid1), http.StatusBadRequest},
		{"evidence ids without a collection", call(k.addHashes, "POST", "/images/known-sets/"+uuid1+"/hashes", `{"evidence_ids":["`+uuid1+`"]}`, uuid1), http.StatusBadRequest},
		{"retire with a bad set id", call(k.retire, "POST", "/images/known-sets/x/retire", ``, "x"), http.StatusBadRequest},
		{"flags without a collection", call(k.flags, "GET", "/images/known", ``, ""), http.StatusBadRequest},
		{"flags with a bad set filter", call(k.flags, "GET", "/images/known?collection_id=c&sets=nope", ``, ""), http.StatusBadRequest},
		{"a list without a database", call(k.list, "GET", "/images/known-sets", ``, ""), http.StatusServiceUnavailable},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got HTTP %d, want %d", c.name, c.got, c.want)
		}
	}
}

func TestKnownSetsAreOffUnlessSwitchedOn(t *testing.T) {
	t.Setenv("FORENSIC_IMAGE_KNOWN_SETS", "")
	if imageKnownSetsEnabled() {
		t.Error("known sets must be off by default")
	}
	t.Setenv("FORENSIC_IMAGE_KNOWN_SETS", "true")
	if !imageKnownSetsEnabled() {
		t.Error("FORENSIC_IMAGE_KNOWN_SETS=true must switch them on")
	}
}
