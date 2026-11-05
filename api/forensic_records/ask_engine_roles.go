package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ASK ENGINE V2: WHERE THE QUESTION PUTS ITS IDENTIFIER (the verified bank run of 2026-11-02, VB-BANK-050, 091, 112 and 123).
//
// "Which accounts or numbers sent money to the account X?" was read as a question about OUTGOING events, because the verb is "sent". The direction a verb names is the direction of
// whoever does it; the identifier of the question is the one that RECEIVES, so the events to read are X's incoming ones ("credit" for money) and the other side of each is who sent.
// The check demanded 'outgoing' or 'debit', the one repair the engine had for the opposite fault (DIRECTION_ROLE) demanded 'credit', the two contradicted each other, and the model's
// right query was refused three times. The direction of a question is relative to the identifier it names, and the identifier's place in the sentence says which end of the event it is:
//
//   - after "to" (what is sent, called, paid or messaged TO it): the receiving end, so its events are incoming;
//   - after "from" (what comes FROM it): the sending end, so its events are outgoing;
//   - before the verb ("did X send ...", "X called ..."): the doer, so the verbs decide, as they always did.
//
// This is read from the question's own words and the positions of its identifiers; it needs no list of questions. It applies only when every identifier of the question is in one of
// the first two places and they agree: a question that names the doer too ("calls from A to B", "A sent money to B") is a pair and keeps the verbs' reading.

// askRoleToken is a word of the question or the marker that holds an identifier back (askExtractFrameWith writes \x00N\x00 for the Nth identifier it set aside).
var askRoleToken = regexp.MustCompile(`[a-z]+|\x00\d+\x00`)

// askRoleVerbs are the words that give a question a direction of its own: what is sent and what is received. "calls" and "messages" count, as they do in the cues of the frame.
var askRoleVerbs = map[string]bool{
	"send": true, "sends": true, "sent": true, "sending": true, "call": true, "calls": true, "called": true, "calling": true, "message": true, "messages": true, "messaged": true,
	"messaging": true, "text": true, "texts": true, "texted": true, "texting": true, "pay": true, "pays": true, "paid": true, "paying": true, "transfer": true, "transfers": true,
	"transferred": true, "transferring": true, "wire": true, "wired": true, "remit": true, "remitted": true, "deposit": true, "deposits": true, "deposited": true, "dial": true, "dials": true,
	"dialled": true, "dialed": true, "dialling": true, "dialing": true, "make": true, "makes": true, "made": true, "making": true,
	"receive": true, "receives": true, "received": true, "receiving": true, "get": true, "gets": true, "got": true, "getting": true,
}

// askRoleFillers are the words that may stand between "to" or "from" and the identifier: the article and the name of the kind of identifier ("to the account X", "from number X").
var askRoleFillers = map[string]bool{
	"the": true, "a": true, "an": true, "this": true, "that": true, "its": true, "their": true, "his": true, "her": true, "same": true, "account": true, "accounts": true, "acct": true,
	"number": true, "numbers": true, "wallet": true, "iban": true, "msisdn": true, "mobile": true, "phone": true, "line": true, "bank": true, "sim": true, "subscriber": true,
}

// askIdentifierRole is the place of one identifier in the question: "subject" (before the first verb), "to", "from", or "" (anywhere else).
type askIdentifierRole struct{ Identifier, Role string }

// askIdentifierRoles reads the place of every identifier of the frame in the question, and whether the question has a verb of sending or receiving at all. lower is the question in lower
// case with its identifiers held back as markers, and held is what each marker stands for.
func askIdentifierRoles(lower string, held []string, f askFrame) ([]askIdentifierRole, bool) {
	identifier := map[string]bool{}
	for _, id := range f.Numbers {
		identifier[id] = true
	}
	for _, id := range f.Accounts {
		identifier[id] = true
	}
	tokens := askRoleToken.FindAllString(lower, -1)
	firstVerb := -1
	for i, t := range tokens {
		if askRoleVerbs[t] {
			firstVerb = i
			break
		}
	}
	var roles []askIdentifierRole
	for i, t := range tokens {
		if len(t) < 3 || t[0] != 0 {
			continue
		}
		index, err := strconv.Atoi(t[1 : len(t)-1])
		if err != nil || index < 0 || index >= len(held) || !identifier[held[index]] {
			continue
		}
		role := ""
		j := i - 1
		for j >= 0 && i-j <= 5 && askRoleFillers[tokens[j]] {
			j--
		}
		if j >= 0 {
			switch tokens[j] {
			case "to", "toward", "towards":
				role = "to"
			case "from":
				role = "from"
			}
		}
		if role == "" && firstVerb >= 0 && i < firstVerb {
			role = "subject"
		}
		roles = append(roles, askIdentifierRole{Identifier: held[index], Role: role})
	}
	return roles, firstVerb >= 0
}

