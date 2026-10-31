package main

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ASK ENGINE V2: THE SETTINGS OF A CASE (migration 032). An analyst gives a case its own hub threshold (PATCH /cases/{id}); the engine reads it per question and uses it as the default of the
// hub_threshold parameter when the question names none. A question that names a number uses its own. The engine's environment setting (FORENSIC_ASK_V2_HUB_THRESHOLD) stays as the default of
// every case that has no setting of its own. The answer states the number used and where it came from (askParamNotes).

// askCaseSetting is what the case record holds for the Ask engine; a zero value means the case has no setting.
type askCaseSetting struct {
	HubThreshold int
}

// askCaseSettingsFn reads the case's settings. A variable so that a spec can stand in for the database.
var askCaseSettingsFn = readAskCaseSettings

func readAskCaseSettings(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID string) (askCaseSetting, error) {
	if db == nil {
		return askCaseSetting{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return askCaseSetting{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // read-only transaction
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return askCaseSetting{}, err
	}
	var hub *int
	err = tx.QueryRow(ctx, `SELECT hub_threshold FROM forensic.case_record WHERE tenant_id = $1 AND collection_id = $2`, tenantID, collectionID).Scan(&hub)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return askCaseSetting{}, nil // a case that was never named or changed has no record and no setting
	case err != nil:
		return askCaseSetting{}, err
	}
	setting := askCaseSetting{}
	if hub != nil {
		setting.HubThreshold = *hub
	}
	return setting, nil
}

// askCaseParamDefaults are the defaults the case and the engine give the parameters: the case's own setting first, then the engine's for every case, then (not here) the declaration's default.
func askCaseParamDefaults(setting askCaseSetting, engineHub int) map[string]askParamDefault {
	out := map[string]askParamDefault{}
	switch {
	case setting.HubThreshold > 0:
		out["hub_threshold"] = askParamDefault{Value: setting.HubThreshold, Source: "case"}
	case engineHub > 0:
		out["hub_threshold"] = askParamDefault{Value: engineHub, Source: "engine"}
	}
	return out
}
