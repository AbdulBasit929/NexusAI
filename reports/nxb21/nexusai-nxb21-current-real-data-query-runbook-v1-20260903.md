# NX-B2.1 current real-data query runbook

Status: **source/live separated; 79 operations accounted for, not certified.**

# What I Can Run Right Now

Use only rows marked `LIVE_RUNNABLE_WITH_LIMITATION`; select the stated authorized source first. The current live runtime predates B1+C+D source activation.

| Capability | Select | English query | Urdu / Roman query | Result / authority | Citation | Limitation |
|---|---|---|---|---|---|---|
| `cdr.frequent_contacts` | Authorized communications_cdr evidence | Show top 10 contacts for {PHONE} | {PHONE} کے دس زیادہ رابطے دکھائیں / {PHONE} ke top 10 contacts dikhao | DETERMINISTIC_STRUCTURED_FACT via `cdr.frequent_contacts` | row, evidence | Current live routing may require established phrasing; verify against the independent oracle. |
| `cdr.time_activity` | Authorized communications_cdr evidence | Summarize call activity for {PHONE} from {DATE1} to {DATE2} | {DATE1} سے {DATE2} تک {PHONE} کی کال سرگرمی دکھائیں / {DATE1} se {DATE2} tak {PHONE} ki call activity dikhao | DETERMINISTIC_STRUCTURED_FACT via `cdr.temporal_activity` | row, evidence | Current live routing may require established phrasing; verify against the independent oracle. |
| `document.phrase` | Authorized document_intelligence evidence | Find the exact phrase "{PHRASE}" in this document | اس دستاویز میں عین عبارت "{PHRASE}" تلاش کریں / is document mein exact phrase "{PHRASE}" dhoondo | SOURCE_TEXT via `document.search` | page, artifact, evidence | Current live routing may require established phrasing; verify against the independent oracle. |
| `ocr.phrase` | Authorized image_intelligence evidence | Find OCR text containing "{PHRASE}" | OCR متن میں "{PHRASE}" تلاش کریں / OCR text mein "{PHRASE}" dhoondo | MODEL_OBSERVATION via `image.ocr_search` | region, artifact, evidence | Current live routing may require established phrasing; verify against the independent oracle. |
| `transcript.phrase` | Authorized audio_intelligence evidence | Find where the transcript contains "{PHRASE}" | ٹرانسکرپٹ میں "{PHRASE}" کہاں ہے / transcript mein "{PHRASE}" kahan hai | MODEL_OBSERVATION via `audio.transcript_search` | source_time, artifact, evidence | Current live routing may require established phrasing; verify against the independent oracle. |
| `roman_urdu.phrase` | Authorized audio_intelligence evidence | Search Roman Urdu transcript for "{PHRASE}" | رومن اردو ٹرانسکرپٹ میں "{PHRASE}" تلاش کریں / Roman Urdu transcript mein "{PHRASE}" dhoondo | DERIVED_REPRESENTATION via `audio.transcript_search` | source_time, artifact, evidence | Current live routing may require established phrasing; verify against the independent oracle. |
| `image.plate` | Authorized anpr_vehicles evidence | Find exact plate {PLATE} in this image | اس تصویر میں عین نمبر پلیٹ {PLATE} تلاش کریں / is image mein exact plate {PLATE} dhoondo | MODEL_OBSERVATION via `anpr.sightings` | region, artifact, evidence | Current live routing may require established phrasing; verify against the independent oracle. |
| `video.group_plate` | Authorized anpr_vehicles evidence | Show grouped timeline for plate {PLATE} in this video | اس ویڈیو میں پلیٹ {PLATE} کی گروپ ٹائم لائن دکھائیں / is video mein plate {PLATE} ki grouped timeline dikhao | MODEL_OBSERVATION via `video.anpr_grouped_timeline` | frame, source_time, artifact, evidence | Current live routing may require established phrasing; verify against the independent oracle. |

# What Becomes Available After B1+C+D Activation

