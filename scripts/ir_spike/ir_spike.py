#!/usr/bin/env python3
"""WI-0 — IR-generation feasibility spike.

Measures whether the live 4B synthesis model can fill an enum-constrained
SourceNativePlanV1 over a CURATED semantic catalogue. Throwaway harness; it
imports nothing from the product and changes no production source.

Method, arms, metrics and the binding decision rule are fixed in
reports/ir-spike-20260921/PRE-REGISTRATION.md. This file implements that
document and must not be used to reinterpret it.

Standard library only (house style, matching scripts/nexusai_live_eval.py).

Usage:
  python scripts/ir_spike/ir_spike.py --arm A --out reports/ir-spike-20260921/raw
  python scripts/ir_spike/ir_spike.py --dry-run          # build only, no inference
"""
import argparse
import copy
import json
import os
import re
import statistics
import sys
import time
import urllib.error
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
LOCALAI = os.environ.get("IR_SPIKE_LOCALAI", "http://localhost:8080")
MODEL = os.environ.get("IR_SPIKE_MODEL", "qwen3-4b-instruct-2507-q4km-nxb21d-dev")
CONTRACT = "forensics.source-native-plan/v1"

# Mirrors source_native_algebra.go:29 and query.go:30.
PROJECT_LIMIT, FILTER_LIMIT, MEASURE_LIMIT, GROUP_LIMIT, SORT_LIMIT = 12, 8, 4, 2, 2
MAX_LIMIT = 100
AGGREGATES = ["COUNT", "SUM", "AVG", "MIN", "MAX"]
FILTER_OPS = ["EQ", "NEQ", "IN", "CONTAINS", "GT", "GTE", "LT", "LTE",
              "BETWEEN", "IS_NULL", "IS_NOT_NULL"]


# --------------------------------------------------------------------------
# Minimal YAML-subset loader.
#
# Deliberately small: it parses only the constructs used by
# semantic_layer_min.yaml (nested maps, lists of maps, folded '>' scalars,
# inline flow lists, quoted scalars). It is NOT a general YAML parser and is
# not intended to survive this work item -- WI-4 owns the real loader.
# --------------------------------------------------------------------------
def _scalar(text):
    text = text.strip()
    if len(text) >= 2 and text[0] == text[-1] and text[0] in "\"'":
        return text[1:-1]
    if text.startswith("[") and text.endswith("]"):
        inner = text[1:-1].strip()
        if not inner:
            return []
        return [_scalar(part) for part in _split_flow(inner)]
    if text in ("true", "false"):
        return text == "true"
    if re.fullmatch(r"-?\d+", text):
        return int(text)
    return text


def _split_flow(inner):
    out, buf, quote = [], "", None
    for ch in inner:
        if quote:
            if ch == quote:
                quote = None
            buf += ch
        elif ch in "\"'":
            quote, buf = ch, buf + ch
        elif ch == ",":
            out.append(buf)
            buf = ""
        else:
            buf += ch
    if buf.strip():
        out.append(buf)
    return out


def _lines(raw):
    out = []
    for line in raw.splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        out.append((len(line) - len(line.lstrip()), line.strip()))
    return out


def load_yaml(path):
    with open(path, encoding="utf-8") as handle:
        rows = _lines(handle.read())
    value, _ = _parse_block(rows, 0, rows[0][0] if rows else 0)
    return value


def _parse_block(rows, index, indent):
    if index < len(rows) and rows[index][1].startswith("- "):
        return _parse_list(rows, index, indent)
    return _parse_map(rows, index, indent)


def _parse_list(rows, index, indent):
    items = []
    while index < len(rows) and rows[index][0] == indent and rows[index][1].startswith("- "):
        head = rows[index][1][2:]
        child = indent + 2
        if ":" in head and not head.startswith("["):
            # Rewrite "- key: v" as a map row so the item parses uniformly.
            synthetic = [(child, head)]
            index += 1
            while index < len(rows) and rows[index][0] >= child:
                synthetic.append(rows[index])
                index += 1
            item, _ = _parse_map(synthetic, 0, child)
            items.append(item)
        else:
            items.append(_scalar(head))
            index += 1
    return items, index


def _parse_map(rows, index, indent):
    out = {}
    while index < len(rows) and rows[index][0] == indent:
        depth, text = rows[index]
        if text.startswith("- "):
            break
        key, _, rest = text.partition(":")
        key, rest = key.strip(), rest.strip()
        if rest in (">", "|", ">-", "|-"):
            index += 1
            parts = []
            while index < len(rows) and rows[index][0] > depth:
                parts.append(rows[index][1])
                index += 1
            out[key] = " ".join(parts)
        elif rest:
            out[key] = _scalar(rest)
            index += 1
        else:
            index += 1
            if index < len(rows) and rows[index][0] > depth:
                out[key], index = _parse_block(rows, index, rows[index][0])
            else:
                out[key] = None
    return out, index


