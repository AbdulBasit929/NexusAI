package main

import (
	"regexp"
	"strconv"
	"strings"
)

// normalizeAnalystSemantics creates a routing-only copy of an analyst question.
// Entity extraction continues to use the original question so phone numbers,
// device identifiers, plates, source IDs, and dates are never rewritten by the
// language layer.
func normalizeAnalystSemantics(value string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
	for _, replacement := range analystSemanticReplacements {
		normalized = replacement.pattern.ReplaceAllString(normalized, replacement.value)
	}
	return strings.Join(strings.Fields(normalized), " ")
}

type analystSemanticReplacement struct {
	pattern *regexp.Regexp
	value   string
}

var analystSemanticReplacements = []analystSemanticReplacement{
	{regexp.MustCompile(`\bcdr\s+actvty\b`), "cdr activity"},
	{regexp.MustCompile(`\bfrequent\s+cntcts\b`), "frequent contacts"},
	{regexp.MustCompile(`\boutgng\b`), "outgoing"},
	{regexp.MustCompile(`\bincomng\b`), "incoming"},
	{regexp.MustCompile(`\bevidnce\b`), "evidence"},
	{regexp.MustCompile(`\bshow\s+(outgoing|incoming)\s+only\b`), "only $1"},
	{regexp.MustCompile(`\b(?:src|srcs)\b`), "sources"},
	{regexp.MustCompile(`\bwho\s+(?:did\s+)?he\s+call(?:ed)?\s+most\b`), "frequent contacts"},
	{regexp.MustCompile(`\bkis\s+se\s+contact\s+hua\b`), "frequent contacts"},
	{regexp.MustCompile(`\bsab\s+se\s+zyada\s+kis\s+number\s+se\s+baat\s+hui\b`), "frequent contacts"},
	{regexp.MustCompile(`\bsirf\s+outgoing\s+dikhao\b`), "only outgoing"},
	{regexp.MustCompile(`\bsirf\s+incoming\s+dikhao\b`), "only incoming"},
	{regexp.MustCompile(`\bactivity\s+(?:dikhao|batao)\b`), "cdr activity"},
	{regexp.MustCompile(`\bcommon\s+contacts\s+batao\b`), "common contacts"},
	{regexp.MustCompile(`\b(?:any\s+)?common\s+imei\b`), "shared imei observations"},
	{regexp.MustCompile(`\bsame\s+imei(?:\s+observation)?\s+hai\b`), "shared imei observations"},
	{regexp.MustCompile(`\bsame\s+tower\s+use\s+hua\b`), "shared tower observations"},
	{regexp.MustCompile(`\bcompare\s+karo\b`), "compare"},
	{regexp.MustCompile(`\bkis\s+waqt\s+hua\b`), "timeline"},
	{regexp.MustCompile(`\bevidence\s+dikhao\b`), "show evidence"},
	{regexp.MustCompile(`\b(?:dikhao|batao)\b`), "show"},
	{regexp.MustCompile(`سرگرمی\s+دکھائیں`), "cdr activity"},
	{regexp.MustCompile(`سب\s+سے\s+زیادہ\s+رابطہ`), "frequent contacts"},
	{regexp.MustCompile(`صرف\s+آؤٹ\s*گوئنگ`), "only outgoing"},
	{regexp.MustCompile(`صرف\s+اِن\s*کمنگ`), "only incoming"},
	{regexp.MustCompile(`مشترکہ\s+رابطے`), "common contacts"},
	{regexp.MustCompile(`ایک\s+ہی\s+imei`), "shared imei observations"},
	{regexp.MustCompile(`ایک\s+ہی\s+ٹاور`), "shared tower observations"},
	{regexp.MustCompile(`موازنہ\s+کریں`), "compare"},
	{regexp.MustCompile(`ثبوت\s+دکھائیں`), "show evidence"},
	{regexp.MustCompile(`(?:ویڈیو|وڈیو)`), "video"},
	{regexp.MustCompile(`(?:ایویڈنس|شواہد|ثبوت)`), "evidence"},
	{regexp.MustCompile(`اے\s*این\s*پی\s*آر`), "anpr"},
	{regexp.MustCompile(`(?:گروپڈ|گروپ\s*شدہ|گروہی)`), "grouped"},
	{regexp.MustCompile(`(?:ٹائم\s*لائن|وقت\s+کی\s+ترتیب)`), "timeline"},
	{regexp.MustCompile(`(?:مشاہدات|مشاہدے)`), "observations"},
	{regexp.MustCompile(`نمبر\s+پلیٹ(?:یں)?`), "plate"},
	{regexp.MustCompile(`گاڑی\s+کی\s+پلیٹ(?:یں)?`), "plate"},
	{regexp.MustCompile(`نظر\s+آ(?:ئی|یا|ئیں|ئے)`), "sighting"},
	{regexp.MustCompile(`کہاں`), "where"},
	{regexp.MustCompile(`(?:ملی|ملا|ملیں|ملے)`), "found"},
	{regexp.MustCompile(`تصویر`), "image"},
	{regexp.MustCompile(`کب`), "when"},
	{regexp.MustCompile(`(?:تلاش\s+کریں|تلاش\s+کرو)`), "find"},
	{regexp.MustCompile(`(?:دکھا\s+دیں|دکھا\s+دو)`), "show"},
	{regexp.MustCompile(`(?:دکھائیں|دکھاؤ|بتائیں|بتاؤ)`), "show"},
	{regexp.MustCompile(`outgoing\s+activity\s+show\s+karo`), "outgoing cdr activity"},
}