The source candidate adds the strict 22-capability proposal, exact literal/identifier validation, scope-safe dynamic planning, source/calendar-time separation, and fail-closed unsupported requests. These remain unavailable in the current live images until a separately approved activation passes.

# 22 Query Capability Inventory

| Capability | Family | Reference | Kind | Live today | Candidate | Demo tier |
|---|---|---|---|---|---|---|
| `cdr.identifier_lookup` | case_cross_family | `forensics.canonical_records` | QUERY_OPERATION | SOURCE_CANDIDATE_ONLY | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_2_AFTER_ACTIVATION |
| `cdr.frequent_contacts` | communications_cdr | `cdr.frequent_contacts` | QUERY_OPERATION | LIVE_RUNNABLE_WITH_LIMITATION | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_1_DEMO_NOW |
| `cdr.time_activity` | communications_cdr | `cdr.temporal_activity` | QUERY_OPERATION | LIVE_RUNNABLE_WITH_LIMITATION | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_1_DEMO_NOW |
| `ipdr.endpoint` | network_ipdr | `ipdr.endpoint_summary` | QUERY_OPERATION | SOURCE_CANDIDATE_ONLY | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_2_AFTER_ACTIVATION |
| `ipdr.sessions` | network_ipdr | `ipdr.subscriber_sessions` | QUERY_OPERATION | SOURCE_CANDIDATE_ONLY | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_2_AFTER_ACTIVATION |
| `subscriber.lookup` | subscriber_identity | `subscriber.identity_lookup` | QUERY_OPERATION | SOURCE_CANDIDATE_ONLY | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_2_AFTER_ACTIVATION |
| `device.observations` | communications_cdr | `forensics.imei_imsi_usage` | QUERY_OPERATION | SOURCE_CANDIDATE_ONLY | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_2_AFTER_ACTIVATION |
| `tower.lookup` | tower_location | `tower.site_lookup` | QUERY_OPERATION | SOURCE_CANDIDATE_ONLY | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_2_AFTER_ACTIVATION |
| `financial.summary` | financial_transactions | `financial.transaction_summary` | QUERY_OPERATION | SOURCE_CANDIDATE_ONLY | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_2_AFTER_ACTIVATION |
| `logs.failures` | access_security_logs | `access.failed_events` | QUERY_OPERATION | SOURCE_CANDIDATE_ONLY | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_2_AFTER_ACTIVATION |
| `generic.filter` | generic_tabular | `generic.filter_records` | QUERY_OPERATION | SOURCE_CANDIDATE_ONLY | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_2_AFTER_ACTIVATION |
| `document.phrase` | document_intelligence | `document.search` | QUERY_OPERATION | LIVE_RUNNABLE_WITH_LIMITATION | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_1_DEMO_NOW |
| `knowledge.semantic` | knowledge_evidence | `forensics.evidence` | QUERY_OPERATION | NO_CURRENT_REAL_EVIDENCE | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_4_DO_NOT_DEMO_YET |
| `ocr.phrase` | image_intelligence | `image.ocr_search` | QUERY_OPERATION | LIVE_RUNNABLE_WITH_LIMITATION | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_1_DEMO_NOW |
| `transcript.phrase` | audio_intelligence | `audio.transcript_search` | QUERY_OPERATION | LIVE_RUNNABLE_WITH_LIMITATION | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_1_DEMO_NOW |
| `roman_urdu.phrase` | audio_intelligence | `audio.transcript_search` | QUERY_OPERATION | LIVE_RUNNABLE_WITH_LIMITATION | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_1_DEMO_NOW |
| `image.plate` | anpr_vehicles | `anpr.sightings` | QUERY_OPERATION | LIVE_RUNNABLE_WITH_LIMITATION | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_1_DEMO_NOW |
| `video.group_plate` | anpr_vehicles | `video.anpr_grouped_timeline` | QUERY_OPERATION | LIVE_RUNNABLE_WITH_LIMITATION | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_1_DEMO_NOW |
| `image.similarity` | image_intelligence | `image.visual_similarity` | DIRECT_DESCRIPTOR | NO_CURRENT_REAL_EVIDENCE | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_4_DO_NOT_DEMO_YET |
| `face.candidates` | face_intelligence | `face.similarity` | DIRECT_DESCRIPTOR | NO_CURRENT_REAL_EVIDENCE | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_4_DO_NOT_DEMO_YET |
| `cross_family.identifiers` | case_cross_family | `forensics.cross_family_correlation` | QUERY_OPERATION | SOURCE_CANDIDATE_ONLY | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_2_AFTER_ACTIVATION |
| `case.evidence_package` | case_cross_family | `forensics.evidence_package_summary` | QUERY_OPERATION | SOURCE_CANDIDATE_ONLY | SOURCE_VALIDATED_NOT_DEPLOYED | TIER_2_AFTER_ACTIVATION |