# --------------------------------------------------------------------------
# S4 -- deterministic catalogue narrowing.
#
# Lexical only: family synonyms, then field synonyms/display names. NEVER an
# embedding ranker (hard rule: D2 and the measure defect both came from
# embedding signals on structural roles).
# --------------------------------------------------------------------------
def _tokens(text):
    return set(re.findall(r"[a-z0-9]+", text.lower()))


# Literal shapes that are strong evidence of one family. Deterministic, and
# narrow on purpose: precision over recall (S2).
LITERAL_AFFINITY = [
    (re.compile(r"\bPK-SUB-[A-Z0-9-]+\b"), "subscriber"),
    (re.compile(r"\bPK-[A-Z]{3}-SYN-\d+\b"), "tower_location"),
    (re.compile(r"\bACCT-\d+\b"), "transaction"),
    (re.compile(r"\bCAM-\d+\b"), "anpr"),
    (re.compile(r"\b[A-Z]{2,4}-\d{3,4}\b"), "anpr"),
    (re.compile(r"\b(?:92|03)\d{7,13}\b"), "cdr"),
]


def _singular(token):
    for suffix in ("ies", "es", "s"):
        if token.endswith(suffix) and len(token) > len(suffix) + 2:
            return token[: -len(suffix)] + ("y" if suffix == "ies" else "")
    return token


def _stems(text):
    return {_singular(token) for token in _tokens(text)}


def resolve_family(question, layer):
    """Deterministic family resolution. Returns (entity|None, score).

    Three lexical signals, no embeddings (hard rule): entity synonyms, the
    synonyms of the entity's own FIELDS, and literal-shape affinity. Field
    evidence is what separates "which cell site handled the most CALLS" (a CDR
    question with a cell-site dimension) from a question about towers.
    """
    stems = _stems(question)
    scored = []
    for entity in layer["entities"]:
        entity_score = 0
        for phrase in [entity["family"], entity["display_name"]] + list(entity.get("synonyms") or []):
            phrase_stems = _stems(phrase)
            if phrase_stems and phrase_stems <= stems:
                entity_score = max(entity_score, 3 * len(phrase_stems))
        field_hits = 0
        for field in entity["fields"]:
            for phrase in [field["display_name"]] + list(field.get("synonyms") or []):
                phrase_stems = _stems(phrase)
                if phrase_stems and phrase_stems <= stems:
                    field_hits += 1
                    break
        affinity = 0
        for pattern, family in LITERAL_AFFINITY:
            if family == entity["family"] and pattern.search(question):
                affinity = 4
                break
        scored.append((entity_score + field_hits + affinity, entity_score, entity))
    scored.sort(key=lambda row: (-row[0], -row[1]))
    if not scored or scored[0][0] == 0:
        return None, 0
    return scored[0][2], scored[0][0]


def family_entity(layer, name):
    for entity in layer["entities"]:
        if entity["family"] == name:
            return entity
    return None


def narrow(question, entity, cap=30):
    """Working set for one question. Small families return in full."""
    fields = list(entity["fields"])
    if len(fields) <= cap:
        return fields
    words = _tokens(question)
    scored = []
    for field in fields:
        hay = " ".join([field["id"], field["display_name"]] + list(field.get("synonyms") or []))
        scored.append((len(words & _tokens(hay)), field))
    scored.sort(key=lambda pair: -pair[0])
    return [field for _, field in scored[:cap]]