func looksLikeANPRPlateTarget(value string) bool {
	compact := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(value)))
	return regexp.MustCompile(`^[A-Z]{1,4}[0-9]{1,6}[A-Z]{0,3}$`).MatchString(compact)
}

var sourceSecondRangePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bfrom\s+([0-9]+(?:\.[0-9]+)?)\s*(?:seconds?|secs?|s)\s+(?:to|until|through)\s+([0-9]+(?:\.[0-9]+)?)\s*(?:seconds?|secs?|s)\b`),
	regexp.MustCompile(`(?i)\b(?:between\s+)?([0-9]+(?:\.[0-9]+)?)\s*(?:(?:seconds?|secs?|s)\s*)?(?:and|aur|to|-)\s*([0-9]+(?:\.[0-9]+)?)\s*(?:seconds?|secs?|s)\b`),
	regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*(?:سیکنڈ\s*)?سے\s*([0-9]+(?:\.[0-9]+)?)\s*سیکنڈ`),
}

func extractSourceSecondRange(value string) (*float64, *float64) {
	for _, pattern := range sourceSecondRangePatterns {
		match := pattern.FindStringSubmatch(value)
		if len(match) != 3 {
			continue
		}
		start, startErr := strconv.ParseFloat(match[1], 64)
		end, endErr := strconv.ParseFloat(match[2], 64)
		if startErr == nil && endErr == nil && start >= 0 && end >= 0 {
			return &start, &end
		}
	}
	return nil, nil
}

func bindVideoANPRQueryParameters(req hybridQueryRequest, planner runtimePlan) hybridQueryRequest {
	if planner.Template != "video_anpr_grouped_timeline" && planner.Template != "video_timeline" && planner.Template != "face_candidate_observations" {
		return req
	}
	if req.EvidenceID == "" {
		req.EvidenceID = strings.ToLower(forensicUUIDInTextPattern.FindString(req.Query))
	}
	if req.EvidenceID == "" && forensicUUIDPattern.MatchString(strings.TrimSpace(req.Target)) {
		req.EvidenceID = strings.ToLower(strings.TrimSpace(req.Target))
	}
	if planner.Template == "video_anpr_grouped_timeline" && req.Plate == "" {
		for _, candidate := range canonicalTargetSet(req.Target, req.Targets, planner.Targets) {
			if !forensicUUIDPattern.MatchString(candidate) && looksLikeANPRPlateTarget(candidate) {
				req.Plate = candidate
				break
			}
		}
	}
	if planner.Template != "face_candidate_observations" && req.StartSeconds == nil && req.EndSeconds == nil {
		req.StartSeconds, req.EndSeconds = extractSourceSecondRange(req.Query)
	}
	return req
}

func bindVideoANPRPrimaryTarget(req hybridQueryRequest, template string) hybridQueryRequest {
	if (template != "video_anpr_grouped_timeline" && template != "video_timeline" && template != "face_candidate_observations") || !forensicUUIDPattern.MatchString(strings.TrimSpace(req.EvidenceID)) {
		return req
	}
	req.EvidenceID = strings.ToLower(strings.TrimSpace(req.EvidenceID))
	req.Target = req.EvidenceID
	req.Targets = []string{req.EvidenceID}
	return req
}

func queryHasMixedScript(value string) bool {
	hasUrdu := regexp.MustCompile(`[\x{0600}-\x{06ff}]`).MatchString(value)
	hasLatin := regexp.MustCompile(`[A-Za-z]`).MatchString(value)
	return hasUrdu && hasLatin
}
