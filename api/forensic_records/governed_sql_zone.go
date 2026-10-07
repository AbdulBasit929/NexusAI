package main

import (
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	pg "github.com/pganalyze/pg_query_go/v6"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// GOVERNED SQL: THE CASE CLOCK.
//
// Evidence times are stored as UTC instants: the ingest reads a time without an offset in
// the source zone and converts a time with an offset by itself, and records which. A
// question is asked in the investigator's own words, "at night", "in June", "on the 14th",
// and means the clock of the place the case is about, not the clock of the database.
// Evaluated on the stored UTC instant, "night" counted a different six hours of the day
// (CDR: 05:00 to 10:59 in Pakistan) while the answer said "as recorded at the source".
//
// PostgreSQL evaluates the hour, day and month of a timestamptz, and the date a text time is
// cast to, in the SESSION time zone. The lane therefore sets the session zone of its
// read-only transaction to ONE case time zone, and every condition and every time shown
// means that clock, for every evidence family alike. Nothing stored is changed, and UTC
// remains the canonical instant.
//
// The zone is FORENSIC_ANALYSIS_TIMEZONE, default Asia/Karachi (the typed lane already
// buckets time in Asia/Karachi). A name that is not a known IANA zone is not guessed at:
// the default applies and the server log says so once.
const (
	govSQLTimeZoneEnv     = "FORENSIC_ANALYSIS_TIMEZONE"
	govSQLDefaultTimeZone = "Asia/Karachi"
)

var govSQLZoneWarnOnce sync.Once

// govSQLTimeZone returns the case time zone: the name PostgreSQL and the analyst read, and
// the location the lane formats times in.
func govSQLTimeZone() (string, *time.Location) {
	name := strings.TrimSpace(os.Getenv(govSQLTimeZoneEnv)) //nolint:forbidigo // forensic records product API reads its settings from the environment by design (docker compose); not LocalAI server config
	if name == "" {
		name = govSQLDefaultTimeZone
	}
	loc, err := time.LoadLocation(name)
	// "Local" is Go's word for the process's own zone, which PostgreSQL does not know.
	if err != nil || strings.EqualFold(name, "Local") {
		govSQLZoneWarnOnce.Do(func() {
			slog.Warn("governed SQL: "+govSQLTimeZoneEnv+" is not a known IANA time zone; using the default", "value", name, "default", govSQLDefaultTimeZone)
		})
		name = govSQLDefaultTimeZone
		loc, err = time.LoadLocation(name)
		if err != nil {
			// The zone database is compiled in (time/tzdata), so this cannot happen. If it ever does,
			// UTC is stated as UTC rather than passed off as the case clock.
			return "UTC", time.UTC
		}
	}
	return name, loc
}

// govSQLZoneOffset writes a zone's offset as an analyst reads it: "UTC+05:00".
func govSQLZoneOffset(loc *time.Location, at time.Time) string {
	return "UTC" + at.In(loc).Format("-07:00")
}

// govSQLCellValue turns one database value into what the lane carries, by the column's
// TYPE. A timestamptz is shown on the case clock with its zone's abbreviation. A date has
// no time and no zone: the driver hands it back as midnight UTC, and shifting that into a
// zone would move it to another calendar day's morning, so it is shown as the date. A
// timestamp without a zone is already a wall-clock reading (a conversion the query made)
// and is shown as written. Every other value is what it always was.
func govSQLCellValue(value any, typeOID uint32, loc *time.Location) any {
	t, isTime := value.(time.Time)
	if !isTime {
		return normalizeDBValue(value)
	}
	switch typeOID {
	case pgtype.TimestamptzOID:
		return t.In(loc).Format("2006-01-02 15:04:05 MST")
	case pgtype.DateOID:
		return t.Format("2006-01-02")
	case pgtype.TimestampOID:
		return t.Format("2006-01-02 15:04:05")
	}
	return normalizeDBValue(value)
}

// govSQLRecordedTime keeps a time that is not a recording out of every time condition.
// The canonical record time is coalesce(source time, ingest time): a record with no time of
// its own (a plate read derived from a video frame, a row whose time cell was blank) holds
// the moment it was ingested, and counting that as "when it happened" answers a question
// nobody asked (3 of the 165 ANPR "night" sightings were such rows). Both ways the platform
// marks them, and the structural signature of the fallback (the time equals the ingest
// time), make the value NULL here, so a time condition does not match it and MIN and MAX
// ignore it, while a count that names no time still counts the row.
func govSQLRecordedTime(expr string) string {
	return "(CASE WHEN jsonb_exists(r.metadata, 'derived_from_media') OR jsonb_exists(r.metadata, 'timestamp_fallback') " +
		"OR r.timestamp IS NOT DISTINCT FROM r.ingested_at THEN NULL ELSE " + expr + " END)"
}

// govSQLForeignZoneConversions lists the time-zone conversions a query applies (AT TIME
// ZONE, timezone(...)) to any zone but the case's. The views already read time on the case
// clock, so a conversion to another zone counts a different window than the one the
// question means and the answer states, which is the defect this file exists to remove.
// A redundant conversion to the case zone itself is harmless and allowed.
func govSQLForeignZoneConversions(tree *pg.ParseResult, zone string) []string {
	if tree == nil {
		return nil
	}
	var found []string
	for _, stmt := range tree.Stmts {
		govSQLWalkTree(stmt.ProtoReflect(), func(msg protoreflect.Message) bool {
			call, ok := msg.Interface().(*pg.FuncCall)
			if !ok || len(call.Funcname) == 0 {
				return true
			}
			last := call.Funcname[len(call.Funcname)-1].GetString_()
			if last == nil || !strings.EqualFold(last.Sval, "timezone") {
				return true
			}
			target := "an expression"
			if len(call.Args) > 0 {
				if constant := call.Args[0].GetAConst(); constant != nil && constant.GetSval() != nil {
					target = constant.GetSval().Sval
				}
			}
			if !strings.EqualFold(target, zone) {
				found = append(found, target)
			}
			return true
		})
	}
	return found
}
