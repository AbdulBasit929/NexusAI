package main

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// GOVERNED SQL: A THRESHOLD WRITTEN IN A UNIT (RUN 15).
//
// "How many calls lasted more than 15 minutes?" abstained twice (pre-registration, "Run 15 plan", class L): the
// verifier demanded the number 15 in the query, and the duration column holds seconds, so the right query says
// 900. The check that stops a query from dropping a stated threshold must accept the threshold in the unit the
// column is kept in. It does, for durations (seconds, minutes, hours, days), for sizes (bytes up to terabytes,
// in both the 1,000 and the 1,024 convention) and for percentages (50 percent is 50 or 0.5), and for an interval
// written as a literal ('15 minutes').

type govSQLUnit struct {
	family string  // "time", "bytes" or "percent"
	size   float64 // seconds in a time unit
	power  int     // thousands (or 1,024s) in a size unit
}

var govSQLUnitWords = func() map[string]govSQLUnit {
	out := map[string]govSQLUnit{}
	for _, w := range []string{"second", "seconds", "sec", "secs"} {
		out[w] = govSQLUnit{family: "time", size: 1}
	}
	for _, w := range []string{"minute", "minutes", "min", "mins"} {
		out[w] = govSQLUnit{family: "time", size: 60}
	}
	for _, w := range []string{"hour", "hours", "hr", "hrs"} {
		out[w] = govSQLUnit{family: "time", size: 3600}
	}
	for _, w := range []string{"day", "days"} {
		out[w] = govSQLUnit{family: "time", size: 86400}
	}
	for i, words := range [][]string{
		{"byte", "bytes"}, {"kb", "kilobyte", "kilobytes"}, {"mb", "megabyte", "megabytes"}, {"gb", "gigabyte", "gigabytes"}, {"tb", "terabyte", "terabytes"},
	} {
		for _, w := range words {
			out[w] = govSQLUnit{family: "bytes", power: i}
		}
	}
	for _, w := range []string{"percent", "percentage"} {
		out[w] = govSQLUnit{family: "percent"}
	}
	return out
}()

// govSQLUnitOf is the unit word that ends a stated quantity: "more than 15 minutes". A quantity with no unit
// word ("more than 15") has none.
func govSQLUnitOf(phrase string) (govSQLUnit, bool) {
	fields := strings.Fields(strings.ToLower(phrase))
	if len(fields) < 2 {
		return govSQLUnit{}, false
	}
	unit, ok := govSQLUnitWords[strings.Trim(fields[len(fields)-1], ".,;:?!")]
	return unit, ok
}

// govSQLUnitValues is every constant a column kept in some unit would compare with for n of this unit.
func govSQLUnitValues(n float64, u govSQLUnit) []float64 {
	switch u.family {
	case "time":
		seconds := n * u.size
		return []float64{n, seconds, seconds / 60, seconds / 3600, seconds / 86400, seconds * 1000}
	case "bytes":
		out := []float64{n}
		for j := 0; j <= 4; j++ {
			out = append(out, n*math.Pow(1000, float64(u.power-j)), n*math.Pow(1024, float64(u.power-j)))
		}
		return out
	case "percent":
		return []float64{n, n / 100}
	}
	return []float64{n}
}

func govSQLSameNumber(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
}

var govSQLRxIntervalText = regexp.MustCompile(`(?i)^\s*(\d+(?:\.\d+)?)\s*(seconds?|secs?|minutes?|mins?|hours?|hrs?|days?)\s*$`)
var govSQLRxHMS = regexp.MustCompile(`^\s*(\d+):(\d{2}):(\d{2})\s*$`)

// govSQLIntervalSeconds reads an interval written as a string constant: '15 minutes', '00:15:00'.
func govSQLIntervalSeconds(text string) (float64, bool) {
	if m := govSQLRxIntervalText.FindStringSubmatch(text); m != nil {
		n, err := strconv.ParseFloat(m[1], 64)
		unit, ok := govSQLUnitWords[strings.ToLower(m[2])]
		if err != nil || !ok {
			return 0, false
		}
		return n * unit.size, true
	}
	if m := govSQLRxHMS.FindStringSubmatch(text); m != nil {
		h, _ := strconv.Atoi(m[1])
		mi, _ := strconv.Atoi(m[2])
		s, _ := strconv.Atoi(m[3])
		return float64(h*3600 + mi*60 + s), true
	}
	return 0, false
}

// govSQLQuantityInUnit is true when the query's constants hold a stated quantity in its own unit or in another
// unit of the same kind: 15 minutes as 900, as 0.25 or as '15 minutes'.
func govSQLQuantityInUnit(facts govSQLQuestionFacts, consts []govSQLConst) bool {
	unit, ok := govSQLUnitOf(facts.Magnitude)
	if !ok {
		return false
	}
	for _, text := range facts.Numbers {
		n, err := strconv.ParseFloat(text, 64)
		if err != nil {
			continue
		}
		for _, c := range consts {
			if c.Numeric {
				value, err := strconv.ParseFloat(c.Text, 64)
				if err != nil {
					continue
				}
				for _, want := range govSQLUnitValues(n, unit) {
					if govSQLSameNumber(value, want) {
						return true
					}
				}
				continue
			}
			if unit.family == "time" {
				if seconds, ok := govSQLIntervalSeconds(c.Text); ok && govSQLSameNumber(seconds, n*unit.size) {
					return true
				}
			}
		}
	}
	return false
}

func govSQLTrimNumber(n float64) string {
	return strconv.FormatFloat(n, 'f', -1, 64)
}

// govSQLUnitAdvice tells the model what a stated quantity is in the unit a column is kept in.
func govSQLUnitAdvice(facts govSQLQuestionFacts) string {
	unit, ok := govSQLUnitOf(facts.Magnitude)
	if !ok || len(facts.Numbers) == 0 {
		return ""
	}
	n, err := strconv.ParseFloat(facts.Numbers[0], 64)
	if err != nil {
		return ""
	}
	switch unit.family {
	case "time":
		if unit.size == 1 {
			return ""
		}
		return fmt.Sprintf("A duration column holds seconds: %s is %s seconds.", strings.TrimSpace(facts.Numbers[0]+" "+govSQLLastWord(facts.Magnitude)), govSQLTrimNumber(n*unit.size))
	case "bytes":
		if unit.power == 0 {
			return ""
		}
		return fmt.Sprintf("A size column holds bytes: %s is %s bytes.", strings.TrimSpace(facts.Numbers[0]+" "+govSQLLastWord(facts.Magnitude)), govSQLTrimNumber(n*math.Pow(1000, float64(unit.power))))
	case "percent":
		return "A share written as a percentage is the number itself, or a fraction of 1 when the value is a ratio."
	}
	return ""
}

func govSQLLastWord(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[len(fields)-1], ".,;:?!")
}
