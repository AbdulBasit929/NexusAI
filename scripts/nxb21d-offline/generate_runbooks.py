import json,re
from pathlib import Path
R=Path(__file__).resolve().parents[2]; OUT=R/'reports/nxb21'; PRIVATE=R/'local-acceptance-models/nxb21-demo'
load=lambda p:json.loads((R/p).read_text(encoding='utf-8-sig'))
refs=load('api/forensic_records/contracts/query-capability-references-v1.json')['capabilities']
ops=load('reports/nxb21/d-operation-verification-map-v1.json')['records']; opmap={x['operation_id']:x for x in ops}
examples={
'cdr.identifier_lookup':('Find records for {PHONE}','{PHONE} کے ریکارڈ دکھائیں','{PHONE} ke records dikhao'),
'cdr.frequent_contacts':('Show top 10 contacts for {PHONE}','{PHONE} کے دس زیادہ رابطے دکھائیں','{PHONE} ke top 10 contacts dikhao'),
'cdr.time_activity':('Summarize call activity for {PHONE} from {DATE1} to {DATE2}','{DATE1} سے {DATE2} تک {PHONE} کی کال سرگرمی دکھائیں','{DATE1} se {DATE2} tak {PHONE} ki call activity dikhao'),
'ipdr.endpoint':('Summarize traffic for {IP}','{IP} کا ٹریفک خلاصہ دکھائیں','{IP} ka traffic summary dikhao'),
'ipdr.sessions':('Show subscriber sessions for {PHONE}','{PHONE} کے سیشن دکھائیں','{PHONE} ke sessions dikhao'),
'subscriber.lookup':('Look up subscriber observations for {PHONE}','{PHONE} کی سبسکرائبر معلومات دکھائیں','{PHONE} ki subscriber observations dikhao'),
'device.observations':('Group device observations for {IMEI}','{IMEI} کے ڈیوائس مشاہدات دکھائیں','{IMEI} ke device observations dikhao'),
'tower.lookup':('Look up tower {SITE}','ٹاور {SITE} دکھائیں','tower {SITE} dikhao'),
'financial.summary':('Summarize transactions for {ACCOUNT}','{ACCOUNT} کی ٹرانزیکشنز کا خلاصہ دیں','{ACCOUNT} ki transactions ka summary do'),
'logs.failures':('Show failed events for {USER}','{USER} کے ناکام واقعات دکھائیں','{USER} ke failed events dikhao'),
'generic.filter':('Filter rows where {FIELD} equals {VALUE}','وہ قطاریں دکھائیں جہاں {FIELD} برابر {VALUE} ہو','rows dikhao jahan {FIELD} {VALUE} ho'),
'document.phrase':('Find the exact phrase "{PHRASE}" in this document','اس دستاویز میں عین عبارت "{PHRASE}" تلاش کریں','is document mein exact phrase "{PHRASE}" dhoondo'),
'knowledge.semantic':('Find evidence about {TOPIC}','{TOPIC} کے بارے میں ثبوت تلاش کریں','{TOPIC} ke bare mein evidence dhoondo'),
'ocr.phrase':('Find OCR text containing "{PHRASE}"','OCR متن میں "{PHRASE}" تلاش کریں','OCR text mein "{PHRASE}" dhoondo'),
'transcript.phrase':('Find where the transcript contains "{PHRASE}"','ٹرانسکرپٹ میں "{PHRASE}" کہاں ہے','transcript mein "{PHRASE}" kahan hai'),
'roman_urdu.phrase':('Search Roman Urdu transcript for "{PHRASE}"','رومن اردو ٹرانسکرپٹ میں "{PHRASE}" تلاش کریں','Roman Urdu transcript mein "{PHRASE}" dhoondo'),
'image.plate':('Find exact plate {PLATE} in this image','اس تصویر میں عین نمبر پلیٹ {PLATE} تلاش کریں','is image mein exact plate {PLATE} dhoondo'),
'video.group_plate':('Show grouped timeline for plate {PLATE} in this video','اس ویڈیو میں پلیٹ {PLATE} کی گروپ ٹائم لائن دکھائیں','is video mein plate {PLATE} ki grouped timeline dikhao'),
'image.similarity':('Find images visually similar to this selected image','اس منتخب تصویر سے ملتی جلتی تصاویر دکھائیں','is selected image jaisi images dikhao'),
'face.candidates':('Show similar face candidates for this selected face','اس منتخب چہرے کے مشابہ امیدوار دکھائیں','is selected face ke similar candidates dikhao'),
'cross_family.identifiers':('Correlate {PHONE} across authorized evidence families','مجاز ثبوت میں {PHONE} کا باہمی تعلق دکھائیں','authorized evidence mein {PHONE} ka cross-family link dikhao'),
'case.evidence_package':('Summarize this investigation evidence package','اس تفتیش کے ثبوت کا خلاصہ دیں','is investigation ke evidence package ka summary do')}
live_ids={'cdr.frequent_contacts','cdr.time_activity','document.phrase','ocr.phrase','transcript.phrase','roman_urdu.phrase','image.plate','video.group_plate'}
no_real={'image.similarity','face.candidates','knowledge.semantic'}
rows=[]
for x in refs:
 cid=x['query_capability_id']; op=opmap.get(x['operation_ref'],{}); en,ur,ru=examples[cid]
 status='LIVE_RUNNABLE_WITH_LIMITATION' if cid in live_ids else ('NO_CURRENT_REAL_EVIDENCE' if cid in no_real else 'SOURCE_CANDIDATE_ONLY')
 rows.append({'capability_id':cid,'human_name':cid.replace('.',' ').replace('_',' ').title(),'family':x['family'],'underlying_reference':x['operation_ref'],'authority_type':x['authority'],'reference_kind':x['reference_kind'],'canonical_certification':op.get('canonical_certification','DIRECT_DESCRIPTOR_NOT_OPERATION_CERTIFICATION'),'source_validation_state':'SOURCE_CONTRACT_VALIDATED_MODEL_EVALUATION_PENDING','live_today_state':status,'b1_c_d_source_candidate_state':'SOURCE_VALIDATED_NOT_DEPLOYED','required_evidence_or_input':x['input_types'],'current_real_evidence_available':'KNOWN_FOR_DEMO' if cid in live_ids else ('NOT_ESTABLISHED' if cid in no_real else 'REQUIRES_DYNAMIC_DISCOVERY'),'scope_modes':x['scope_modes'],'query_existing_vs_process_new':'QUERY_EXISTING_RESULTS_ONLY','supported_semantics':x['semantics'],'language_applicability':{'english':True,'urdu':True,'roman_urdu':True,'mixed':True},'expected_result_type':x['authority'],'zero_no_match_behavior':'Typed zero/no-match only when executor completeness supports absence; otherwise incomplete/unavailable.','citation_locator':x['locator_types'],'activity_expectation':'One retained Activity only for a normal live Ask; offline model evaluation creates none.','known_limitations':x.get('limitations',[]),'demo_status':'TIER_1_DEMO_NOW' if cid in live_ids else ('TIER_4_DO_NOT_DEMO_YET' if cid in no_real else 'TIER_2_AFTER_ACTIVATION'),'F_certification_requirement':'Independent positive, zero, scope, current-version, citation, navigation and Activity proof.','target_discovery':'Select an authorized source, inspect its current cited row/artifact read-only, and copy an exact identifier/literal from that source.','queries':{'canonical_english':en,'alternative_english':en.replace('Show','List').replace('Find','Locate'),'urdu':ur,'roman_urdu':ru,'mixed':ru+' please','positive':en,'zero_no_match':en.replace('{PHONE}','{CONFIRMED_ABSENT_PHONE}').replace('{PLATE}','{CONFIRMED_ABSENT_PLATE}').replace('{PHRASE}','{CONFIRMED_ABSENT_PHRASE}'),'follow_up':'Show the same result in the other authorized scope.'}})