# --------------------------------------------------------------------------
# S5 -- request-time JSON Schema with enum-constrained field slots.
#
# Top-level keys are named so that the converter's ALPHABETICAL property order
# (json_schema.go:154-173, no properties_order available per request) yields
# decision -> plan -> reason: decide first, then fill, then explain.
# Every declared property is mandatory in the generated grammar, so anything
# optional is modelled as an empty-able array or an explicit null union.
# --------------------------------------------------------------------------
def build_schema(fields):
    ids = [field["id"] for field in fields]
    time_ids = [f["id"] for f in fields if f["type"] in ("TIMESTAMP", "DATE")] or [ids[0]]
    measure_ids = ["m1", "m2", "m3", "m4"]
    text = {"type": "string", "maxLength": 256}

    def enum(*values):
        return {"type": "string", "enum": list(values)}

    def obj(props):
        return {"type": "object", "additionalProperties": False,
                "properties": props, "required": sorted(props)}

    field_id = enum(*ids)
    plan = obj({
        "contract_version": {"type": "string", "const": CONTRACT},
        "project": {"type": "array", "items": field_id, "maxItems": PROJECT_LIMIT},
        "filters": {"type": "array", "maxItems": FILTER_LIMIT, "items": obj({
            "field_id": field_id,
            "op": enum(*FILTER_OPS),
            "value": text,
            "values": {"type": "array", "items": text, "maxItems": 20},
        })},
        "group_fields": {"type": "array", "items": field_id, "maxItems": GROUP_LIMIT},
        "measures": {"type": "array", "maxItems": MEASURE_LIMIT, "items": obj({
            "measure_id": enum(*measure_ids),
            "op": enum(*AGGREGATES),
            "field_id": enum(*([""] + ids)),
        })},
        "time_bucket": {"anyOf": [obj({
            "field_id": enum(*time_ids),
            "bucket": enum("hour", "day", "week", "month"),
            "timezone": text,
        }), {"type": "null"}]},
        "having": {"type": "array", "maxItems": MEASURE_LIMIT, "items": obj({
            "measure_id": enum(*measure_ids),
            "op": enum("EQ", "NEQ", "GT", "GTE", "LT", "LTE"),
            "value": text,
        })},
        "sort": {"type": "array", "maxItems": SORT_LIMIT, "items": obj({
            "target": enum(*(ids + measure_ids)),
            "direction": enum("ASC", "DESC"),
        })},
        "limit": {"type": "integer", "minimum": 0, "maximum": MAX_LIMIT},
    })
    # `reason` is an ENUM, not free text. Two reasons, both load-bearing:
    # the GBNF converter does not enforce maxLength, so a free-text reason is
    # unbounded generation (~30s of the first smoke call's 130s at ~4 tok/s);
    # and a typed abstention code is what S10 needs to build a specific
    # clarifying question, where a paragraph of prose is useless.
    return obj({
        "decision": enum("PLAN", "ABSTAIN"),
        "plan": {"anyOf": [plan, {"type": "null"}]},
        "reason": enum("PLAN_OK", "FIELD_NOT_IN_CATALOGUE", "NEEDS_DISTINCT_COUNT",
                       "NEEDS_COMPUTED_VALUE", "FAMILY_NOT_IN_CASE", "AMBIGUOUS_MEASURE"),
    })


SYSTEM = """You convert one analyst question into ONE bounded analytical plan over an issued field catalogue. The plan is executed for you. You never write SQL, and you never name a table or a column - only the field_id values given to you.

HOW TO BUILD THE PLAN
- Count rows with one measure {"measure_id":"m1","op":"COUNT","field_id":""}. COUNT is the only aggregate allowed an empty field_id.
- "How many X are there" counts rows.
- A total or sum of a numeric field uses SUM. The largest uses MAX, the smallest MIN. Never answer "what is the total amount" or "what was the largest" with COUNT.
- A breakdown, "by X", "for each X", "of each type" or "X versus Y" puts that field in group_fields and counts rows.
- "Which X has the most / most often / most frequently" puts X in group_fields, counts rows, sorts {"target":"m1","direction":"DESC"} and sets limit 1.
- "first and last", "earliest and latest", "what date range" uses two measures: MIN and MAX of the time field.
- project lists attributes of particular rows ("where is tower T", "who is subscriber S"). project can NEVER be combined with group_fields, measures, having or time_bucket.

FILTERS ARE SUPPORTED AND ARE MANDATORY
Every literal in the question - a phone number, plate, account, site code, status, HTTP code, date or month - MUST become an entry in filters. This is always expressible: put the field_id in field_id, the operator in op, and the literal text in value.
  "How many sightings of plate ABC-123?" ->
    filters [{"field_id":"anpr.plate_number","op":"EQ","value":"ABC-123","values":[]}]
    measures [{"measure_id":"m1","op":"COUNT","field_id":""}]
  A month or period uses op BETWEEN with values ["<start>","<end>"] and an empty value.
Never drop a literal. Never decide a filter is impossible - filters are part of this format.

WHEN TO ABSTAIN
Set decision "ABSTAIN" with plan null ONLY in these cases:
- the question needs a field that is not in the catalogue -> reason FIELD_NOT_IN_CATALOGUE
- it needs a count of DISTINCT values ("how many unique", "how many different") -> reason NEEDS_DISTINCT_COUNT
- it needs a value computed from two fields, such as a duration or a difference -> reason NEEDS_COMPUTED_VALUE
- the evidence family it asks about is not in this case -> reason FAMILY_NOT_IN_CASE
- it is genuinely unclear which measure is meant -> reason AMBIGUOUS_MEASURE
Otherwise set decision "PLAN" and reason "PLAN_OK". Abstaining when you could have built a plan is a failure; filtering on a literal is never a reason to abstain."""


