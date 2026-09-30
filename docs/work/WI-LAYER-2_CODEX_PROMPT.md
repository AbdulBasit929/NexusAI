# WI-LAYER-2 — the join graph, the weakest capability in the product

WI-UI-9 is **accepted**, and the curation is verified independently:

    demo collection   81/94 (86%)  ->  93/94 (99%)
    cdr               11/16        ->  16/16   location, Site_Id, Lac_Id, lat, longitude
    tower_location     8/15        ->  14/15

`TAC` excluded because its meaning is not recoverable from the export, and the
`generic` case-notes prose excluded as free text. **Both are the right call and
the right way to record it.** An honest "I cannot establish what this holds"
beats a guessed description — a guessed description is exactly how "average beam
width" was answered with azimuth.

---

## 1. YOUR NEXT ITEM — declare the join graph

**Cross-family is the weakest capability in the product: 0 of 5.** The reason is
now measurable and it is yours:

    access_log  0 joins        cdr          1 join
    anpr        0 joins        ipdr         1 join
    subscriber  0 joins        tower        0 joins
    transaction 0 joins

**Two joins across seven families.** A question like *"where does 03001234567
appear across all evidence?"* (X-01) cannot be answered because nothing declares
how the families connect. It clarifies safely today — which is correct
behaviour, and also a capability the product is supposed to have.

**Your curation just made the most valuable join possible.** `cdr.location`,
`Site_Id` and `Lac_Id` are curated for the first time, so CDR can finally be
joined to `tower_location`. "Which cell site handled these calls, and where is
it?" becomes answerable.

### The joins to declare, in value order

| join | on | why |
|---|---|---|
| `cdr` ↔ `subscriber` | msisdn | who a number belongs to — already declared, verify it |
| `cdr` ↔ `tower_location` | cell site / LAC | **newly possible**; turns call records into locations |
| `ipdr` ↔ `subscriber` | msisdn | same subject across two network families |
| `cdr` ↔ `ipdr` | msisdn | one subject's voice and data activity together |
| `transaction` ↔ `subscriber` | account / msisdn | financial activity attributed to a person |
| `anpr` ↔ `tower_location` | location / coordinates | vehicle sightings against known sites |

**Declare only joins the DATA actually supports.** Check the join column exists
and matches on both sides before writing it — a declared join that matches
nothing produces a confident empty answer, which is the worst outcome this
product can give. If the columns do not align (different formats, different
padding), say so rather than declaring it.

**Cardinality matters** and must be stated (`many_to_one`, `many_to_many`). A
wrong cardinality silently multiplies rows and inflates every count taken across
the join.

## 2. THE MEDIA FAMILIES HAVE NO LAYER ENTITY AT ALL

`document`, `image`, `audio` and `video` appear in no `semantic_layer/*.yaml`.
They are answered today by their own governed executors — document search,
image OCR search and audio transcript search all work — so **this is not urgent
and may not be correct to change.**

Before curating them, answer the question rather than assuming: *do these
families need typed-plan access, or are their dedicated executors the right
home for them?* Media questions are retrieval, not algebra. **Report your reasoning; do
not curate them speculatively.** Adding entities that duplicate a working
executor is how two vocabularies start disagreeing about one question.

## 3. WHAT IS BLOCKED ON THE BACKEND — not yours, and I owe you these

Your WI-UI-9 report named these correctly and they are now on the backend
track's list: login/session identity · tenant, role, classification · paginated
full structured-record access · guaranteed normalized page/region/timed-modality
locators · case entity and create-case · server-persisted pinned questions and
history · authoritative Dashboard attention data.

**Do not build UI against endpoints that do not exist, and do not simulate
them.** Leaving them explicitly unavailable, as you have, is the right
behaviour and it should stay that way until the contracts land.

## 4. CONSTRAINTS — unchanged

Never invent a column, a join, a label for an uncurated value, or a producer
that is not recorded. `sensitivity: PII`/`RESTRICTED` withholds a field from the
dynamic enum, deliberately. `type: NUMBER` maps to DECIMAL — a numeric column
typed as STRING compares as text and `MAX` returns the lexicographically largest
value. **Do not touch `api/**`.**

## 5. VERIFICATION

1. `go test -C api/forensic_records -run TestWI4 .` — loader and layer invariants.
2. `FAMILY_AUDIT_DB=... go test -C api/forensic_records -run TestFamilyCatalogAudit -v .`
   — confirms every question's issued enum and that **no enum is empty** (an
   empty enum is an unparseable grammar and an HTTP 500 before inference).
3. For every join you declare, a SQL check showing it matches real rows on both
   sides, with the row counts. **A join that matches nothing must not ship.**
4. Full Go suite green.

Do not run the live 62-question evaluation — that is the backend track's gate
and needs a coordinated deploy.

## 6. REPORT BACK

Per join: the columns, the cardinality, the SQL row counts proving it matches,
and anything you declined to declare and why. For the media families: your
reasoning on whether they belong in the layer at all — a well-argued "no" is a
complete answer to §2.