(OUT/'d-22-capability-real-data-runbook-v1.json').write_text(json.dumps({'schema_version':'nexusai.nxb21d-22-capability-real-data-runbook/v1','status':'SOURCE_LIVE_SEPARATED_NOT_CERTIFIED','count':len(rows),'rows':rows},ensure_ascii=False,indent=2)+'\n',encoding='utf8')
oprows=[]
for x in ops:
 oprows.append({**x,'query_capability':x.get('query_capability_ids',[]),'real_evidence_available':'REQUIRES_F_TARGET_DISCOVERY','planner_proof':x.get('planner_coverage'),'executor_proof':x.get('semantic_coverage'),'positive_proof':x.get('positive_result_proof'),'negative_proof':x.get('negative_result_proof'),'oracle_defined':x.get('oracle_type'),'F_certification_required':x.get('future_F_disposition')!='NO_F_CERTIFICATION_REQUIRED','example_query_pattern':examples.get((x.get('query_capability_ids')or[''])[0],('','',''))[0] if x.get('query_capability_ids') else ''})
(OUT/'d-79-operation-execution-verification-map-v1.json').write_text(json.dumps({'schema_version':'nexusai.nxb21d-79-operation-execution-verification-map/v1','status':'ALL_79_ACCOUNTED_NOT_CERTIFIED','count':len(oprows),'rows':oprows},ensure_ascii=False,indent=2)+'\n',encoding='utf8')