def build_prompt(question, fields, family):
    catalogue = []
    for field in fields:
        catalogue.append({
            "field_id": field["id"],
            "name": field["display_name"],
            "description": field.get("description", ""),
            "type": field["type"],
            "groupable": bool(field.get("groupable")),
            "aggregates": field.get("allowed_aggregates") or [],
            "also_called": field.get("synonyms") or [],
        })
    return json.dumps({
        "question": question,
        "evidence_family": family,
        "fields": catalogue,
    }, ensure_ascii=False)


def strip_curation(fields):
    """Arm D: price the curated layer by removing what curation adds."""
    out = []
    for field in fields:
        bare = dict(field)
        bare["description"] = ""
        bare["synonyms"] = []
        bare["display_name"] = field["id"]
        out.append(bare)
    return out


def hash_ids(fields, entity_family):
    """Arm C: price the opaque content-hash ID convention (PRE-REG 2.3)."""
    import hashlib
    out, mapping = [], {}
    for field in fields:
        digest = hashlib.sha256((entity_family + "\x00" + field["id"]).encode()).hexdigest()[:24]
        opaque = "fld_" + digest
        mapping[opaque] = field["id"]
        renamed = dict(field)
        renamed["id"] = opaque
        out.append(renamed)
    return out, mapping


# --------------------------------------------------------------------------
# Inference
# --------------------------------------------------------------------------
def call_model(schema, question, fields, family, timeout, extra=None):
    messages = [{"role": "system", "content": SYSTEM},
                {"role": "user", "content": build_prompt(question, fields, family)}]
    if extra:
        messages.append({"role": "user", "content": extra})
    payload = {
        "model": MODEL, "temperature": 0, "max_tokens": 512, "messages": messages,
        "response_format": {"type": "json_schema", "json_schema": {
            "name": "bounded_analytical_plan", "strict": True, "schema": schema}},
    }
    request = urllib.request.Request(
        LOCALAI.rstrip("/") + "/v1/chat/completions",
        data=json.dumps(payload).encode(),
        headers={"Content-Type": "application/json"})
    started = time.time()
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            body = json.loads(response.read().decode())
    except urllib.error.HTTPError as err:
        return None, time.time() - started, "http_%d" % err.code
    except Exception as err:                                    # noqa: BLE001
        return None, time.time() - started, "transport:%s" % type(err).__name__
    elapsed = time.time() - started
    choices = body.get("choices") or []
    if len(choices) != 1:
        return None, elapsed, "no_choice"
    return choices[0]["message"]["content"], elapsed, None


