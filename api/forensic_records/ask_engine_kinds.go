package main

import (
	"fmt"
	"sort"
	"strings"
)

// ASK ENGINE V2: WHICH VIEW CAN CARRY A KIND OF EVENT (the verified bank run of 2026-11-02, VB-MESS-009 and VB-MESS-019).
//
// "Which number did X send the most text messages to?" was written as a ranking over v_contact_pairs, which counts the events of every kind together and has no event_type column. The
// first two drafts asked for event_type there (UNKNOWN_COLUMN: the repair listed the columns of v_contact_pairs and said nothing of the view that has the one asked for), the third
// dropped the kind (EVENT_TYPE: the repair said "filter event_type = 'sms'", which that view cannot do), and the question was abstained. A repair has to say where the thing lives: the
// views that have the column are read from the catalogue the query was validated against, not from a list in the code.

// askViewsWithColumn are the views offered for the question (the shortlist; all of the catalogue when none was given) that have a column of this name, in alphabetical order.
func askViewsWithColumn(cat *askCatalogue, offered []string, column string) []string {
	if cat == nil {
		return nil
	}
	var out []string
	for _, view := range cat.Views {
		if len(offered) > 0 && !askContains(offered, view.Name) {
			continue
		}
		for _, c := range view.Columns {
			if c.Name == column {
				out = append(out, view.Name)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// askEventTypeMessage is what the model is told when the question names a kind of event (SMS, MMS, USSD) and the query does not filter on it. When the views the query reads cannot carry
// the kind at all, the message says which view can.
func askEventTypeMessage(kind string, v *askValidated) string {
	message := fmt.Sprintf("The question is about %s: filter event_type = '%s'.", strings.ToUpper(kind), kind)
	if v == nil {
		return message
	}
	carriers := askViewsWithColumn(v.Cat, v.Allowed, "event_type")
	if len(carriers) == 0 {
		return message
	}
	for _, view := range v.Views {
		if askContains(carriers, view) {
			return message
		}
	}
	return message + fmt.Sprintf(" %s counts every kind of event together and has no event_type column, so it cannot say how many of them were %s: read %s, which has one row for each event (src is the subscriber whose records they are, dst the other party), and group or count there.",
		strings.Join(v.Views, " and "), strings.ToUpper(kind), strings.Join(carriers, " or "))
}

// askColumnHomeHint says where a column that the query asked for lives, when it is a column of an offered view the query does not read: the UNKNOWN_COLUMN repair of a model that reached
// for the right column in the wrong view. "" when no other offered view has it.
func askColumnHomeHint(offered map[string]*askView, used []string, column string) string {
	var homes []string
	for name, view := range offered {
		if askContains(used, name) {
			continue
		}
		for _, c := range view.Columns {
			if c.Name == column {
				homes = append(homes, name)
				break
			}
		}
	}
	if len(homes) == 0 {
		return ""
	}
	sort.Strings(homes)
	return fmt.Sprintf(" %s is a column of %s, which this query does not read.", column, strings.Join(homes, " and "))
}
