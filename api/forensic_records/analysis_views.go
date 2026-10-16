package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ANALYSIS VIEWS FOR THE WORKSPACE: THE CONNECTIONS AND PLACES OF A CASE (roadmap 7.8.3 rows 2 and 3).
//
//	GET /analysis/connections?collection_id=...   who is in contact with whom: the subscribers and their strongest counterparties
//	GET /analysis/places?collection_id=...        which cell sites the subscribers used, and when
//
// These are NOT model-written. They are two fixed, reviewed reads of the analyst views (migration 017), run through the same executor as the Ask engine: the read-only
// analyst role, the tenant and collection settings, the case clock, a statement timeout and a row cap. The numbers a chart draws are therefore the numbers a question
// about the same thing would have given, and the coverage guard of the Ask engine applies: a case whose records are not (all) in the typed tables the views read gets
// `available: false` and the reason, never an empty chart that looks like a finding.
//
// The interface draws the graph and the map and adds what it can derive from them (shared contacts, a path, the carrier of a number); this file only reads.
const (
	analysisPerSubject  = 40
	analysisMaxPerSub   = 200
	analysisMaxEdges    = 6000
	analysisMaxSubjects = 60
	analysisMaxSites    = 3000
	analysisMaxVisits   = 30000
	analysisContract    = "forensics.analysis-views/v1"
)

type analysisSubject struct {
	Number   string `json:"number"`
	Events   int64  `json:"events"`
	Contacts int64  `json:"contacts"`
	First    string `json:"first,omitempty"`
	Last     string `json:"last,omitempty"`
	IMSI     string `json:"imsi,omitempty"`
	IMEI     string `json:"imei,omitempty"`
}

type analysisEdge struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Events   int64  `json:"events"`
	Outgoing int64  `json:"outgoing"`
	Incoming int64  `json:"incoming"`
	Unknown  int64  `json:"unknown"`
	Duration int64  `json:"duration_s"`
	First    string `json:"first,omitempty"`
	Last     string `json:"last,omitempty"`
	Service  bool   `json:"is_service"`
	Hub      bool   `json:"is_hub"`
}

type analysisConnections struct {
	Contract    string            `json:"contract"`
	Available   bool              `json:"available"`
	Reason      string            `json:"reason,omitempty"`
	GeneratedAt time.Time         `json:"generated_at"`
	Collection  string            `json:"collection_id"`
	TimeZone    string            `json:"time_zone"`
	Subjects    []analysisSubject `json:"subjects"`
	Edges       []analysisEdge    `json:"edges"`
	Totals      map[string]int64  `json:"totals"`
	Truncated   bool              `json:"truncated"`
	PerSubject  int               `json:"per_subject"`
	Caveats     []string          `json:"caveats"`
}

type analysisSite struct {
	CellKey  string   `json:"cell_key"`
	Name     string   `json:"name,omitempty"`
	Lat      *float64 `json:"lat,omitempty"`
	Lon      *float64 `json:"lon,omitempty"`
	RadiusM  *int64   `json:"radius_m,omitempty"`
	Events   int64    `json:"events"`
	Visitors int64    `json:"visitors"`
}

type analysisVisit struct {
	Subject string `json:"subject"`
	CellKey string `json:"cell_key"`
	Hour    string `json:"hour"`
	Events  int64  `json:"events"`
}

type analysisPlaces struct {
	Contract    string           `json:"contract"`
	Available   bool             `json:"available"`
	Reason      string           `json:"reason,omitempty"`
	GeneratedAt time.Time        `json:"generated_at"`
	Collection  string           `json:"collection_id"`
	TimeZone    string           `json:"time_zone"`
	Sites       []analysisSite   `json:"sites"`
	Visits      []analysisVisit  `json:"visits"`
	Totals      map[string]int64 `json:"totals"`
	Truncated   bool             `json:"truncated"`
	Caveats     []string         `json:"caveats"`
}

func analysisStr(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// analysisTime writes a time the way a browser can read it: RFC 3339 with the case clock's own offset (the executor shows times as "2026-02-02 04:00:00 PKT", a zone
// abbreviation that a browser cannot parse and that two zones can share).
func analysisTime(v any) string {
	if v == nil {
		return ""
	}
	if t, ok := v.(time.Time); ok {
		return t.Format(time.RFC3339)
	}
	text := strings.TrimSpace(fmt.Sprint(v))
	if len(text) >= 19 {
		_, loc := govSQLTimeZone()
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", text[:19], loc); err == nil {
			return t.Format(time.RFC3339)
		}
	}
	return text
}