# --------------------------------------------------------------------------
# S6 -- hard validation. Total, deterministic, and the source of the typed
# diff fed back in the single self-correction round.
# --------------------------------------------------------------------------
def validate(plan, fields):
    by_id = {field["id"]: field for field in fields}
    errors = []

    def known(field_id, where):
        if field_id not in by_id:
            errors.append("field_id %r in %s is not in the issued catalogue; issued: %s"
                          % (field_id, where, ", ".join(sorted(by_id))))
            return None
        return by_id[field_id]

    if plan.get("contract_version") != CONTRACT:
        errors.append("contract_version must be %s" % CONTRACT)

    project = plan.get("project") or []
    measures = plan.get("measures") or []
    groups = plan.get("group_fields") or []
    filters = plan.get("filters") or []
    having = plan.get("having") or []
    sort = plan.get("sort") or []

    if project and (groups or measures or plan.get("time_bucket") or having):
        errors.append("project cannot be combined with group_fields, measures, having or time_bucket")
    if not project and not measures:
        errors.append("plan needs either project or measures")
    if len(groups) > GROUP_LIMIT:
        errors.append("at most %d group fields" % GROUP_LIMIT)
    if len(measures) > MEASURE_LIMIT or len(filters) > FILTER_LIMIT:
        errors.append("too many measures or filters")
    if not isinstance(plan.get("limit"), int) or not 0 <= plan["limit"] <= MAX_LIMIT:
        errors.append("limit must be between 0 and %d" % MAX_LIMIT)

    for field_id in project:
        field = known(field_id, "project")
        if field and not field.get("projectable"):
            errors.append("field_id %s is not projectable" % field_id)
    for field_id in groups:
        field = known(field_id, "group_fields")
        if field and not field.get("groupable"):
            groupable = [f["id"] for f in fields if f.get("groupable")]
            errors.append("field_id %s is not groupable; groupable fields are: %s"
                          % (field_id, ", ".join(groupable)))
    for index, measure in enumerate(measures):
        want = "m%d" % (index + 1)
        if measure.get("measure_id") != want:
            errors.append("measure %d must have measure_id %s" % (index + 1, want))
        op = (measure.get("op") or "").upper()
        if op not in AGGREGATES:
            errors.append("aggregate %r is not allowed" % op)
        field_id = measure.get("field_id") or ""
        if not field_id:
            if op != "COUNT":
                errors.append("only COUNT may omit field_id; %s needs a field_id" % op)
        else:
            field = known(field_id, "measures")
            if field and op not in (field.get("allowed_aggregates") or []):
                errors.append("aggregate %s is not allowed on %s; allowed: %s"
                              % (op, field_id, ", ".join(field.get("allowed_aggregates") or [])))
    for filt in filters:
        field = known(filt.get("field_id"), "filters")
        op = (filt.get("op") or "").upper()
        if field and op not in (field.get("allowed_filters") or []):
            errors.append("filter %s is not allowed on %s; allowed: %s"
                          % (op, filt.get("field_id"), ", ".join(field.get("allowed_filters") or [])))
        if op == "BETWEEN" and len(filt.get("values") or []) != 2:
            errors.append("BETWEEN requires exactly two values")
        if op == "IN" and not (filt.get("values") or []):
            errors.append("IN requires at least one value")
        if op not in ("BETWEEN", "IN", "IS_NULL", "IS_NOT_NULL") and not (filt.get("value") or ""):
            errors.append("%s requires a value" % op)
    issued = set("m%d" % (i + 1) for i in range(len(measures)))
    for spec in sort:
        target = spec.get("target")
        if target in issued:
            continue
        if target not in groups and target not in project:
            errors.append("sort target %r is not an issued output; issued: %s"
                          % (target, ", ".join(sorted(issued | set(groups) | set(project)))))
        elif target in by_id and not by_id[target].get("sortable"):
            errors.append("field_id %s is not sortable" % target)
    for spec in having:
        if spec.get("measure_id") not in issued:
            errors.append("having references unissued measure %r" % spec.get("measure_id"))
    return errors


# --------------------------------------------------------------------------
# S9 -- constraint obligations. Every literal in the question must bind.
# --------------------------------------------------------------------------
LITERAL = re.compile(
    r"\b(?:\d{9,15}|[A-Z]{2,4}-\d{2,6}|ACCT-\d+|PK-[A-Z0-9-]+|CAM-\d+|\d{3})\b")
MONTHS = ("january february march april may june july august september october "
          "november december").split()


def obligations(question):
    found = set(LITERAL.findall(question))
    lowered = question.lower()
    for month in MONTHS:
        if month in lowered:
            found.add(month)
    return found


def bound(plan, question):
    """Which of the question's literals actually appear in the plan?"""
    blob = json.dumps(plan or {}).lower()
    missing = set()
    for literal in obligations(question):
        if literal in MONTHS:
            index = MONTHS.index(literal) + 1
            if re.search(r"-%02d-" % index, blob) or literal in blob:
                continue
            missing.add(literal)
        elif literal.lower() not in blob:
            missing.add(literal)
    return missing


# --------------------------------------------------------------------------
# Scoring
# --------------------------------------------------------------------------
def canon(plan):
    """Order-insensitive canonical form for exact-IR comparison."""
    if not plan:
        return None
    out = {
        "project": sorted(plan.get("project") or []),
        "group_fields": sorted(plan.get("group_fields") or []),
        "filters": sorted(json.dumps({k: v for k, v in f.items() if k != "values"} |
                                     {"values": sorted(f.get("values") or [])}, sort_keys=True)
                          for f in (plan.get("filters") or [])),
        "measures": sorted(json.dumps({"op": (m.get("op") or "").upper(),
                                       "field_id": m.get("field_id") or ""}, sort_keys=True)
                           for m in (plan.get("measures") or [])),
        "having": sorted(json.dumps(h, sort_keys=True) for h in (plan.get("having") or [])),
        "sort": [json.dumps({"target": s.get("target"),
                             "direction": (s.get("direction") or "").upper()}, sort_keys=True)
                 for s in (plan.get("sort") or [])],
        "time_bucket": plan.get("time_bucket") or None,
    }
    return json.dumps(out, sort_keys=True)


