import collections, hashlib, json, re
from pathlib import Path

ROOT=Path(__file__).resolve().parents[1]; OUT=ROOT/'reports/nxb21'; PRIVATE=ROOT/'local-acceptance-models/nxb21-d'
read=lambda p:json.loads((ROOT/p).read_text(encoding='utf-8-sig'))
write=lambda n,o:(OUT/n).write_text(json.dumps(o,ensure_ascii=False,indent=2)+'\n',encoding='utf8')
corpus=read('reports/nxb21/d-unseen-query-corpus-v1.json'); cases=corpus['cases']
bylang=collections.Counter(c['language'] for c in cases)
language_rows=[]
for language,count in sorted(bylang.items()):
    language_rows.append({'language':language,'corpus_cases':count,'proposal_contract':'PASS_SOURCE','actual_model_cases':0,'accuracy':'NOT_MEASURED_MODEL_NOT_LOADED','threshold_percent':95})
write('d-language-results-v1.json',{'schema_version':'nexusai.nxb21.d-language-results/v1','status':'ACTUAL_MODEL_EVALUATION_PENDING','rows':language_rows,'critical_safety_threshold_percent':100,'mocked_schema_results_are_not_language_accuracy':True})

def matrix(name,dimension):
    selected=[c for c in cases if c['dimension']==dimension]
    write(name,{'schema_version':'nexusai.nxb21.'+name.replace('-v1.json','').replace('-','.')+'/v1','status':'SOURCE_EXPECTATIONS_FROZEN_MODEL_EVALUATION_PENDING','case_count':len(selected),'cases':[{'id':c['id'],'language':c['language'],'expected':c['expected'],'oracle':c['oracle'],'actual_model':'NOT_RUN'} for c in selected]})
matrix('d-followup-matrix-v1.json','governed_followup')
matrix('d-adversarial-matrix-v1.json','adversarial')
composition=[c for c in cases if c.get('expected',{}).get('semantic')=='COMPOSE']
write('d-composition-matrix-v1.json',{'schema_version':'nexusai.nxb21.d-composition-matrix/v1','status':'BOUNDED_SOURCE_CONTRACT_PASS_MODEL_EVALUATION_PENDING','maximum_steps':3,'existing_execution_budget_reused':True,'cases':[{'id':c['id'],'expected':c['expected'],'actual_model':'NOT_RUN'} for c in composition]})
model={'model_id':'qwen_qwen3-4b-instruct-2507','artifact':'Qwen_Qwen3-4B-Instruct-2507-Q8_0.gguf','artifact_bytes':4280405216,'quantization':'Q8_0','backend':'llama-cpp','gpu_layers':0,'threads':8,'configured_context_size':8192,'top_level_context_size':4096,'temperature_runtime_request':0,'installed':True,'loaded':False,'actual_inference_authorized':False,'host_total_memory_bytes':16871448576,'host_free_memory_at_inspection_bytes':3372187648}
metrics=['valid_schema_rate','capability_selection_accuracy','semantic_selection_accuracy','entity_preservation','literal_preservation','scope_accuracy','calendar_time_accuracy','source_time_accuracy','composition_accuracy','clarification_accuracy','english','urdu','roman_urdu','mixed_language','latency','peak_memory','invalid_or_invented_field_rate','invented_identifier_rate']
write('d-model-fitness-v1.json',{'schema_version':'nexusai.nxb21.d-model-fitness/v1','status':'BLOCKED_MODEL_NOT_LOADED','model':model,'acceptance':corpus['acceptance'],'metrics':{m:'NOT_MEASURED' for m in metrics},'source_contract_validation':'PASS','suitability':'NOT_YET_CLASSIFIABLE','better_model_required':'NOT_YET_DETERMINED','new_model_downloaded':False,'next_gate':'Explicit authorization to load the already-installed current model for non-retained evaluation; do not download or reconfigure.'})
variants=read('reports/nxb21/d-214-variant-regression-v1.json')['records']
counts=collections.Counter(r['classification'] for r in variants)
payload=read('reports/nxb21/d-214-variant-regression-v1.json');payload.update(status='PASS_CLASSIFIED_NOT_RESULT_CERTIFIED',variant_count=214,classification_counts=dict(counts),actual_planner_executed=True,retained_queries=False);write('d-214-variant-regression-v1.json',payload)
write('d-source-live-parity-v1.json',{'schema_version':'nexusai.nxb21.d-source-live-parity/v1','status':'EXPLICIT','activation_receipt':'20260903T063719719Z','activation_rerun':False,'live_catalog':'2026-08-28.nxb1.1','source_candidate':'B1+C+D_SOURCE_ONLY','live_has_D_dynamic_capability_proposal':False,'source_has_D_dynamic_capability_proposal':True,'current_model_loaded':False,'retained_baseline_exception':'64|64|77|22507|828|61 observed at C closure; no D retained write','deployment_performed':False,'model_state_changed':False})
print(json.dumps({'corpus':len(cases),'language':dict(bylang),'variants':dict(counts),'model_loaded':False}))
