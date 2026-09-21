# Query-language quality

Test clean, conversational, messy, abbreviated, misspelled, reordered, and short English; Roman Urdu; Urdu script; English mixed with Roman Urdu or Urdu; exact identifiers embedded in every language; and bounded follow-ups replacing target, date, direction, or filters.

Semantic equivalence is equality of intent, family/operation or plan, entities/target, time range, direction, filters, and authorized scope—not identical prose.

Keep high-confidence questions deterministic. Let a model propose only allowlisted operations and parameters for unresolved language. Server validation owns capabilities, exact identifiers, scope, authorization, and execution. A model cannot invent operations/capabilities/identifiers, alter identifiers, create arbitrary SQL/tools, or widen scope.

Cover realistic concepts rather than exact phrase memorization, including `dikhao`, `batao`, `kis se`, `sab se zyada`, incoming/outgoing, common contacts, same device/SIM/tower, comparison, and date/time phrases. Preserve exact tokens through Unicode normalization and RTL presentation.

Clarify when target, time, direction, family, or relationship meaning is materially ambiguous. Revalidate inherited follow-up context against tenant/user/case scope and expiry; explicit current-turn values win.