def execution_equivalent(got, gold, shape):
    """Same result set, allowing differences that provably cannot change it.

    Equivalence is deliberately narrow: measure ORDER and a limit on a scalar
    cannot change the answer, but a different field, aggregate, filter or
    grouping always can.
    """
    if got is None or gold is None:
        return False
    if sorted(got.get("project") or []) != sorted(gold.get("project") or []):
        return False
    if sorted(got.get("group_fields") or []) != sorted(gold.get("group_fields") or []):
        return False

    def norm_measures(plan):
        return sorted((( m.get("op") or "").upper(), m.get("field_id") or "")
                      for m in (plan.get("measures") or []))
    if norm_measures(got) != norm_measures(gold):
        return False

    def norm_filters(plan):
        out = []
        for f in plan.get("filters") or []:
            out.append(((f.get("field_id") or ""), (f.get("op") or "").upper(),
                        (f.get("value") or "").strip().lower(),
                        tuple(sorted(v.strip().lower() for v in (f.get("values") or [])))))
        return sorted(out)
    if norm_filters(got) != norm_filters(gold):
        return False

    # A rank must stay ordered and bounded; a scalar's limit is immaterial.
    if shape == "rank":
        got_sort = [( s.get("target"), (s.get("direction") or "").upper())
                    for s in (got.get("sort") or [])]
        gold_sort = [(s.get("target"), (s.get("direction") or "").upper())
                     for s in (gold.get("sort") or [])]
        if got_sort != gold_sort or (got.get("limit") or 0) != (gold.get("limit") or 0):
            return False
    if shape == "rows" and (got.get("limit") or 0) != (gold.get("limit") or 0):
        return False
    return True


def attribute(got, gold, shape):
    """One label per failure, so results are actionable rather than a number."""
    if got is None or gold is None:
        return "no_plan"
    if sorted(got.get("group_fields") or []) != sorted(gold.get("group_fields") or []):
        return "spurious_group" if len(got.get("group_fields") or []) > len(gold.get("group_fields") or []) else "wrong_group"
    got_ops = sorted((m.get("op") or "").upper() for m in (got.get("measures") or []))
    gold_ops = sorted((m.get("op") or "").upper() for m in (gold.get("measures") or []))
    if got_ops != gold_ops:
        return "wrong_aggregate"
    got_measure_fields = sorted(m.get("field_id") or "" for m in (got.get("measures") or []))
    gold_measure_fields = sorted(m.get("field_id") or "" for m in (gold.get("measures") or []))
    if got_measure_fields != gold_measure_fields:
        return "wrong_field"
    if len(got.get("filters") or []) < len(gold.get("filters") or []):
        return "missing_filter"
    if sorted(json.dumps(f, sort_keys=True) for f in (got.get("filters") or [])) != \
       sorted(json.dumps(f, sort_keys=True) for f in (gold.get("filters") or [])):
        return "wrong_filter"
    if sorted(got.get("project") or []) != sorted(gold.get("project") or []):
        return "wrong_field"
    return "wrong_shape"


