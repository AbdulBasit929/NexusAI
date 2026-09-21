import collections, hashlib, json
from pathlib import Path

ROOT=Path(__file__).resolve().parents[1]; OUT=ROOT/'reports/nxb21'
read=lambda p:json.loads((ROOT/p).read_text(encoding='utf-8-sig'))
inventory=read('reports/nxb21/reconciliation-inventory-v1.json')
refs=read('api/forensic_records/contracts/query-capability-references-v1.json')['capabilities']
certify={r['operation_id']:r for r in read('reports/nxb21/c-catalog-only-disposition-v1.json')['records']}
byop=collections.defaultdict(list)
for ref in refs: byop[ref['operation_ref']].append(ref['query_capability_id'])
ledger=collections.defaultdict(list)
for row in inventory['query_ledger']: ledger[row['canonical_operation']].append(row)
records=[]
for operation in inventory['operations']:
    op=operation['operation_id']; cert=operation['certification']; caps=byop[op]
    if caps: disposition='ANALYST_FACING_CAPABILITY'
    elif op in certify: disposition=certify[op]['disposition']
    else: disposition='REGRESSION_ONLY_PENDING_EF_REVIEW'
    proof=lambda key,positive='RECORDED': positive if cert.get(key) else 'REQUIRED_IN_F'
    records.append({'operation_id':op,'family':operation['family_id'],'tier':operation['criticality_tier'],'query_capability_ids':caps,
      'analyst_facing_disposition':disposition,'canonical_certification':cert['certification_level'],
      'planner_coverage':{'historical_variants':len(ledger[op]),'languages':sorted({r['language'] for r in ledger[op]}),'D_dynamic_capability_reference':bool(caps)},
      'semantic_coverage':'C_REFERENCE' if caps else 'HISTORICAL_EXAMPLES_ONLY' if ledger[op] else 'NONE',
      'positive_result_proof':proof('calculation_pass'),'negative_result_proof':'REQUIRED_IN_F','oracle_type':'INDEPENDENT_DB_ARTIFACT_OR_MATH_REQUIRED',
      'scope_proof':'REQUIRED_IN_F','current_version_proof':'REQUIRED_IN_F','citation_proof':proof('citation_pass'),'navigation_proof':'REQUIRED_IN_F',
      'activity_proof':'REQUIRED_IN_F','ui_proof':proof('presentation_pass'),'performance_proof':'REQUIRED_IN_F','security_proof':proof('security_pass'),
      'language_applicability':sorted({r['language'] for r in ledger[op]}) or ['CLASSIFY_IN_E'],
      'future_F_disposition':disposition if disposition in {'INTERNAL_ONLY','DEFER_DEPTH'} else 'CERTIFY_IF_ADMITTED',
      'blockers':[] if disposition=='INTERNAL_ONLY' else ['independent_result_oracle','positive_and_zero','scope_current_version','citation_navigation_activity_ui_performance']})
assert len(records)==79 and len({r['operation_id'] for r in records})==79
(OUT/'d-operation-verification-map-v1.json').write_text(json.dumps({'schema_version':'nexusai.nxb21.d-operation-verification-map/v1','status':'ALL_79_ACCOUNTED_NOT_CERTIFIED','operation_count':79,'records':records},ensure_ascii=False,indent=2)+'\n',encoding='utf8')

corpus=read('reports/nxb21/d-unseen-query-corpus-v1.json')
caprows=[]
for ref in refs:
    cases=[c for c in corpus['cases'] if c.get('expected',{}).get('capability')==ref['query_capability_id']]
    caprows.append({'query_capability_id':ref['query_capability_id'],'operation_ref':ref['operation_ref'],'reference_kind':ref['reference_kind'],'semantics':ref['semantics'],'scope_modes':ref['scope_modes'],'corpus_cases':len(cases),'languages':sorted({c['language'] for c in cases}),'proposal_contract_validation':'PASS_SOURCE','natural_language_model_evaluation':'BLOCKED_MODEL_NOT_LOADED'})
(OUT/'d-planner-capability-matrix-v1.json').write_text(json.dumps({'schema_version':'nexusai.nxb21.d-planner-capability-matrix/v1','status':'SOURCE_CONTRACT_VALIDATED_MODEL_EVALUATION_BLOCKED','capability_count':22,'rows':caprows},ensure_ascii=False,indent=2)+'\n',encoding='utf8')
print(json.dumps({'operations':len(records),'capabilities':len(caprows),'corpus':len(corpus['cases'])}))
