package records

import (
	"regexp"
	"slices"
	"strings"
)

var nonIdentifier = regexp.MustCompile(`[^a-z0-9]+`)

var canonicalAliases = map[string][]string{
	"timestamp": {
		"timestamp", "time", "datetime", "date_time", "event_time", "event_timestamp",
		"start_time", "started_at", "call_time", "call_datetime", "sighting_time",
		"access_time", "transaction_time", "txn_time", "logged_at",
	},
	"date": {
		"date", "call_date", "event_date", "sighting_date", "transaction_date", "txn_date",
	},
	"duration_seconds": {
		"duration", "duration_sec", "duration_secs", "duration_seconds", "call_duration",
		"call_duration_sec", "call_duration_seconds", "billsec", "seconds",
	},
	"source_number": {
		"source_number", "src_number", "from_number", "calling_number", "caller",
		"a_number", "msisdn_a", "originating_number", "originator", "source_msisdn",
	},
	"target_number": {
		"target_number", "dst_number", "to_number", "called_number", "callee",
		"b_number", "msisdn_b", "terminating_number", "recipient", "target_msisdn",
	},
	"msisdn": {
		"msisdn", "phone", "phone_number", "mobile", "mobile_number", "subscriber_number",
	},
	"imsi": {
		"imsi", "subscriber_imsi",
	},
	"imei": {
		"imei", "device_imei", "handset_imei",
	},
	"cell_id": {
		"cell_id", "cellid", "cid", "cgi", "tower_id", "site_id", "enodeb", "eci",
	},
	"lac": {
		"lac", "location_area_code", "tac",
	},
	"area": {
		"area", "region", "zone", "city", "district", "sector",
	},
	"status": {
		"status", "call_status", "result", "disposition", "response_status",
	},
	"roaming": {
		"roaming", "is_roaming", "roam", "roaming_flag",
	},
	"call_type": {
		"call_type", "direction", "event_type", "cdr_type", "service_type",
	},
	"plate_number": {
		"plate", "plate_number", "license_plate", "licence_plate", "registration",
		"registration_no", "reg_no", "vehicle_registration", "vrn", "anpr",
	},
	"location": {
		"location", "camera_location", "site_location", "address", "place", "checkpoint",
	},
	"camera_id": {
		"camera", "camera_id", "camera_name", "checkpoint_id", "lane_camera",
	},
	"latitude": {
		"lat", "latitude", "gps_lat", "y",
	},
	"longitude": {
		"lon", "lng", "longitude", "gps_lon", "gps_lng", "x",
	},
	"ip": {
		"ip", "ip_address", "client_ip", "subscriber_ip", "user_ip",
	},
	"source_ip": {
		"source_ip", "src_ip", "srcaddr", "client_ip", "origin_ip",
	},
	"target_ip": {
		"target_ip", "destination_ip", "dst_ip", "dstaddr", "server_ip",
	},
	"source_port": {
		"source_port", "src_port", "sport",
	},
	"target_port": {
		"target_port", "destination_port", "dst_port", "dport",
	},
	"bytes": {
		"bytes", "byte_count", "total_bytes", "bytes_total", "octets",
	},
	"upload_bytes": {
		"upload_bytes", "bytes_up", "tx_bytes", "uplink_bytes",
	},
	"download_bytes": {
		"download_bytes", "bytes_down", "rx_bytes", "downlink_bytes",
	},
	"subscriber_name": {
		"name", "subscriber_name", "customer_name", "owner_name", "full_name",
	},
	"identifier": {
		"id", "identifier", "national_id", "cnic", "nic", "passport", "customer_id",
	},
	"account": {
		"account", "account_no", "account_number", "wallet", "iban",
	},
	"counterparty": {
		"counterparty", "beneficiary", "merchant", "receiver", "sender",
	},
	"transaction_id": {
		"transaction_id", "txn_id", "tx_id", "reference", "reference_no", "rrn",
	},
	"amount": {
		"amount", "txn_amount", "transaction_amount", "value", "debit", "credit",
	},
	"currency": {
		"currency", "ccy",
	},
	"http_method": {
		"method", "http_method", "verb",
	},
	"path": {
		"path", "url", "uri", "endpoint", "request_path",
	},
	"user_agent": {
		"user_agent", "ua", "browser",
	},
	"user": {
		"user", "username", "user_id", "login", "actor", "principal",
	},
}

var recordTypeSignals = map[string][]string{
	RecordTypeCDR: {
		"source_number", "target_number", "duration_seconds", "call_type", "cell_id", "imei", "imsi",
	},
	RecordTypeANPR: {
		"plate_number", "camera_id", "location", "sighting_time", "timestamp",
	},
	RecordTypeIPDR: {
		"source_ip", "target_ip", "ip", "source_port", "target_port", "bytes", "upload_bytes", "download_bytes",
	},
	RecordTypeSubscriber: {
		"msisdn", "subscriber_name", "identifier", "account", "imsi", "imei",
	},
	RecordTypeTowerLocation: {
		"cell_id", "lac", "latitude", "longitude", "area", "location",
	},
	RecordTypeTransaction: {
		"transaction_id", "amount", "currency", "account", "counterparty",
	},
	RecordTypeAccessLog: {
		"http_method", "path", "status", "ip", "source_ip", "user_agent", "user",
	},
}

func NormalizeFieldName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = nonIdentifier.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")
	return name
}

func CanonicalFieldName(name string) string {
	normalized := NormalizeFieldName(name)
	for canonical, aliases := range canonicalAliases {
		if canonical == normalized || slices.Contains(aliases, normalized) {
			return canonical
		}
	}
	return normalized
}

func AliasesForRecordType(recordType string) map[string][]string {
	out := map[string][]string{}
	for canonical, aliases := range canonicalAliases {
		if recordType == "" || recordType == RecordTypeGeneric || slices.Contains(recordTypeSignals[recordType], canonical) {
			out[canonical] = append([]string{canonical}, aliases...)
		}
	}
	return out
}

func inferRecordTypeFromFields(fields []FieldSchema) string {
	scores := map[string]int{}
	for _, f := range fields {
		canonical := f.CanonicalName
		if canonical == "" {
			canonical = f.NormalizedName
		}
		for recordType, signals := range recordTypeSignals {
			if slices.Contains(signals, canonical) {
				scores[recordType]++
			}
		}
	}

	bestType := RecordTypeGeneric
	bestScore := 0
	for _, recordType := range SupportedRecordTypes {
		if recordType == RecordTypeGeneric {
			continue
		}
		if scores[recordType] > bestScore {
			bestType = recordType
			bestScore = scores[recordType]
		}
	}
	if bestScore < 2 {
		return RecordTypeGeneric
	}
	return bestType
}