# --------------------------------------------------------------------------
def run(args):
    layer = load_yaml(os.path.join(HERE, "semantic_layer_min.yaml"))
    with open(os.path.join(HERE, "gold_plans.json"), encoding="utf-8") as handle:
        gold_doc = json.load(handle)
    items = gold_doc["items"]

    results, latencies = [], []
    for item in items:
        question = item["q"]
        auto_entity, score = resolve_family(question, layer)
        auto_family = auto_entity["family"] if auto_entity else None
        gold_family = item.get("family")

        # S1/S4 family routing and S5 IR generation are DIFFERENT stages, and
        # family routing is its own open defect (D4 -> WI-3). Holding the family
        # fixed at the corpus value is what makes an IR failure attributable to
        # the IR rather than to routing. The auto resolver still runs on every
        # item and its accuracy is reported separately.
        if args.family == "auto":
            entity = auto_entity
        else:
            entity = family_entity(layer, gold_family) if gold_family != "absent" else None

        record = {"id": item["id"], "arm": args.arm, "question": question,
                  "shape": item.get("shape"), "gold_kind":
                  "inexpressible" if item.get("inexpressible") else
                  ("abstain" if item.get("gold") == "ABSTAIN" else "plan"),
                  "gold_family": gold_family, "s4_auto_family": auto_family,
                  "s4_auto_correct": auto_family == gold_family}

        if args.dry_run:
            if entity is None:
                print("%-8s %-14s (no family -> deterministic abstain)" % (item["id"], "-"))
                record.update(dry_run=True, family=None)
            else:
                fields = narrow(question, entity)
                schema = build_schema(fields)
                print("%-8s gold=%-14s auto=%-14s %s fields=%-3d schema=%db"
                      % (item["id"], gold_family, auto_family or "-",
                         "OK " if auto_family == gold_family else "MISS",
                         len(fields), len(json.dumps(schema))))
                record.update(dry_run=True, family=entity["family"], working_set=len(fields))
            results.append(record)
            continue

        if entity is None:
            # S9 FAMILY: no family in this case matches. Abstain is correct.
            record.update(family=None, decision="ABSTAIN", abstained=True,
                          deterministic_abstention=True,
                          correct=record["gold_kind"] == "abstain",
                          confident_wrong=False, latency=0.0, malformed=False,
                          attribution=None if record["gold_kind"] == "abstain" else "no_plan")
            results.append(record)
            print("%-8s %-10s deterministic ABSTAIN (no family)  %s"
                  % (item["id"], "-", "OK" if record["correct"] else "MISS"))
            continue

        fields = narrow(question, entity)
        mapping = None
        if args.arm == "D":
            fields = strip_curation(fields)
        elif args.arm == "C":
            fields, mapping = hash_ids(fields, entity["family"])
        schema = build_schema(fields)
        record["family"] = entity["family"]
        record["working_set"] = len(fields)

        content, elapsed, err = call_model(schema, question, fields, entity["family"], args.timeout)
        total = elapsed
        malformed, parsed, retried = False, None, False
        if err:
            record.update(decision=None, error=err, malformed=False, latency=total,
                          correct=False, confident_wrong=False, attribution="timeout")
            results.append(record)
            print("%-8s %-14s ERROR %s (%.1fs)" % (item["id"], entity["family"], err, total))
            continue
        try:
            parsed = json.loads(content)
        except Exception:                                        # noqa: BLE001
            malformed = True

        errors = []
        if not malformed:
            decision = (parsed.get("decision") or "").upper()
            plan = parsed.get("plan")
            if decision == "PLAN" and plan:
                errors = validate(plan, fields)
                if errors and args.arm == "B":
                    diff = ("The plan was rejected. Fix exactly these problems and "
                            "return a corrected object:\n- " + "\n- ".join(errors[:6]))
                    retried = True
                    content2, elapsed2, err2 = call_model(
                        schema, question, fields, entity["family"], args.timeout, extra=diff)
                    total += elapsed2
                    if not err2:
                        try:
                            reparsed = json.loads(content2)
                            if (reparsed.get("decision") or "").upper() == "PLAN" and reparsed.get("plan"):
                                candidate = reparsed["plan"]
                                if not validate(candidate, fields):
                                    parsed, plan, errors = reparsed, candidate, []
                            elif (reparsed.get("decision") or "").upper() == "ABSTAIN":
                                parsed, plan, errors = reparsed, None, []
                                decision = "ABSTAIN"
                        except Exception:                        # noqa: BLE001
                            pass

        latencies.append(total)
        if malformed:
            record.update(decision=None, malformed=True, latency=total, correct=False,
                          confident_wrong=False, attribution="malformed", raw=content[:400])
            results.append(record)
            print("%-8s %-14s MALFORMED (%.1fs)" % (item["id"], entity["family"], total))
            continue

        decision = (parsed.get("decision") or "").upper()
        plan = parsed.get("plan") if decision == "PLAN" else None
        if mapping and plan:
            plan = json.loads(re.sub(r"fld_[0-9a-f]{24}",
                                     lambda m: mapping.get(m.group(0), m.group(0)),
                                     json.dumps(plan)))
        abstained = decision == "ABSTAIN" or plan is None or bool(errors)
        gold = item.get("gold") if isinstance(item.get("gold"), dict) else None
        unbound = bound(plan, question) if plan else set()
        if unbound:
            abstained = True                     # S9 CONSTRAINT_APPLIED: refuse, never drop.

        gold_kind = record["gold_kind"]
        if gold_kind in ("abstain", "inexpressible"):
            correct = abstained
            confident_wrong = not abstained
            attribution = None if correct else ("inexpressible" if gold_kind == "inexpressible" else "no_plan")
            exact = False
        else:
            exact = (not abstained) and canon(plan) == canon(gold)
            correct = (not abstained) and execution_equivalent(plan, gold, item.get("shape"))
            confident_wrong = (not abstained) and not correct
            attribution = None if correct else (attribute(plan, gold, item.get("shape")) if not abstained else "abstained")

        record.update(decision=decision, plan=plan, malformed=False, latency=total,
                      retried=retried, validation_errors=errors, unbound_literals=sorted(unbound),
                      abstained=abstained, exact=exact, correct=correct,
                      confident_wrong=confident_wrong, attribution=attribution,
                      reason=(parsed.get("reason") or "")[:200])
        results.append(record)
        flag = "OK " if correct else ("ABS" if abstained else "WRONG")
        print("%-8s %-14s %-5s %-16s %.1fs%s"
              % (item["id"], entity["family"], flag, attribution or "", total,
                 " (retried)" if retried else ""))

    summarise(results, latencies, args)
    return results


