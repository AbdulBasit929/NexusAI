package main

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	pg "github.com/pganalyze/pg_query_go/v6"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ASK ENGINE V2: THE VALUES A COLUMN REALLY HOLDS IN THIS CASE (the verified bank run of 2026-11-02, VB-INVE-003).
//
// "How many tabular files are in this case?" was answered 4 (WHERE modality = 'document') where the case holds 3 tabular files. The description of v_evidence_inventory.modality in the
// database lists five values (image, audio, video, document, structured_records); the files of a case also carry text and tabular (and structured_data, database, container, ...), so the
// model, which reads only the description, took the nearest value it knew. A word of the question that IS one of the column's values must not be answered with another value of it.
//
// The values are the case's own: they are read with the coverage (one small query, cached for the same 30 seconds), not typed into the code, so a new kind of file needs no change here.
// They are used twice. The model is told, in the hints of the question, which of them the question names (so the first draft is right); and the validated query is held to it
// (VALUE_DOMAIN), so a draft that compares the column with a different value is repaired before it runs.

// askDomainColumns are the columns whose values the case's data extends beyond the catalogue's description. A column joins this list when a bank run shows the description lagging the data.
var askDomainColumns = []struct{ View, Column string }{
	{"v_evidence_inventory", "modality"},
	{"v_evidence_inventory", "detected_type"},
}

const askDomainsQuery = `SELECT coalesce(array_agg(DISTINCT modality::text) FILTER (WHERE modality IS NOT NULL), '{}'::text[]),
  coalesce(array_agg(DISTINCT detected_type::text) FILTER (WHERE detected_type IS NOT NULL), '{}'::text[])
FROM forensic.evidence_items WHERE tenant_id = $1 AND collection_id = $2`

// askDomainMaxValues bounds the values kept for a column, and askDomainMaxLength the length of each (the values go into a prompt).
const (
	askDomainMaxValues = 40
	askDomainMaxLength = 40
)

// askReadDomains fills Domains of c, in the transaction of the coverage read, with the values that the case's evidence files carry in the columns of askDomainColumns.
func askReadDomains(ctx context.Context, tx pgx.Tx, tenantID, collectionID string, c *askCoverage) error {
	var modalities, types []string
	if err := tx.QueryRow(ctx, askDomainsQuery, tenantID, collectionID).Scan(&modalities, &types); err != nil {
		return err
	}
	c.Domains = map[string][]string{}
	for key, values := range map[string][]string{"v_evidence_inventory.modality": modalities, "v_evidence_inventory.detected_type": types} {
		kept := make([]string, 0, len(values))
		for _, v := range values {
			if v = strings.ToLower(strings.TrimSpace(v)); v != "" && len(v) <= askDomainMaxLength && len(kept) < askDomainMaxValues {
				kept = append(kept, v)
			}
		}
		sort.Strings(kept)
		c.Domains[key] = kept
	}
	return nil
}

// askDomainsFor are the domains of the case for the views a question reads (the columns of askDomainColumns), as "view.column" to the values.
func askDomainsFor(c askCoverage, views []string) map[string][]string {
	out := map[string][]string{}
	for _, d := range askDomainColumns {
		key := d.View + "." + d.Column
		if values := c.Domains[key]; len(values) > 0 && askContains(views, d.View) {
			out[key] = values
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// askDomainNamed are the values of a domain that the question names as a word (a plural, or an underscore written as a space, counts), in the domain's order.
func askDomainNamed(question string, values []string) []string {
	q := strings.ToLower(question)
	var named []string
	for _, value := range values {
		words := strings.ReplaceAll(value, "_", " ")
		if len(words) > 3 {
			words = strings.TrimSuffix(words, "s")
		}
		if regexp.MustCompile(`\b` + regexp.QuoteMeta(words) + `(?:s|es)?\b`).MatchString(q) {
			named = append(named, value)
		}
	}
	return named
}

// askDomainKey splits "view.column".
func askDomainKey(key string) (view, column string) {
	view, column, _ = strings.Cut(key, ".")
	return view, column
}

// askDomainKeys are the keys of the frame's domains in a fixed order.
func askDomainKeys(f askFrame) []string {
	keys := make([]string, 0, len(f.Domains))
	for k := range f.Domains {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// askDomainHints tell the model, for each column whose value the question names, the values the case holds and the one to use.
func askDomainHints(f askFrame) []string {
	var hints []string
	for _, key := range askDomainKeys(f) {
		_, column := askDomainKey(key)
		if named := askDomainNamed(f.Question, f.Domains[key]); len(named) > 0 {
			hints = append(hints, fmt.Sprintf("The column %s holds these values in this case: %s. The question names %s: compare %s with that value, not with another one.", column, strings.Join(f.Domains[key], ", "), strings.Join(named, ", "), column))
		}
	}
	return hints
}

// askComparedWith are the constants the query compares a column with (column = 'x', column IN ('x', 'y'), column = ANY (ARRAY['x'])), in lower case.
func askComparedWith(v *askValidated, column string) map[string]bool {
	used := map[string]bool{}
	if v == nil || v.Tree == nil {
		return used
	}
	askWalkTree(v.Tree.ProtoReflect(), func(m protoreflect.Message) bool {
		if n, ok := m.Interface().(*pg.Node); ok {
			if name, values, ok := askComparison(n); ok && name == column {
				for _, value := range values {
					used[strings.ToLower(value)] = true
				}
			}
		}
		return true
	})
	return used
}

// askDomainObligations holds the query to the values the case holds: when the question names a value of a column the query filters on, the query must use a value the question names. A column the
// query does not filter on is not concerned (the question may be answered through another column, as "text files" by their extension), and a word that is no value of the case is not either.
func askDomainObligations(f askFrame, v *askValidated, add func(kind, message, reason string)) {
	for _, key := range askDomainKeys(f) {
		view, column := askDomainKey(key)
		if !askUsesView(v, view) {
			continue
		}
		used := askComparedWith(v, column)
		named := askDomainNamed(f.Question, f.Domains[key])
		if len(used) == 0 || len(named) == 0 {
			continue
		}
		agrees, wrong := false, []string{}
		for _, n := range named {
			agrees = agrees || used[n]
		}
		for u := range used {
			wrong = append(wrong, "'"+u+"'")
		}
		sort.Strings(wrong)
		if agrees {
			continue
		}
		quoted := make([]string, len(named))
		for i, n := range named {
			quoted[i] = "'" + n + "'"
		}
		kind := "a value"
		if len(named) > 1 {
			kind = "values"
		}
		add("VALUE_DOMAIN", fmt.Sprintf("The question names %[1]s, which is %[2]s of %[3]s in this case (the values present are: %[4]s), but the query compares %[3]s with %[5]s. Compare %[3]s with %[1]s, as the question says, not with another value.",
			strings.Join(quoted, " and "), kind, column, strings.Join(f.Domains[key], ", "), strings.Join(wrong, " or ")),
			"the question names a value of a column and the query filters the column on another value")
	}
}