PRIVATE.mkdir(parents=True,exist_ok=True);private={'schema_version':'nexusai.nxb21d-private-real-data-targets/v1','status':'DYNAMIC_DISCOVERY_REQUIRED_BEFORE_DEMO','generated_from_retained_snapshot_read_only':True,'entries':[]}
snap=load('local-acceptance-models/nxb21-reconciliation/snapshot.json');blob=json.dumps(snap,ensure_ascii=False)
patterns={'phone':r'(?<!\d)(?:92|0)3\d{9}(?!\d)','ip':r'(?<![\d.])(?:\d{1,3}\.){3}\d{1,3}(?![\d.])','plate':r'\b[A-Z]{2,3}[ -]?\d{2,4}[A-Z]{0,3}\b'}
targets={k:(re.findall(p,blob)[0] if re.findall(p,blob) else None) for k,p in patterns.items()}
for row in rows:
 kind='phone' if row['capability_id'].startswith(('cdr.','subscriber.','cross_family.','ipdr.sessions')) else ('ip' if row['capability_id']=='ipdr.endpoint' else ('plate' if row['capability_id'] in {'image.plate','video.group_plate'} else None))
 if kind and targets.get(kind): private['entries'].append({'capability_id':row['capability_id'],'runtime_derived_target':targets[kind],'source':'read-only reconciliation snapshot','positive_query':row['queries']['positive'].replace('{PHONE}',targets[kind]).replace('{IP}',targets[kind]).replace('{PLATE}',targets[kind]),'scope':row['scope_modes'][0],'operation':row['underlying_reference'],'expected_oracle_summary':'Recompute from current source rows/artifacts before F.','citation_expectation':row['citation_locator'],'demo_flag':'CANDIDATE_REQUIRES_F_ORACLE'})
(PRIVATE/'current-real-data-query-targets-v1.json').write_text(json.dumps(private,ensure_ascii=False,indent=2)+'\n',encoding='utf8')