// askRoleDirection is the direction of the question relative to its identifiers, when their places decide it: "incoming" when every identifier is the receiving end (after "to"),
// "outgoing" when every one is the sending end (after "from"), and "" when a doer is named, an identifier stands elsewhere, the places disagree, or the question has no verb of
// sending or receiving at all. The identifiers whose places decided it are returned with it.
func askRoleDirection(roles []askIdentifierRole, hasVerb bool) (string, []string) {
	if !hasVerb || len(roles) == 0 {
		return "", nil
	}
	to, from := 0, 0
	var ids []string
	for _, r := range roles {
		switch r.Role {
		case "to":
			to++
		case "from":
			from++
		default:
			return "", nil
		}
		ids = append(ids, r.Identifier)
	}
	switch {
	case to > 0 && from == 0:
		return "incoming", ids
	case from > 0 && to == 0:
		return "outgoing", ids
	}
	return "", nil
}

// askOtherPartyColumns are the columns that hold the OTHER end of an event: the counterparty of an account, the dialled or calling number of a subscriber.
var askOtherPartyColumns = map[string]bool{"dst": true, "counterparty": true, "counterparty_account": true, "counterparty_msisdn": true, "received_from": true, "sent_to": true}

// askComparesOtherParty is whether the query compares the identifiers only with the columns of the other end of an event. Such a query reads the same events from the senders' side: the
// rows are theirs, and the direction a row carries is the opposite of the one the identifier has ("account = X" credits are "counterparty_account = X" debits).
func askComparesOtherParty(sql string, ids []string) bool {
	seen := false
	for _, id := range ids {
		columns := askComparedColumns(sql, id)
		for column := range columns {
			if !askOtherPartyColumns[column] {
				return false
			}
			seen = true
		}
	}
	return seen
}

// askRoleDirectionMessage is what the model is told when a query reads the identifier's events in the direction that the place of the identifier excludes, and the reason an analyst is
// shown when the question is not answered. direction is the identifier's own direction; flipped is whether the query reads the other end of the events.
func askRoleDirectionMessage(direction string, ids []string, flipped bool) (message, reason string) {
	who := strings.Join(ids, " and ")
	switch {
	case direction == "incoming" && !flipped:
		return fmt.Sprintf("The question asks what was sent TO %[1]s, so %[1]s is the receiving end: read the events of %[1]s that came in (direction = 'incoming' for calls and messages, direction = 'credit' for money); the other side of each (dst, counterparty_account) is who sent. Its outgoing events or debits are what it sent, which is another question.", who),
			"the question asks what was sent to the identifier and the query reads what the identifier sent"
	case direction == "incoming":
		return fmt.Sprintf("You compare %[1]s with the column of the other end of an event, so the rows you read are the senders' own: their direction is 'outgoing' ('debit' for money), not 'incoming' ('credit'). The question asks what was sent TO %[1]s.", who),
			"the question asks what was sent to the identifier and the query reads the senders' rows in the wrong direction"
	case !flipped:
		return fmt.Sprintf("The question asks what came FROM %[1]s, so %[1]s is the sending end: read the events of %[1]s that went out (direction = 'outgoing' for calls and messages, direction = 'debit' for money); the other side of each is who received. Its incoming events or credits are what it received, which is another question.", who),
			"the question asks what the identifier sent and the query reads what it received"
	}
	return fmt.Sprintf("You compare %[1]s with the column of the other end of an event, so the rows you read are the receivers' own: their direction is 'incoming' ('credit' for money), not 'outgoing' ('debit'). The question asks what came FROM %[1]s.", who),
		"the question asks what the identifier sent and the query reads the receivers' rows in the wrong direction"
}