# English Query Examples

- `cdr.identifier_lookup`: Find records for {PHONE}
- `cdr.frequent_contacts`: Show top 10 contacts for {PHONE}
- `cdr.time_activity`: Summarize call activity for {PHONE} from {DATE1} to {DATE2}
- `ipdr.endpoint`: Summarize traffic for {IP}
- `ipdr.sessions`: Show subscriber sessions for {PHONE}
- `subscriber.lookup`: Look up subscriber observations for {PHONE}
- `device.observations`: Group device observations for {IMEI}
- `tower.lookup`: Look up tower {SITE}
- `financial.summary`: Summarize transactions for {ACCOUNT}
- `logs.failures`: Show failed events for {USER}
- `generic.filter`: Filter rows where {FIELD} equals {VALUE}
- `document.phrase`: Find the exact phrase "{PHRASE}" in this document
- `knowledge.semantic`: Find evidence about {TOPIC}
- `ocr.phrase`: Find OCR text containing "{PHRASE}"
- `transcript.phrase`: Find where the transcript contains "{PHRASE}"
- `roman_urdu.phrase`: Search Roman Urdu transcript for "{PHRASE}"
- `image.plate`: Find exact plate {PLATE} in this image
- `video.group_plate`: Show grouped timeline for plate {PLATE} in this video
- `image.similarity`: Find images visually similar to this selected image
- `face.candidates`: Show similar face candidates for this selected face
- `cross_family.identifiers`: Correlate {PHONE} across authorized evidence families
- `case.evidence_package`: Summarize this investigation evidence package

# Urdu Query Examples

- `cdr.identifier_lookup`: {PHONE} کے ریکارڈ دکھائیں
- `cdr.frequent_contacts`: {PHONE} کے دس زیادہ رابطے دکھائیں
- `cdr.time_activity`: {DATE1} سے {DATE2} تک {PHONE} کی کال سرگرمی دکھائیں
- `ipdr.endpoint`: {IP} کا ٹریفک خلاصہ دکھائیں
- `ipdr.sessions`: {PHONE} کے سیشن دکھائیں
- `subscriber.lookup`: {PHONE} کی سبسکرائبر معلومات دکھائیں
- `device.observations`: {IMEI} کے ڈیوائس مشاہدات دکھائیں
- `tower.lookup`: ٹاور {SITE} دکھائیں
- `financial.summary`: {ACCOUNT} کی ٹرانزیکشنز کا خلاصہ دیں
- `logs.failures`: {USER} کے ناکام واقعات دکھائیں
- `generic.filter`: وہ قطاریں دکھائیں جہاں {FIELD} برابر {VALUE} ہو
- `document.phrase`: اس دستاویز میں عین عبارت "{PHRASE}" تلاش کریں
- `knowledge.semantic`: {TOPIC} کے بارے میں ثبوت تلاش کریں
- `ocr.phrase`: OCR متن میں "{PHRASE}" تلاش کریں
- `transcript.phrase`: ٹرانسکرپٹ میں "{PHRASE}" کہاں ہے
- `roman_urdu.phrase`: رومن اردو ٹرانسکرپٹ میں "{PHRASE}" تلاش کریں
- `image.plate`: اس تصویر میں عین نمبر پلیٹ {PLATE} تلاش کریں
- `video.group_plate`: اس ویڈیو میں پلیٹ {PLATE} کی گروپ ٹائم لائن دکھائیں
- `image.similarity`: اس منتخب تصویر سے ملتی جلتی تصاویر دکھائیں
- `face.candidates`: اس منتخب چہرے کے مشابہ امیدوار دکھائیں
- `cross_family.identifiers`: مجاز ثبوت میں {PHONE} کا باہمی تعلق دکھائیں
- `case.evidence_package`: اس تفتیش کے ثبوت کا خلاصہ دیں