func analysisInt(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int32:
		return int64(x)
	case int:
		return int64(x)
	case float64:
		return int64(x)
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	}
	return 0
}

func analysisFloat(v any) *float64 {
	switch x := v.(type) {
	case float64:
		return &x
	case float32:
		f := float64(x)
		return &f
	case string:
		if f, err := strconv.ParseFloat(x, 64); err == nil {
			return &f
		}
	}
	return nil
}

func analysisBool(v any) bool {
	b, _ := v.(bool)
	return b
}

// analysisGuard resolves the scope the way every other handler does and applies the coverage guard. It returns the scope and, when the views cannot speak for the case,
// the reason (the response then says so and carries no data).
func analysisGuard(w http.ResponseWriter, r *http.Request, db *pgxpool.Pool, views ...string) (forensicScope, string, bool) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return forensicScope{}, "", false
	}
	if db == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("FORENSIC_DATABASE_URL is not configured"))
		return forensicScope{}, "", false
	}
	scope, err := bindForensicScope(r, r.URL.Query().Get("tenant_id"), strings.TrimSpace(r.URL.Query().Get("collection_id")), "", "")
	if err != nil {
		writeScopeError(w, err)
		return forensicScope{}, "", false
	}
	if scope.CollectionID == "" {
		writeError(w, http.StatusBadRequest, errors.New("collection_id is required"))
		return forensicScope{}, "", false
	}
	coverage, err := askCoverageFor(r.Context(), db, scope.TenantID, scope.CollectionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, errors.New("could not read the case's coverage of the typed tables"))
		return forensicScope{}, "", false
	}
	return scope, askCoverageGap(coverage, views), true
}

func connectionsHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scope, gap, ok := analysisGuard(w, r, db, "v_contact_pairs")
		if !ok {
			return
		}
		zone, _ := govSQLTimeZone()
		out := analysisConnections{Contract: analysisContract, GeneratedAt: time.Now().UTC(), Collection: scope.CollectionID, TimeZone: zone, Subjects: []analysisSubject{}, Edges: []analysisEdge{}, Totals: map[string]int64{},
			Caveats: []string{"A link is a record of contact between two numbers, not a finding about the people.", "Shared-contact links are inferred from the links shown; they do not say the two numbers were in touch."}}
		if gap != "" {
			out.Reason = "the analyst views cannot speak for this case: " + gap
			writeJSON(w, http.StatusOK, out)
			return
		}
		per := analysisPerSubject
		if p := queryInt(r, "per_subject", analysisPerSubject); p > 0 && p <= analysisMaxPerSub {
			per = p
		}
		out.PerSubject = per
		limits := askDefaultLimits()
		run := func(sql string, max int) (*govSQLResult, bool) {
			result, err := askRun(r.Context(), db, scope.TenantID, scope.CollectionID, sql, nil, limits, max, false)
			if err != nil {
				writeError(w, http.StatusInternalServerError, errors.New("the connections could not be read"))
				return nil, false
			}
			return result, true
		}
		subs, ok := run(fmt.Sprintf("SELECT src, sum(events)::bigint, count(*)::bigint, min(first_seen), max(last_seen) FROM v_contact_pairs GROUP BY src ORDER BY 2 DESC, 1 LIMIT %d", analysisMaxSubjects+1), analysisMaxSubjects)
		if !ok {
			return
		}
		for _, row := range subs.Rows {
			out.Subjects = append(out.Subjects, analysisSubject{Number: analysisStr(row[0]), Events: analysisInt(row[1]), Contacts: analysisInt(row[2]), First: analysisTime(row[3]), Last: analysisTime(row[4])})
		}
		out.Truncated = subs.Truncated
		edges, ok := run(fmt.Sprintf(`SELECT src, dst, events, outgoing, incoming, unknown, coalesce(total_duration_s, 0), first_seen, last_seen, is_service, is_hub FROM (
SELECT p.*, row_number() OVER (PARTITION BY src ORDER BY events DESC, dst) AS rn FROM v_contact_pairs p WHERE dst IS NOT NULL) t WHERE rn <= %d ORDER BY src, rn LIMIT %d`, per, analysisMaxEdges+1), analysisMaxEdges)
		if !ok {
			return
		}
		for _, row := range edges.Rows {
			out.Edges = append(out.Edges, analysisEdge{Source: analysisStr(row[0]), Target: analysisStr(row[1]), Events: analysisInt(row[2]), Outgoing: analysisInt(row[3]), Incoming: analysisInt(row[4]),
				Unknown: analysisInt(row[5]), Duration: analysisInt(row[6]), First: analysisTime(row[7]), Last: analysisTime(row[8]), Service: analysisBool(row[9]), Hub: analysisBool(row[10])})
		}
		out.Truncated = out.Truncated || edges.Truncated
		// the handset and SIM each subscriber was seen with (the SIM's network code gives the carrier; a number's own range is only a guess)
		dev, ok := run("SELECT src, imsi, imei, events FROM v_device_pairings WHERE imsi IS NOT NULL OR imei IS NOT NULL ORDER BY src, events DESC LIMIT 2000", 2000)
		if !ok {
			return
		}
		bySubject := map[string]int{}
		for i, s := range out.Subjects {
			bySubject[s.Number] = i
		}
		for _, row := range dev.Rows {
			if i, found := bySubject[analysisStr(row[0])]; found && out.Subjects[i].IMSI == "" {
				out.Subjects[i].IMSI, out.Subjects[i].IMEI = analysisStr(row[1]), analysisStr(row[2])
			}
		}
		totals, ok := run("SELECT count(*)::bigint, count(DISTINCT src)::bigint, count(DISTINCT dst)::bigint, coalesce(sum(events), 0)::bigint, count(*) FILTER (WHERE is_service)::bigint, count(DISTINCT dst) FILTER (WHERE is_hub)::bigint FROM v_contact_pairs", 1)
		if !ok {
			return
		}
		if len(totals.Rows) == 1 {
			t := totals.Rows[0]
			out.Totals = map[string]int64{"pairs": analysisInt(t[0]), "subjects": analysisInt(t[1]), "counterparties": analysisInt(t[2]), "events": analysisInt(t[3]), "service_pairs": analysisInt(t[4]), "hubs": analysisInt(t[5])}
		}
		out.Available = true
		writeJSON(w, http.StatusOK, out)
	}
}

func placesHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scope, gap, ok := analysisGuard(w, r, db, "v_cell_visits")
		if !ok {
			return
		}
		zone, _ := govSQLTimeZone()
		out := analysisPlaces{Contract: analysisContract, GeneratedAt: time.Now().UTC(), Collection: scope.CollectionID, TimeZone: zone, Sites: []analysisSite{}, Visits: []analysisVisit{}, Totals: map[string]int64{},
			Caveats: []string{"A cell site is a coverage area with a stated radius, not the position of a person.", "A visit is a record on the cell, by hour; it does not say where the person was inside its coverage."}}
		if gap != "" {
			out.Reason = "the analyst views cannot speak for this case: " + gap
			writeJSON(w, http.StatusOK, out)
			return
		}
		limits := askDefaultLimits()
		run := func(sql string, max int) (*govSQLResult, bool) {
			result, err := askRun(r.Context(), db, scope.TenantID, scope.CollectionID, sql, nil, limits, max, false)
			if err != nil {
				writeError(w, http.StatusInternalServerError, errors.New("the places could not be read"))
				return nil, false
			}
			return result, true
		}
		sites, ok := run(fmt.Sprintf("SELECT cell_key, max(site_name), max(lat), max(lon), max(radius_m)::bigint, sum(events)::bigint, count(DISTINCT src)::bigint FROM v_cell_visits GROUP BY cell_key ORDER BY 6 DESC, 1 LIMIT %d", analysisMaxSites+1), analysisMaxSites)
		if !ok {
			return
		}
		for _, row := range sites.Rows {
			out.Sites = append(out.Sites, analysisSite{CellKey: analysisStr(row[0]), Name: analysisStr(row[1]), Lat: analysisFloat(row[2]), Lon: analysisFloat(row[3]),
				RadiusM: func() *int64 { n := analysisInt(row[4]); return &n }(), Events: analysisInt(row[5]), Visitors: analysisInt(row[6])})
		}
		out.Truncated = sites.Truncated
		visits, ok := run(fmt.Sprintf("SELECT src, cell_key, hour_utc, events FROM v_cell_visits ORDER BY src, hour_utc LIMIT %d", analysisMaxVisits+1), analysisMaxVisits)
		if !ok {
			return
		}
		for _, row := range visits.Rows {
			out.Visits = append(out.Visits, analysisVisit{Subject: analysisStr(row[0]), CellKey: analysisStr(row[1]), Hour: analysisTime(row[2]), Events: analysisInt(row[3])})
		}
		out.Truncated = out.Truncated || visits.Truncated
		positioned := int64(0)
		for _, s := range out.Sites {
			if s.Lat != nil && s.Lon != nil {
				positioned++
			}
		}
		out.Totals = map[string]int64{"sites": int64(len(out.Sites)), "sites_with_position": positioned, "visits": int64(len(out.Visits))}
		out.Available = true
		writeJSON(w, http.StatusOK, out)
	}
}