def summarise(results, latencies, args):
    expressible = [r for r in results if r["gold_kind"] == "plan"]
    inexpr = [r for r in results if r["gold_kind"] == "inexpressible"]
    abst = [r for r in results if r["gold_kind"] == "abstain"]
    scored = [r for r in expressible if not r.get("dry_run")]
    if not scored:
        print("\nDry run: %d items prepared, no inference performed." % len(results))
        return
    n = len(scored)
    correct = sum(1 for r in scored if r.get("correct"))
    exact = sum(1 for r in scored if r.get("exact"))
    cw = sum(1 for r in scored if r.get("confident_wrong"))
    ab = sum(1 for r in scored if r.get("abstained"))
    mal = sum(1 for r in results if r.get("malformed"))
    print("\n" + "=" * 68)
    print("ARM %s  ---  %s" % (args.arm, MODEL))
    print("=" * 68)
    print("Expressible items scored         : %d" % n)
    print("Execution-equivalent match       : %d/%d = %.1f%%" % (correct, n, 100.0 * correct / n))
    print("Exact-IR match                   : %d/%d = %.1f%%" % (exact, n, 100.0 * exact / n))
    print("Abstention (expressible)         : %d/%d = %.1f%%" % (ab, n, 100.0 * ab / n))
    print("CONFIDENT-WRONG                  : %d/%d = %.1f%%" % (cw, n, 100.0 * cw / n))
    print("Malformed (all items)            : %d  [expected 0]" % mal)
    if latencies:
        ordered = sorted(latencies)
        p95 = ordered[min(len(ordered) - 1, int(round(0.95 * (len(ordered) - 1))))]
        print("Latency p50/p95/max              : %.1fs / %.1fs / %.1fs"
              % (statistics.median(ordered), p95, ordered[-1]))
    print("\nInexpressible (abstain = correct): %d/%d correct"
          % (sum(1 for r in inexpr if r.get("correct")), len(inexpr)))
    print("Family-absent  (abstain = correct): %d/%d correct"
          % (sum(1 for r in abst if r.get("correct")), len(abst)))
    buckets = {}
    for r in scored:
        if r.get("attribution"):
            buckets[r["attribution"]] = buckets.get(r["attribution"], 0) + 1
    if buckets:
        print("\nFailure attribution:")
        for key, value in sorted(buckets.items(), key=lambda kv: -kv[1]):
            print("  %-18s %d" % (key, value))
    s4 = [r for r in results if not r.get("dry_run")]
    s4_ok = sum(1 for r in s4 if r.get("s4_auto_correct"))
    print("\nS4 deterministic family routing (reported, not scored): %d/%d = %.1f%%"
          % (s4_ok, len(s4), 100.0 * s4_ok / len(s4)))
    for r in s4:
        if not r.get("s4_auto_correct"):
            print("  %-8s gold=%-14s auto=%s" % (r["id"], r.get("gold_family"), r.get("s4_auto_family")))

    rate = 100.0 * correct / n
    verdict = "PROCEED" if rate >= 80 else ("PROCEED, REINFORCED" if rate >= 60 else "STOP — ESCALATE")
    print("\nPre-registered rule -> %s  (%.1f%% execution-equivalent)" % (verdict, rate))
    if mal > 0:
        print("OVERRIDE: malformed > 0 -> STOP AND ESCALATE (keystone constraint leaked)")
    if 100.0 * cw / n > 10:
        print("OVERRIDE: confident-wrong > 10%% -> STOP AND ESCALATE")

    if args.out:
        os.makedirs(args.out, exist_ok=True)
        path = os.path.join(args.out, "arm_%s.json" % args.arm)
        with open(path, "w", encoding="utf-8") as handle:
            json.dump({"arm": args.arm, "model": MODEL, "results": results}, handle, indent=2)
        print("\nRaw results: %s" % path)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--arm", default="A", choices=["A", "B", "C", "D"])
    parser.add_argument("--timeout", type=float, default=180.0)
    parser.add_argument("--out", default="")
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument("--family", default="gold", choices=["gold", "auto"],
                        help="gold: hold S4 family fixed at the corpus value so IR "
                             "failures are attributable to the IR (default). "
                             "auto: end-to-end, including family routing.")
    args = parser.parse_args()
    run(args)


if __name__ == "__main__":
    main()