# Roman Urdu Query Examples

- `cdr.identifier_lookup`: {PHONE} ke records dikhao
- `cdr.frequent_contacts`: {PHONE} ke top 10 contacts dikhao
- `cdr.time_activity`: {DATE1} se {DATE2} tak {PHONE} ki call activity dikhao
- `ipdr.endpoint`: {IP} ka traffic summary dikhao
- `ipdr.sessions`: {PHONE} ke sessions dikhao
- `subscriber.lookup`: {PHONE} ki subscriber observations dikhao
- `device.observations`: {IMEI} ke device observations dikhao
- `tower.lookup`: tower {SITE} dikhao
- `financial.summary`: {ACCOUNT} ki transactions ka summary do
- `logs.failures`: {USER} ke failed events dikhao
- `generic.filter`: rows dikhao jahan {FIELD} {VALUE} ho
- `document.phrase`: is document mein exact phrase "{PHRASE}" dhoondo
- `knowledge.semantic`: {TOPIC} ke bare mein evidence dhoondo
- `ocr.phrase`: OCR text mein "{PHRASE}" dhoondo
- `transcript.phrase`: transcript mein "{PHRASE}" kahan hai
- `roman_urdu.phrase`: Roman Urdu transcript mein "{PHRASE}" dhoondo
- `image.plate`: is image mein exact plate {PLATE} dhoondo
- `video.group_plate`: is video mein plate {PLATE} ki grouped timeline dikhao
- `image.similarity`: is selected image jaisi images dikhao
- `face.candidates`: is selected face ke similar candidates dikhao
- `cross_family.identifiers`: authorized evidence mein {PHONE} ka cross-family link dikhao
- `case.evidence_package`: is investigation ke evidence package ka summary do

# Mixed Query Examples

- `cdr.identifier_lookup`: {PHONE} ke records dikhao please
- `cdr.frequent_contacts`: {PHONE} ke top 10 contacts dikhao please
- `cdr.time_activity`: {DATE1} se {DATE2} tak {PHONE} ki call activity dikhao please
- `ipdr.endpoint`: {IP} ka traffic summary dikhao please
- `ipdr.sessions`: {PHONE} ke sessions dikhao please
- `subscriber.lookup`: {PHONE} ki subscriber observations dikhao please
- `device.observations`: {IMEI} ke device observations dikhao please
- `tower.lookup`: tower {SITE} dikhao please
- `financial.summary`: {ACCOUNT} ki transactions ka summary do please
- `logs.failures`: {USER} ke failed events dikhao please
- `generic.filter`: rows dikhao jahan {FIELD} {VALUE} ho please
- `document.phrase`: is document mein exact phrase "{PHRASE}" dhoondo please
- `knowledge.semantic`: {TOPIC} ke bare mein evidence dhoondo please
- `ocr.phrase`: OCR text mein "{PHRASE}" dhoondo please
- `transcript.phrase`: transcript mein "{PHRASE}" kahan hai please
- `roman_urdu.phrase`: Roman Urdu transcript mein "{PHRASE}" dhoondo please
- `image.plate`: is image mein exact plate {PLATE} dhoondo please
- `video.group_plate`: is video mein plate {PLATE} ki grouped timeline dikhao please
- `image.similarity`: is selected image jaisi images dikhao please
- `face.candidates`: is selected face ke similar candidates dikhao please
- `cross_family.identifiers`: authorized evidence mein {PHONE} ka cross-family link dikhao please
- `case.evidence_package`: is investigation ke evidence package ka summary do please