md=['# NX-B2.1 current real-data query runbook','', 'Status: **source/live separated; 79 operations accounted for, not certified.**','', '# What I Can Run Right Now','', 'Use only rows marked `LIVE_RUNNABLE_WITH_LIMITATION`; select the stated authorized source first. The current live runtime predates B1+C+D source activation.','', '| Capability | Select | English query | Urdu / Roman query | Result / authority | Citation | Limitation |','|---|---|---|---|---|---|---|']
for r in rows:
 if r['live_today_state'].startswith('LIVE_'):md.append(f"| `{r['capability_id']}` | Authorized {r['family']} evidence | {r['queries']['canonical_english']} | {r['queries']['urdu']} / {r['queries']['roman_urdu']} | {r['expected_result_type']} via `{r['underlying_reference']}` | {', '.join(r['citation_locator'])} | Current live routing may require established phrasing; verify against the independent oracle. |")
md += ['', '# What Becomes Available After B1+C+D Activation','', 'The source candidate adds the strict 22-capability proposal, exact literal/identifier validation, scope-safe dynamic planning, source/calendar-time separation, and fail-closed unsupported requests. These remain unavailable in the current live images until a separately approved activation passes.','', '# 22 Query Capability Inventory','', '| Capability | Family | Reference | Kind | Live today | Candidate | Demo tier |','|---|---|---|---|---|---|---|']
for r in rows:md.append(f"| `{r['capability_id']}` | {r['family']} | `{r['underlying_reference']}` | {r['reference_kind']} | {r['live_today_state']} | {r['b1_c_d_source_candidate_state']} | {r['demo_status']} |")
for title,key in [('English Query Examples','canonical_english'),('Urdu Query Examples','urdu'),('Roman Urdu Query Examples','roman_urdu'),('Mixed Query Examples','mixed'),('Positive Query Patterns','positive'),('Zero / No-Match Patterns','zero_no_match')]:
 md+=['',f'# {title}',''];md += [f"- `{r['capability_id']}`: {r['queries'][key]}" for r in rows]
md += ['', '# Team-Lead Demo Tiers',''];
for tier in range(1,5):md.append(f"- **Tier {tier}:** "+', '.join('`'+r['capability_id']+'`' for r in rows if r['demo_status'].startswith(f'TIER_{tier}')))
pack=['cdr.frequent_contacts','cdr.time_activity','transcript.phrase','roman_urdu.phrase','document.phrase','ocr.phrase','image.plate','video.group_plate','ipdr.endpoint','cross_family.identifiers']
md += ['', '# Best Team-Lead Query Pack',''];
for i,cid in enumerate(pack,1):
 r=next(x for x in rows if x['capability_id']==cid);md.append(f"{i}. Select authorized **{r['family']}** evidence; ask: “{r['queries']['canonical_english']}”. Expect {r['expected_result_type']} from `{r['underlying_reference']}` and open its {', '.join(r['citation_locator'])} citation. Do not claim certification or identity beyond the stated authority.")
md += ['', '# 79-Operation Appendix','', 'The machine-readable appendix contains all 79 rows and every E/F proof field: `d-79-operation-execution-verification-map-v1.json`. **79 OPERATIONS ACCOUNTED FOR; 79 operations are not certified.**','', '# Independent Oracle Map','', '- Structured results: read-only canonical database rows and independent calculations.','- Document/OCR/ASR: native extracted block or retained raw artifact, including source-time.','- ANPR: retained plate observation/group with frame/region locator.','- Similarity/face: independently inspect candidate vectors/scores; never claim identity.','- Cross-family: verify every constituent authority separately.','', '# Real-Data Target Discovery','', 'Use the ignored private manifest as a starting point, then reconfirm every target from current authorized evidence immediately before a demo. Never copy private target values into this public report.','', '# Certification Boundary','', 'Planner evaluation proves question → plan only. E/F must prove plan → correct result, citation, navigation and Activity behavior.']
(OUT/'nexusai-nxb21-current-real-data-query-runbook-v1-20260903.md').write_text('\n'.join(md)+'\n',encoding='utf8')
print(json.dumps({'capabilities':len(rows),'operations':len(oprows),'private_targets':len(private['entries'])}))
