package main

import (
	"os"
	"strings"
)

// A1.1. TEMPLATE RESULT TABLES READ LIKE ENGLISH.
//
// `humanizeField` upper-cases every word of three letters or fewer, which is
// right for "IP" and wrong nearly everywhere else. Headers recorded on the
// deployed build, 2026-09-29: "Call END TS", "ROW Hash", "Completed AT",
// "Sampled NON Empty", while real acronyms of four or more letters came out
// "Imei", "Imsi", "Cnic", "Msisdn".
//
// This is used for result-table column HEADERS only. `humanizeField` itself is
// unchanged, because it also names metrics, and `factPacketPlumbingMetric`
// recognises machinery labels ("Records Row Count") by their exact text.
// Changing it globally would let those labels reach the narrator.
//
// Headers only: keys and values are untouched. See reports/table-header-casing-20260929/.
const tableHeaderCasingEnv = "FORENSIC_TABLE_HEADER_CASING"

func tableHeaderCasingEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(tableHeaderCasingEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

// headerAcronyms are the abbreviations this evidence actually uses, written
// the way an investigator writes them.
var headerAcronyms = map[string]string{
	"id": "ID", "ip": "IP", "imei": "IMEI", "imsi": "IMSI", "msisdn": "MSISDN", "cnic": "CNIC",
	"iccid": "ICCID", "sms": "SMS", "mms": "MMS", "url": "URL", "http": "HTTP", "https": "HTTPS",
	"lac": "LAC", "tac": "TAC", "cgi": "CGI", "ecgi": "ECGI", "pdf": "PDF", "ocr": "OCR",
	"anpr": "ANPR", "cdr": "CDR", "ipdr": "IPDR", "utc": "UTC", "gps": "GPS", "nat": "NAT",
	"dns": "DNS", "tcp": "TCP", "udp": "UDP", "sim": "SIM", "mac": "MAC", "bts": "BTS",
	"rag": "RAG", "kb": "KB", "api": "API", "vpn": "VPN", "ssid": "SSID",
}

// headerExpansions spell out source-schema abbreviations that have one reading
// in these files ("CALL_DIALED_NUM", "call_end_ts", "CALL_END_DT_TM").
var headerExpansions = map[string]string{
	"num": "number", "ts": "timestamp", "dt": "date", "tm": "time", "ind": "indicator",
	"org": "originating", "qty": "quantity", "amt": "amount", "cnt": "count", "pct": "percent",
	"dur": "duration", "seq": "sequence", "src": "source", "dst": "destination",
}

// tableHeader turns a column key into a sentence-case header with real acronyms.
func tableHeader(key string) string {
	if !tableHeaderCasingEnabled() {
		return humanizeField(key)
	}
	words := strings.Fields(strings.NewReplacer("_", " ", "-", " ").Replace(strings.TrimSpace(key)))
	if len(words) == 0 {
		return ""
	}
	for index, word := range words {
		lowered := strings.ToLower(word)
		if acronym, ok := headerAcronyms[lowered]; ok {
			words[index] = acronym
			continue
		}
		if expanded, ok := headerExpansions[lowered]; ok {
			lowered = expanded
		}
		words[index] = lowered
	}
	return capitalizeFirst(strings.Join(words, " "))
}