# Positive Query Patterns

- `cdr.identifier_lookup`: Find records for {PHONE}
- `cdr.frequent_contacts`: Show top 10 contacts for {PHONE}
- `cdr.time_activity`: Summarize call activity for {PHONE} from {DATE1} to {DATE2}
- `ipdr.endpoint`: Summarize traffic for {IP}
- `ipdr.sessions`: Show subscriber sessions for {PHONE}
- `subscriber.lookup`: Look up subscriber observations for {PHONE}
- `device.observations`: Group device observations for {IMEI}
- `tower.lookup`: Look up tower {SITE}
- `financial.summary`: Summarize transactions for {ACCOUNT}
- `logs.failures`: Show failed events for {USER}
- `generic.filter`: Filter rows where {FIELD} equals {VALUE}
- `document.phrase`: Find the exact phrase "{PHRASE}" in this document
- `knowledge.semantic`: Find evidence about {TOPIC}
- `ocr.phrase`: Find OCR text containing "{PHRASE}"
- `transcript.phrase`: Find where the transcript contains "{PHRASE}"
- `roman_urdu.phrase`: Search Roman Urdu transcript for "{PHRASE}"
- `image.plate`: Find exact plate {PLATE} in this image
- `video.group_plate`: Show grouped timeline for plate {PLATE} in this video
- `image.similarity`: Find images visually similar to this selected image
- `face.candidates`: Show similar face candidates for this selected face
- `cross_family.identifiers`: Correlate {PHONE} across authorized evidence families
- `case.evidence_package`: Summarize this investigation evidence package

# Zero / No-Match Patterns

- `cdr.identifier_lookup`: Find records for {CONFIRMED_ABSENT_PHONE}
- `cdr.frequent_contacts`: Show top 10 contacts for {CONFIRMED_ABSENT_PHONE}
- `cdr.time_activity`: Summarize call activity for {CONFIRMED_ABSENT_PHONE} from {DATE1} to {DATE2}
- `ipdr.endpoint`: Summarize traffic for {IP}
- `ipdr.sessions`: Show subscriber sessions for {CONFIRMED_ABSENT_PHONE}
- `subscriber.lookup`: Look up subscriber observations for {CONFIRMED_ABSENT_PHONE}
- `device.observations`: Group device observations for {IMEI}
- `tower.lookup`: Look up tower {SITE}
- `financial.summary`: Summarize transactions for {ACCOUNT}
- `logs.failures`: Show failed events for {USER}
- `generic.filter`: Filter rows where {FIELD} equals {VALUE}
- `document.phrase`: Find the exact phrase "{CONFIRMED_ABSENT_PHRASE}" in this document
- `knowledge.semantic`: Find evidence about {TOPIC}
- `ocr.phrase`: Find OCR text containing "{CONFIRMED_ABSENT_PHRASE}"
- `transcript.phrase`: Find where the transcript contains "{CONFIRMED_ABSENT_PHRASE}"
- `roman_urdu.phrase`: Search Roman Urdu transcript for "{CONFIRMED_ABSENT_PHRASE}"
- `image.plate`: Find exact plate {CONFIRMED_ABSENT_PLATE} in this image
- `video.group_plate`: Show grouped timeline for plate {CONFIRMED_ABSENT_PLATE} in this video
- `image.similarity`: Find images visually similar to this selected image
- `face.candidates`: Show similar face candidates for this selected face
- `cross_family.identifiers`: Correlate {CONFIRMED_ABSENT_PHONE} across authorized evidence families
- `case.evidence_package`: Summarize this investigation evidence package

# Team-Lead Demo Tiers

