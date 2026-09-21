package main

// Video observations are rows even when they contain nested crop/locator data.
// Keep this specialization out of the generic structured-record flattener.
func videoResultRows(value any) []map[string]any {
	rows := make([]map[string]any, 0)
	for _, original := range mapsFromAny(value) {
		if len(rows) == 100 {
			break
		}
		row := make(map[string]any, len(original)+4)
		for key, value := range original {
			row[key] = value
		}
		row["result_semantics"] = "video_anpr_observation"
		if row["match_kind"] == "selected_group_candidate" {
			row["result_semantics"] = "video_anpr_group"
		}
		locator := make(map[string]any)
		// Group citations use the retained group range; raw citations use the
		// actual observation instant. Never attach a best-frame time to a group start.
		for key, value := range decodedCitationLocator(firstPresent(row, "citation_locator", "group_locator", "best_observation_locator")) {
			locator[key] = value
		}
		locator["artifact_id"] = row["artifact_id"]
		if row["match_kind"] == "selected_group_candidate" {
			if locator["first_seen_seconds"] != nil {
				locator["start_seconds"] = locator["first_seen_seconds"]
			}
			if locator["last_seen_seconds"] != nil {
				locator["end_seconds"] = locator["last_seen_seconds"]
			}
		}
		row["citation_locator"] = locator
		if row["frame_number"] == nil && row["match_kind"] != "selected_group_candidate" {
			row["frame_number"] = locator["frame_number"]
		}
		if row["first_seen_seconds"] != nil {
			row["source_time"] = formatSourceSecond(numericFloat(row["first_seen_seconds"]))
			if row["last_seen_seconds"] != nil && numericFloat(row["last_seen_seconds"]) != numericFloat(row["first_seen_seconds"]) {
				row["source_time"] = row["source_time"].(string) + " – " + formatSourceSecond(numericFloat(row["last_seen_seconds"]))
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func videoResultColumns(rows []map[string]any) []string {
	columns := make([]string, 0)
	for _, key := range []string{"normalized_plate_text", "group_selected_plate_text", "match_kind", "source_time", "frame_number", "source_file", "sightings_count", "manual_review_required"} {
		for _, row := range rows {
			if row[key] != nil {
				columns = append(columns, key)
				break
			}
		}
	}
	return columns
}
