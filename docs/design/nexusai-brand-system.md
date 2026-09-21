# NexusAI brand system

Status: R2 source accepted, 2026-08-06

## Identity

- Product: **NexusAI**
- Primary tagline: **Private AI infrastructure for your applications.**
- Forensic context line: **Enterprise Forensic Intelligence**
- Product personality: private, precise, evidence-first, calm, professional.

## Visual direction

- Primary blue: `#2563EB`
- Accent teal: `#14B8A6`
- Dark navy/charcoal canvas, high-contrast text, restrained radii and shadows.
- Connected-network imagery is appropriate only where it communicates evidence
  relationships; decoration must not compete with analytical content.

## Asset contract

The accepted R2 asset set is a code-native NexusAI mark and horizontal lockup,
the existing served `/favicon.svg`, and the same lockup plus text fallback for
login, loading, reports, and exports. Configured customer assets override these
defaults through the existing public branding contract: `instance_name`,
`instance_tagline`, `logo_url`, `logo_horizontal_url`, and `favicon_url`.

No R2/R3 branding API or second asset store is required. Reports and exports
must consume the same configured horizontal logo, falling back to the NexusAI
lockup and product name; a later reporting phase may extend its output contract
only if actual embedding requirements expose a genuine gap. Open Graph,
installable-app, and mobile icon variants remain R3 follow-up metadata work,
not blockers to the first source slice.

## White-label boundary

Product-facing LocalAI copy and upstream visual identity must be replaced on
ordinary analyst surfaces. Technical package names, compatibility APIs,
containers, environment keys, legal notices, source links, and developer/admin
instructions remain where accuracy or compatibility requires them.

The 80 React-source `localai` occurrences found in the bounded search are mostly
storage/config/compatibility identifiers and technical comments. Remaining
visible candidates such as engine image commands and Agent Hub URLs belong in
administrator/developer surfaces, not ordinary case workflows.

## Governance decisions

The established primary tagline is preserved exactly unless a later documented
product decision changes it. “Enterprise Forensic Intelligence” is a secondary
context line, not a replacement product name or tagline. Internal LocalAI
compatibility names remain technical truth; ordinary analyst identity is
NexusAI.

The primary tagline appears on login, onboarding, About, and other identity-led
surfaces, not repeatedly inside dense analytical work. The forensic context line
appears on login and appropriate case/report surfaces, not in global navigation.

Product-facing LocalAI identity is replaced on ordinary analyst routes;
developer/admin instructions contextualize LocalAI as the underlying engine.
Package names, API compatibility identifiers, environment variables,
containers, legal attribution, licenses, and source notices remain unchanged
where accuracy requires them. These decisions authorize R3 source work but do
not authorize a deployed branding change.
