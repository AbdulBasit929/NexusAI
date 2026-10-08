package main

// GOVERNED SQL: WORDS FOR AN EVIDENCE FAMILY THAT THE CURATED LAYER DOES NOT LIST (RUN 16).
//
// The fresh-seed check found two nouns an analyst uses for a family that the layer does not know. "How many log
// entries were there at night?" named no family, so the lane declined it; "How many different account id are in the
// registered numbers?" named no family either, the shortlist offered the subscribers and the transactions (both have
// an account id), and the model read the transactions: 3 for 9, a confident answer about the wrong evidence.
//
// These words are the lane's own. The layer is shared with the existing path, and a word added there changes what
// that path answers; a word added here changes only which view the lane offers and holds the query to. Keys are the
// prefix of the family's fields ("access_log" for access_log.event_time).
var govSQLLaneSynonyms = map[string][]string{
	"access_log": {"log entry", "log entries", "log line", "log lines"},
	"subscriber": {"registered number", "registered numbers", "registered line", "registered lines"},
}