- **Tier 1:** `cdr.frequent_contacts`, `cdr.time_activity`, `document.phrase`, `ocr.phrase`, `transcript.phrase`, `roman_urdu.phrase`, `image.plate`, `video.group_plate`
- **Tier 2:** `cdr.identifier_lookup`, `ipdr.endpoint`, `ipdr.sessions`, `subscriber.lookup`, `device.observations`, `tower.lookup`, `financial.summary`, `logs.failures`, `generic.filter`, `cross_family.identifiers`, `case.evidence_package`
- **Tier 3:** 
- **Tier 4:** `knowledge.semantic`, `image.similarity`, `face.candidates`

# Best Team-Lead Query Pack

1. Select authorized **communications_cdr** evidence; ask: “Show top 10 contacts for {PHONE}”. Expect DETERMINISTIC_STRUCTURED_FACT from `cdr.frequent_contacts` and open its row, evidence citation. Do not claim certification or identity beyond the stated authority.
2. Select authorized **communications_cdr** evidence; ask: “Summarize call activity for {PHONE} from {DATE1} to {DATE2}”. Expect DETERMINISTIC_STRUCTURED_FACT from `cdr.temporal_activity` and open its row, evidence citation. Do not claim certification or identity beyond the stated authority.
3. Select authorized **audio_intelligence** evidence; ask: “Find where the transcript contains "{PHRASE}"”. Expect MODEL_OBSERVATION from `audio.transcript_search` and open its source_time, artifact, evidence citation. Do not claim certification or identity beyond the stated authority.
4. Select authorized **audio_intelligence** evidence; ask: “Search Roman Urdu transcript for "{PHRASE}"”. Expect DERIVED_REPRESENTATION from `audio.transcript_search` and open its source_time, artifact, evidence citation. Do not claim certification or identity beyond the stated authority.
5. Select authorized **document_intelligence** evidence; ask: “Find the exact phrase "{PHRASE}" in this document”. Expect SOURCE_TEXT from `document.search` and open its page, artifact, evidence citation. Do not claim certification or identity beyond the stated authority.
6. Select authorized **image_intelligence** evidence; ask: “Find OCR text containing "{PHRASE}"”. Expect MODEL_OBSERVATION from `image.ocr_search` and open its region, artifact, evidence citation. Do not claim certification or identity beyond the stated authority.
7. Select authorized **anpr_vehicles** evidence; ask: “Find exact plate {PLATE} in this image”. Expect MODEL_OBSERVATION from `anpr.sightings` and open its region, artifact, evidence citation. Do not claim certification or identity beyond the stated authority.
8. Select authorized **anpr_vehicles** evidence; ask: “Show grouped timeline for plate {PLATE} in this video”. Expect MODEL_OBSERVATION from `video.anpr_grouped_timeline` and open its frame, source_time, artifact, evidence citation. Do not claim certification or identity beyond the stated authority.
9. Select authorized **network_ipdr** evidence; ask: “Summarize traffic for {IP}”. Expect DETERMINISTIC_STRUCTURED_FACT from `ipdr.endpoint_summary` and open its row, evidence citation. Do not claim certification or identity beyond the stated authority.
10. Select authorized **case_cross_family** evidence; ask: “Correlate {PHONE} across authorized evidence families”. Expect COMPOSED_VALIDATED_RESULT from `forensics.cross_family_correlation` and open its row, evidence citation. Do not claim certification or identity beyond the stated authority.

# 79-Operation Appendix

The machine-readable appendix contains all 79 rows and every E/F proof field: `d-79-operation-execution-verification-map-v1.json`. **79 OPERATIONS ACCOUNTED FOR; 79 operations are not certified.**

# Independent Oracle Map

- Structured results: read-only canonical database rows and independent calculations.
- Document/OCR/ASR: native extracted block or retained raw artifact, including source-time.
- ANPR: retained plate observation/group with frame/region locator.
- Similarity/face: independently inspect candidate vectors/scores; never claim identity.
- Cross-family: verify every constituent authority separately.

# Real-Data Target Discovery

Use the ignored private manifest as a starting point, then reconfirm every target from current authorized evidence immediately before a demo. Never copy private target values into this public report.

# Certification Boundary

Planner evaluation proves question → plan only. E/F must prove plan → correct result, citation, navigation and Activity behavior.
