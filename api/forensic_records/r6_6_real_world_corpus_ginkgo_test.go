package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("R6.6 real-world query corpus", func() {
	type routeCase struct {
		query    string
		template string
	}
	routes := []routeCase{
		{"Which files were ingested?", "source_file_audit"},
		{"list the source files in this case", "source_file_audit"},
		{"is this case ready for analysis?", "case_readiness"},
		{"check evidence health and rejected rows", "case_readiness"},
		{"show known limitations and missing data", "limitations_and_data_quality"},
		{"Who does 923001110001 contact most?", "frequent_contacts"},
		{"top contacts of +92 300 1110001", "frequent_contacts"},
		{"is number ke frequent contacts dikhao 923001110001", "frequent_contacts"},
		{"show CDR call type breakdown", "call_type_breakdown"},
		{"show service usage by direction", "service_usage"},
		{"did 923001110001 change devices?", "device_identity_changes"},
		{"which devices were used by 923001110001?", "imei_imsi_usage"},
		{"show tower activity for 923001110001", "tower_activity"},
		{"show first and last seen for 923001110001", "first_seen_last_seen"},
		{"show activity by hour for 923001110001", "activity_by_hour"},
		{"show night activity for 923001110001", "night_activity"},
		{"show network session volume", "ipdr_session_volume"},
		{"show protocol breakdown for network sessions", "ipdr_protocol_breakdown"},
		{"show subscriber sessions for 923001110001", "ipdr_subscriber_sessions"},
		{"show concurrent sessions for 923001110001", "ipdr_concurrent_sessions"},
		{"show camera sequence for plate ABC-123", "anpr_camera_sequence"},
		{"show sightings for plate ABC-123", "anpr_sightings"},
		{"show route timing for plate ABC-123", "anpr_route_timing"},
		{"show observed plate variants for ABC-123", "anpr_plate_variants"},
		{"show subscriber status summary", "subscriber_status_summary"},
		{"subscriber status ka khulasa", "subscriber_status_summary"},
		{"audit tower coordinates and uncertainty", "tower_coordinate_audit"},
		{"show tower status summary", "tower_status_summary"},
		{"show provider tower history for PK-LHR-HIST-001", "tower_reference_timeline"},
		{"audit overlapping tower references", "tower_alias_conflicts"},
		{"show ICCID device links for 8992410000000000001", "subscriber_device_links"},
		{"show subscriber service links for PK-SVC-SYN-001", "subscriber_device_links"},
		{"classify packet data and USSD usage for 923001110001", "service_usage"},
		{"correlate 923001110001 across record families", "cross_family_correlation"},
		{"show duplicate uploaded files", "duplicate_upload_audit"},
	}

	It("routes meaningful natural and mixed-language variants", func() {
		for _, test := range routes {
			Expect(chooseTemplate(test.query, "")).To(Equal(test.template), test.query)
		}
	})

	It("preserves all 79 accepted operation examples", func() {
		Expect(supportedQueryTemplates()).To(HaveLen(79))
		for _, template := range supportedQueryTemplates() {
			Expect(chooseTemplate(template.ExampleQuery, "")).To(Equal(template.Name), template.ExampleQuery)
		}
	})

	It("separates clarification, unsupported capability, and valid target extraction", func() {
		Expect(needsClarification("who are the frequent contacts?", "frequent_contacts", "")).To(BeTrue())
		Expect(extractTarget("top contacts of +92 300 1110001")).To(Equal("923001110001"))
		Expect(extractTarget("show exact ANPR sightings for ZZZ-SYNTHETIC-NO-MATCH")).To(Equal("ZZZ-SYNTHETIC-NO-MATCH"))
		Expect(extractTarget("10 july 2026 ko 923001234567 ki CDR activity dikhao")).To(Equal("923001234567"))
		Expect(chooseTemplate("Show IPDR endpoint activity for 10.20.1.7.", "")).To(Equal("ipdr_endpoint_summary"))
		Expect(assessQueryCapability("transcribe and diarize this audio", "").Status).To(Equal("unavailable"))
	})
})
